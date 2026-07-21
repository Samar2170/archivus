package syncer

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

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
	t.Setenv(config.HomeEnv, t.TempDir())
	cfg := &config.Config{ServerURL: serverURL, Token: "test-token", Username: "tester"}
	s, err := New(cfg, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
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
