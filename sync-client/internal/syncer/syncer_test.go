package syncer

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"archivus-sync/internal/config"
)

// uploadRecorder is a stand-in for the backend's /storage/file/upload endpoint.
type uploadRecorder struct {
	mu      sync.Mutex
	uploads []upload
}

type upload struct {
	driveID    string
	folderPath string
	filename   string
}

func (u *uploadRecorder) handler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		http.Error(w, `{"error":"no files"}`, http.StatusBadRequest)
		return
	}
	u.mu.Lock()
	for _, fh := range files {
		u.uploads = append(u.uploads, upload{
			driveID:    r.FormValue("driveId"),
			folderPath: r.FormValue("folderPath"),
			filename:   fh.Filename,
		})
	}
	u.mu.Unlock()
	io.WriteString(w, `{"message":"ok"}`)
}

func (u *uploadRecorder) count() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.uploads)
}

func newTestSyncer(t *testing.T, serverURL string) *Syncer {
	t.Helper()
	return newTestSyncerIn(t, t.TempDir(), serverURL)
}

// newTestSyncerIn builds a Syncer whose config and state live in home. Passing
// the same home twice simulates a later run picking up the state on disk.
func newTestSyncerIn(t *testing.T, home, serverURL string) *Syncer {
	t.Helper()
	t.Setenv(config.HomeEnv, home)
	cfg := &config.Config{ServerURL: serverURL, Token: "test-token", Username: "tester"}
	s, err := New(cfg, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Shrink the chunk parameters so tests don't have to write a 64 MB file to
	// exercise the chunked path, and don't have to wait out real retry delays.
	s.chunkThreshold = 1024
	s.client.ChunkSize = 1024
	s.client.RetryBackoff = time.Millisecond
	return s
}

func TestUploadTreeOnlyUploadsChangedFiles(t *testing.T) {
	rec := &uploadRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/storage/file/upload" {
			rec.handler(w, r)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"), "alpha")
	writeFile(t, filepath.Join(root, "sub", "b.txt"), "bravo")

	s := newTestSyncer(t, srv.URL)
	ctx := context.Background()

	// First run: both files are new -> 2 uploads.
	res, err := s.UploadPath(ctx, root, "drive-1", "backups", false)
	if err != nil {
		t.Fatalf("first UploadPath: %v", err)
	}
	if res.Uploaded != 2 || res.Skipped != 0 || res.Failed != 0 {
		t.Fatalf("first run = %+v, want 2 uploaded", res)
	}
	if rec.count() != 2 {
		t.Fatalf("server saw %d uploads, want 2", rec.count())
	}

	// Verify destination folder mapping preserves subdirectories.
	got := recordedDests(rec)
	want := []string{"backups", "backups/sub"}
	if !equalStrings(got, want) {
		t.Fatalf("folderPaths = %v, want %v", got, want)
	}

	// Second run: nothing changed -> 0 uploads.
	res, err = s.UploadPath(ctx, root, "drive-1", "backups", false)
	if err != nil {
		t.Fatalf("second UploadPath: %v", err)
	}
	if res.Uploaded != 0 || res.Skipped != 2 {
		t.Fatalf("second run = %+v, want 0 uploaded / 2 skipped", res)
	}

	// Change one file's content -> exactly 1 upload.
	writeFile(t, filepath.Join(root, "a.txt"), "alpha-modified")
	res, err = s.UploadPath(ctx, root, "drive-1", "backups", false)
	if err != nil {
		t.Fatalf("third UploadPath: %v", err)
	}
	if res.Uploaded != 1 || res.Skipped != 1 {
		t.Fatalf("third run = %+v, want 1 uploaded / 1 skipped", res)
	}

	// Force re-uploads everything regardless of state.
	res, err = s.UploadPath(ctx, root, "drive-1", "backups", true)
	if err != nil {
		t.Fatalf("forced UploadPath: %v", err)
	}
	if res.Uploaded != 2 {
		t.Fatalf("forced run = %+v, want 2 uploaded", res)
	}
}

func TestUploadIgnoresTouchOnlyChange(t *testing.T) {
	rec := &uploadRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	defer srv.Close()

	root := t.TempDir()
	p := filepath.Join(root, "a.txt")
	writeFile(t, p, "same-content")

	s := newTestSyncer(t, srv.URL)
	ctx := context.Background()

	if _, err := s.UploadPath(ctx, root, "d", "", false); err != nil {
		t.Fatal(err)
	}
	if rec.count() != 1 {
		t.Fatalf("want 1 initial upload, got %d", rec.count())
	}

	// Rewrite identical content (changes mtime but not the hash).
	writeFile(t, p, "same-content")
	res, err := s.UploadPath(ctx, root, "d", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Uploaded != 0 || res.Skipped != 1 {
		t.Fatalf("touch-only run = %+v, want 0 uploaded", res)
	}
	if rec.count() != 1 {
		t.Fatalf("server should still have 1 upload, got %d", rec.count())
	}
}

// TestLargeFilesUseChunkedUpload checks the size-based routing: files over the
// threshold go to the chunk endpoints, smaller ones to the plain upload.
func TestLargeFilesUseChunkedUpload(t *testing.T) {
	rec := &uploadRecorder{}
	cs := newChunkServer()
	mux := http.NewServeMux()
	mux.HandleFunc("/storage/file/upload", rec.handler)
	cs.register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "small.txt"), "small")
	content := writeBinFile(t, filepath.Join(root, "large.bin"), 4096)

	s := newTestSyncer(t, srv.URL)
	res, err := s.UploadPath(context.Background(), root, "drive-1", "backups", false)
	if err != nil {
		t.Fatalf("UploadPath: %v", err)
	}
	if res.Uploaded != 2 || res.Failed != 0 {
		t.Fatalf("run = %+v, want 2 uploaded", res)
	}

	if rec.count() != 1 {
		t.Errorf("plain upload saw %d files, want only the small one", rec.count())
	}
	sess := cs.only(t)
	if sess.filename != "large.bin" {
		t.Errorf("chunked filename = %q, want large.bin", sess.filename)
	}
	if sess.folderPath != "backups" {
		t.Errorf("chunked folderPath = %q, want backups", sess.folderPath)
	}
	if !bytes.Equal(sess.assembled(), content) {
		t.Error("assembled chunks differ from the source file")
	}
	if !sess.completed {
		t.Error("chunked upload was never completed")
	}
	if len(s.state.Pending) != 0 {
		t.Errorf("state still holds %d pending sessions after success", len(s.state.Pending))
	}
}

// TestChunkedUploadResumesAcrossRuns is the crash case: a run dies partway
// through a large file, and the next run picks the session up from state.json
// and sends only the chunks that never landed.
func TestChunkedUploadResumesAcrossRuns(t *testing.T) {
	cs := newChunkServer()
	// Chunk 2 onwards fails, so the first run gives up with 0 and 1 staged.
	cs.failPart = func(index int) bool { return index >= 2 }
	mux := http.NewServeMux()
	cs.register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	root := t.TempDir()
	content := writeBinFile(t, filepath.Join(root, "large.bin"), 4096)
	home := t.TempDir()
	ctx := context.Background()

	// First run: fails partway through.
	s1 := newTestSyncerIn(t, home, srv.URL)
	res, err := s1.UploadPath(ctx, root, "drive-1", "backups", false)
	if err != nil {
		t.Fatalf("first UploadPath: %v", err)
	}
	if res.Failed != 1 || res.Uploaded != 0 {
		t.Fatalf("first run = %+v, want 1 failed", res)
	}

	// The session must be on disk, not just in memory: that is what a later
	// process has to work from.
	st, err := loadState()
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	pending, ok := st.Pending[filepath.Join(root, "large.bin")]
	if !ok {
		t.Fatal("no pending upload recorded in state.json")
	}
	if pending.UploadID == "" || pending.TotalChunks != 4 || pending.ChunkSize != 1024 {
		t.Fatalf("pending record = %+v, want a full session description", pending)
	}

	// Second run, as a separate process would: same home, healthy server.
	cs.mu.Lock()
	cs.failPart = nil
	sentBefore := maps.Clone(cs.sessions[pending.UploadID].sent) // snapshot, not the live map
	cs.mu.Unlock()

	s2 := newTestSyncerIn(t, home, srv.URL)
	res, err = s2.UploadPath(ctx, root, "drive-1", "backups", false)
	if err != nil {
		t.Fatalf("second UploadPath: %v", err)
	}
	if res.Uploaded != 1 || res.Failed != 0 {
		t.Fatalf("second run = %+v, want 1 uploaded", res)
	}

	// One session for the whole story: the resume reused it rather than
	// starting the file over.
	if n := cs.sessionCount(); n != 1 {
		t.Errorf("server saw %d sessions, want 1 reused across both runs", n)
	}
	sess := cs.only(t)
	if !sess.completed {
		t.Error("resumed upload was never completed")
	}
	if !bytes.Equal(sess.assembled(), content) {
		t.Error("assembled chunks differ from the source file")
	}
	// Chunks 0 and 1 were already staged; the resume must not re-send them.
	for _, i := range []int{0, 1} {
		if sess.sent[i] != sentBefore[i] {
			t.Errorf("chunk %d re-sent on resume (%d -> %d)", i, sentBefore[i], sess.sent[i])
		}
	}

	st, err = loadState()
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if len(st.Pending) != 0 {
		t.Errorf("state.json still holds %d pending sessions after success", len(st.Pending))
	}
}

// TestStaleChunkSessionIsAborted covers a recorded session that no longer
// describes the local file: it must be abandoned server-side, not resumed.
func TestStaleChunkSessionIsAborted(t *testing.T) {
	cs := newChunkServer()
	cs.failPart = func(index int) bool { return index >= 2 }
	mux := http.NewServeMux()
	cs.register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	root := t.TempDir()
	path := filepath.Join(root, "large.bin")
	writeBinFile(t, path, 4096)
	home := t.TempDir()
	ctx := context.Background()

	s1 := newTestSyncerIn(t, home, srv.URL)
	if res, err := s1.UploadPath(ctx, root, "drive-1", "backups", false); err != nil || res.Failed != 1 {
		t.Fatalf("first run = %+v, %v; want 1 failed", res, err)
	}
	stale := cs.only(t).uploadID

	// Rewrite the file before the next run: the staged chunks are now bytes
	// from a file that no longer exists.
	cs.mu.Lock()
	cs.failPart = nil
	cs.mu.Unlock()
	content := writeBinFile(t, path, 4096)

	s2 := newTestSyncerIn(t, home, srv.URL)
	if res, err := s2.UploadPath(ctx, root, "drive-1", "backups", false); err != nil || res.Uploaded != 1 {
		t.Fatalf("second run = %+v, %v; want 1 uploaded", res, err)
	}

	if !cs.wasAborted(stale) {
		t.Errorf("stale session %s was not aborted", stale)
	}
	if n := cs.sessionCount(); n != 2 {
		t.Errorf("server saw %d sessions, want 2 (stale + restart)", n)
	}
	var fresh *chunkSession
	for _, sess := range cs.all() {
		if sess.uploadID != stale {
			fresh = sess
		}
	}
	if fresh == nil || !fresh.completed {
		t.Fatal("restarted upload was never completed")
	}
	if !bytes.Equal(fresh.assembled(), content) {
		t.Error("assembled chunks differ from the rewritten file")
	}
}

// chunkServer stands in for the backend's chunked upload endpoints, staging
// chunks per session so tests can assert what was sent and what was reused.
type chunkServer struct {
	mu       sync.Mutex
	sessions map[string]*chunkSession
	nextID   int
	aborted  map[string]bool
	// failPart, when set, rejects the given chunk index instead of storing it.
	failPart func(index int) bool
}

type chunkSession struct {
	uploadID   string
	filename   string
	driveID    string
	folderPath string
	total      int
	chunks     map[int][]byte
	sent       map[int]int // per-index request count, including failed ones
	completed  bool
}

func newChunkServer() *chunkServer {
	return &chunkServer{sessions: map[string]*chunkSession{}, aborted: map[string]bool{}}
}

func (cs *chunkServer) register(mux *http.ServeMux) {
	mux.HandleFunc("/storage/file/upload/chunk/init", cs.init)
	mux.HandleFunc("/storage/file/upload/chunk/part", cs.part)
	mux.HandleFunc("/storage/file/upload/chunk/status", cs.status)
	mux.HandleFunc("/storage/file/upload/chunk/complete", cs.complete)
	mux.HandleFunc("/storage/file/upload/chunk/abort", cs.abort)
}

func (cs *chunkServer) init(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Filename    string `json:"filename"`
		DriveId     string `json:"driveId"`
		FolderPath  string `json:"folderPath"`
		TotalChunks int    `json:"totalChunks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cs.mu.Lock()
	cs.nextID++
	id := fmt.Sprintf("session-%d", cs.nextID)
	cs.sessions[id] = &chunkSession{
		uploadID:   id,
		filename:   req.Filename,
		driveID:    req.DriveId,
		folderPath: req.FolderPath,
		total:      req.TotalChunks,
		chunks:     map[int][]byte{},
		sent:       map[int]int{},
	}
	cs.mu.Unlock()
	writeJSON(w, map[string]any{"uploadId": id, "totalChunks": req.TotalChunks, "received": []int{}})
}

// lookup resolves a session, replying 404 for one the server does not have —
// the same signal the real backend gives for a completed or aborted upload.
func (cs *chunkServer) lookup(w http.ResponseWriter, uploadID string) (*chunkSession, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	sess, ok := cs.sessions[uploadID]
	if !ok || sess.completed {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error":"upload session not found"}`)
		return nil, false
	}
	return sess, true
}

func (cs *chunkServer) part(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sess, ok := cs.lookup(w, r.FormValue("uploadId"))
	if !ok {
		return
	}
	index, err := strconv.Atoi(r.FormValue("chunkIndex"))
	if err != nil {
		http.Error(w, `{"error":"bad chunkIndex"}`, http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("chunk")
	if err != nil {
		http.Error(w, `{"error":"missing chunk"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cs.mu.Lock()
	sess.sent[index]++
	fail := cs.failPart != nil && cs.failPart(index)
	if !fail {
		sess.chunks[index] = data
	}
	received := sess.receivedLocked()
	cs.mu.Unlock()

	if fail {
		http.Error(w, `{"error":"injected failure"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"received": received, "totalChunks": sess.total})
}

func (cs *chunkServer) status(w http.ResponseWriter, r *http.Request) {
	sess, ok := cs.lookup(w, r.URL.Query().Get("uploadId"))
	if !ok {
		return
	}
	cs.mu.Lock()
	received := sess.receivedLocked()
	cs.mu.Unlock()
	writeJSON(w, map[string]any{
		"received":    received,
		"totalChunks": sess.total,
		"complete":    len(received) == sess.total,
	})
}

func (cs *chunkServer) complete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UploadId string `json:"uploadId"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	sess, ok := cs.lookup(w, req.UploadId)
	if !ok {
		return
	}
	cs.mu.Lock()
	missing := len(sess.chunks) != sess.total
	if !missing {
		sess.completed = true
	}
	cs.mu.Unlock()
	if missing {
		http.Error(w, `{"error":"incomplete upload"}`, http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"message": "upload accepted"})
}

func (cs *chunkServer) abort(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UploadId string `json:"uploadId"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	cs.mu.Lock()
	cs.aborted[req.UploadId] = true
	if sess, ok := cs.sessions[req.UploadId]; ok {
		sess.chunks = map[int][]byte{}
	}
	cs.mu.Unlock()
	writeJSON(w, map[string]any{"message": "upload aborted"})
}

func (s *chunkSession) receivedLocked() []int {
	out := make([]int, 0, len(s.chunks))
	for i := range s.chunks {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

// assembled concatenates the staged chunks in index order.
func (s *chunkSession) assembled() []byte {
	var buf bytes.Buffer
	for _, i := range s.receivedLocked() {
		buf.Write(s.chunks[i])
	}
	return buf.Bytes()
}

func (cs *chunkServer) sessionCount() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return len(cs.sessions)
}

func (cs *chunkServer) all() []*chunkSession {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	out := make([]*chunkSession, 0, len(cs.sessions))
	for _, sess := range cs.sessions {
		out = append(out, sess)
	}
	return out
}

// only returns the single session the server was asked to create.
func (cs *chunkServer) only(t *testing.T) *chunkSession {
	t.Helper()
	all := cs.all()
	if len(all) != 1 {
		t.Fatalf("server has %d sessions, want exactly 1", len(all))
	}
	return all[0]
}

func (cs *chunkServer) wasAborted(uploadID string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.aborted[uploadID]
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeBinFile writes size bytes of random content and returns them, so a test
// can compare what the server assembled against what was on disk.
func writeBinFile(t *testing.T, path string, size int) []byte {
	t.Helper()
	content := make([]byte, size)
	if _, err := rand.Read(content); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return content
}

func recordedDests(rec *uploadRecorder) []string {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	var out []string
	for _, u := range rec.uploads {
		out = append(out, u.folderPath)
	}
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
