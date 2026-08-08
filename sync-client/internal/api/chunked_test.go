package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
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
)

// chunkServer is a stand-in for the backend's chunked upload endpoints: it
// stages chunks by index, reports what it has, and assembles on complete.
type chunkServer struct {
	mu       sync.Mutex
	chunks   map[int][]byte
	initReq  map[string]any
	total    int
	complete bool
	aborted  bool
	// failPart, when set, decides whether a given part request should fail
	// before the chunk is stored. Called with the chunk index and the number of
	// times that index has been attempted (1 for the first attempt).
	failPart func(index, attempt int) bool
	attempts map[int]int
}

func newChunkServer() *chunkServer {
	return &chunkServer{chunks: map[int][]byte{}, attempts: map[int]int{}}
}

func (s *chunkServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/storage/file/upload/chunk/init", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		s.initReq = req
		s.total = int(req["totalChunks"].(float64))
		s.mu.Unlock()
		writeJSON(w, map[string]any{"uploadId": "11111111-1111-1111-1111-111111111111", "received": []int{}})
	})
	mux.HandleFunc("/storage/file/upload/chunk/part", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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
		s.mu.Lock()
		s.attempts[index]++
		fail := s.failPart != nil && s.failPart(index, s.attempts[index])
		if !fail {
			s.chunks[index] = data
		}
		received := s.receivedLocked()
		s.mu.Unlock()
		if fail {
			http.Error(w, `{"error":"injected failure"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"received": received})
	})
	mux.HandleFunc("/storage/file/upload/chunk/status", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		received := s.receivedLocked()
		total := s.total
		s.mu.Unlock()
		writeJSON(w, map[string]any{"received": received, "totalChunks": total, "complete": len(received) == total})
	})
	mux.HandleFunc("/storage/file/upload/chunk/complete", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.complete = true
		s.mu.Unlock()
		writeJSON(w, map[string]any{"message": "upload accepted", "status": "pending"})
	})
	mux.HandleFunc("/storage/file/upload/chunk/abort", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.aborted = true
		s.mu.Unlock()
		writeJSON(w, map[string]any{"message": "upload aborted"})
	})
	return mux
}

func (s *chunkServer) receivedLocked() []int {
	out := make([]int, 0, len(s.chunks))
	for i := range s.chunks {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

// assembled concatenates the staged chunks in index order.
func (s *chunkServer) assembled() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	var buf bytes.Buffer
	for _, i := range s.receivedLocked() {
		buf.Write(s.chunks[i])
	}
	return buf.Bytes()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeRandomFile(t *testing.T, size int) (path string, content []byte) {
	t.Helper()
	content = make([]byte, size)
	if _, err := rand.Read(content); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(t.TempDir(), "big.bin")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path, content
}

// uploadChunked runs the two-step flow the syncer uses.
func uploadChunked(t *testing.T, c *Client, driveID, folderPath, path string) error {
	t.Helper()
	sess, err := c.StartChunkedUpload(context.Background(), driveID, folderPath, path)
	if err != nil {
		return err
	}
	return c.ResumeChunkedUpload(context.Background(), sess, path)
}

func TestChunkedUploadSendsEveryChunk(t *testing.T) {
	cs := newChunkServer()
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()

	// 4.5 chunks: exercises the short final chunk.
	path, content := writeRandomFile(t, 4608)

	c := New(srv.URL, "token")
	c.ChunkSize = 1024
	if err := uploadChunked(t, c, "drive-1", "backups/sub", path); err != nil {
		t.Fatalf("chunked upload: %v", err)
	}

	if cs.total != 5 {
		t.Errorf("totalChunks = %d, want 5", cs.total)
	}
	if got := cs.initReq["filename"]; got != "big.bin" {
		t.Errorf("filename = %v, want big.bin", got)
	}
	if got := cs.initReq["driveId"]; got != "drive-1" {
		t.Errorf("driveId = %v, want drive-1", got)
	}
	if got := cs.initReq["folderPath"]; got != "backups/sub" {
		t.Errorf("folderPath = %v, want backups/sub", got)
	}
	if got := cs.initReq["size"]; got != float64(len(content)) {
		t.Errorf("size = %v, want %d", got, len(content))
	}
	if !bytes.Equal(cs.assembled(), content) {
		t.Errorf("assembled bytes differ from the source file")
	}
	if !cs.complete {
		t.Error("complete was never called")
	}
	if cs.aborted {
		t.Error("upload was aborted despite succeeding")
	}
}

func TestChunkedUploadRetriesFailedChunk(t *testing.T) {
	cs := newChunkServer()
	// Chunk 2 fails on its first attempt only.
	cs.failPart = func(index, attempt int) bool { return index == 2 && attempt == 1 }
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()

	path, content := writeRandomFile(t, 4096)

	c := New(srv.URL, "token")
	c.ChunkSize = 1024
	c.RetryBackoff = time.Millisecond
	if err := uploadChunked(t, c, "d", "", path); err != nil {
		t.Fatalf("chunked upload: %v", err)
	}
	if !bytes.Equal(cs.assembled(), content) {
		t.Errorf("assembled bytes differ from the source file")
	}
	if cs.attempts[2] != 2 {
		t.Errorf("chunk 2 attempts = %d, want 2", cs.attempts[2])
	}
	if !cs.complete {
		t.Error("complete was never called")
	}
}

// TestResumeSendsOnlyMissingChunks is the cross-run case: a first attempt dies
// partway through, and a later resume with the stored session re-sends only what
// never landed.
func TestResumeSendsOnlyMissingChunks(t *testing.T) {
	cs := newChunkServer()
	// Chunk 2 fails on every attempt, so the first pass gives up with 0 and 1
	// staged (nothing past the failure is attempted).
	cs.failPart = func(index, attempt int) bool { return index == 2 }
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()

	path, content := writeRandomFile(t, 4096)

	c := New(srv.URL, "token")
	c.ChunkSize = 1024
	c.RetryBackoff = time.Millisecond
	ctx := context.Background()

	sess, err := c.StartChunkedUpload(ctx, "d", "", path)
	if err != nil {
		t.Fatalf("StartChunkedUpload: %v", err)
	}
	if err := c.ResumeChunkedUpload(ctx, sess, path); err == nil {
		t.Fatal("first pass succeeded, want failure")
	}
	if cs.complete {
		t.Error("complete was called for a failed upload")
	}
	if cs.aborted {
		t.Error("session was aborted; it must survive for the resume")
	}

	// Second pass, as a later run would do it: same session, healthy server.
	cs.mu.Lock()
	cs.failPart = nil
	before := map[int]int{}
	maps.Copy(before, cs.attempts)
	cs.mu.Unlock()

	if err := c.ResumeChunkedUpload(ctx, sess, path); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if !bytes.Equal(cs.assembled(), content) {
		t.Errorf("assembled bytes differ from the source file")
	}
	if !cs.complete {
		t.Error("complete was never called")
	}
	// Chunks 0 and 1 were already staged, so the resume must not re-send them.
	for _, i := range []int{0, 1} {
		if cs.attempts[i] != before[i] {
			t.Errorf("chunk %d was re-sent on resume (%d -> %d attempts)", i, before[i], cs.attempts[i])
		}
	}
}

func TestResumeReportsGoneSession(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/storage/file/upload/chunk/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error":"upload session not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	path, _ := writeRandomFile(t, 4096)
	c := New(srv.URL, "token")

	sess := ChunkUploadSession{UploadID: "gone", ChunkSize: 1024, TotalChunks: 4}
	err := c.ResumeChunkedUpload(context.Background(), sess, path)
	if !errors.Is(err, ErrSessionGone) {
		t.Fatalf("err = %v, want ErrSessionGone", err)
	}
}

// A session no longer describing the local file must not be resumed: its staged
// chunk indexes would mean different bytes.
func TestResumeRejectsChangedFile(t *testing.T) {
	cs := newChunkServer()
	srv := httptest.NewServer(cs.handler())
	defer srv.Close()

	path, _ := writeRandomFile(t, 4096)
	c := New(srv.URL, "token")

	sess := ChunkUploadSession{UploadID: "11111111-1111-1111-1111-111111111111", ChunkSize: 1024, TotalChunks: 9}
	err := c.ResumeChunkedUpload(context.Background(), sess, path)
	if !errors.Is(err, ErrSessionGone) {
		t.Fatalf("err = %v, want ErrSessionGone", err)
	}
	if len(cs.chunks) != 0 {
		t.Errorf("sent %d chunks against a stale session, want 0", len(cs.chunks))
	}
}
