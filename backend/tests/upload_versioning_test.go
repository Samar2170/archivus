package integration_test

import (
	"net/http"
	"testing"
)

// listFiles returns the raw file/dir entries the API reports for a folder.
func listFiles(t *testing.T, e *testEnv, path, driveID, token string) []map[string]any {
	t.Helper()
	r := e.doJSON(t, http.MethodPost, "/storage/files", map[string]any{
		"path":    path,
		"driveId": driveID,
	}, token)
	if r.status != http.StatusOK {
		t.Fatalf("list files %q: status %d body %v", path, r.status, r.body)
	}
	raw, _ := r.body["files"].([]any)
	var out []map[string]any
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// TestUploadOverwriteInPlace verifies home mode (disk backend) treats a repeated
// upload of the same file as an in-place overwrite: the content is replaced, no
// duplicate metadata row is created, and the file id is stable.
func TestUploadOverwriteInPlace(t *testing.T) {
	e := newTestServer(t)
	registerUser(t, e, "owner", "password12", "123456", "owner@example.com", true, "")
	token := loginUser(t, e, "owner", "123456")
	_, driveIDs := getUserInfo(t, e, token)
	if len(driveIDs) == 0 {
		t.Fatal("expected a drive")
	}
	driveID := driveIDs[0]

	// First upload.
	if r := e.uploadFiles(t, "docs", driveID, token, map[string][]byte{
		"note.txt": []byte("version one"),
	}); r.status != http.StatusOK {
		t.Fatalf("first upload: status %d body %v", r.status, r.body)
	}

	files := listFiles(t, e, "docs", driveID, token)
	if len(files) != 1 {
		t.Fatalf("after first upload want 1 file, got %d: %v", len(files), files)
	}
	firstID, _ := files[0]["ID"].(string)
	if firstID == "" {
		t.Fatalf("missing file ID in %v", files[0])
	}

	// Second upload of the same name with new content.
	if r := e.uploadFiles(t, "docs", driveID, token, map[string][]byte{
		"note.txt": []byte("version two is longer"),
	}); r.status != http.StatusOK {
		t.Fatalf("second upload: status %d body %v", r.status, r.body)
	}

	files = listFiles(t, e, "docs", driveID, token)
	if len(files) != 1 {
		t.Fatalf("after overwrite want 1 file (no duplicate), got %d: %v", len(files), files)
	}
	secondID, _ := files[0]["ID"].(string)
	if secondID != firstID {
		t.Fatalf("overwrite should keep the same file id: had %s, now %s", firstID, secondID)
	}

	// Content should be the new version.
	status, content, _ := e.downloadFile(t, secondID, driveID, token)
	if status != http.StatusOK {
		t.Fatalf("download: status %d", status)
	}
	if string(content) != "version two is longer" {
		t.Fatalf("download content = %q, want overwritten content", string(content))
	}
}
