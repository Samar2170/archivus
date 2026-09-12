package integration_test

import (
	"fmt"
	"net/http"
	"testing"
)

// TestDownloadURLFallback covers the drive download-URL endpoint on the local
// disk backend, which cannot mint direct URLs: it must clearly report "no URL"
// so the client falls back to the streamed download endpoint.
func TestDownloadURLFallback(t *testing.T) {
	e, _ := newTestServerWithStore(t)

	registerUser(t, e, "samar", "password12", "123456", "samar@example.com", true, "")
	token := loginUser(t, e, "samar", "123456")
	_, driveIDs := getUserInfo(t, e, token)
	if len(driveIDs) == 0 {
		t.Fatal("admin should have at least one drive")
	}
	driveID := driveIDs[0]

	if r := e.uploadFiles(t, "docs", driveID, token, map[string][]byte{
		"note.txt": []byte("direct url test"),
	}); r.status != http.StatusOK {
		t.Fatalf("upload: status %d body %v", r.status, r.body)
	}

	list := e.doJSON(t, http.MethodPost, "/storage/files", map[string]any{
		"path":    "docs",
		"driveId": driveID,
	}, token)
	files, _ := list.body["files"].([]any)
	var fileID string
	for _, f := range files {
		entry, _ := f.(map[string]any)
		if isDir, _ := entry["IsDir"].(bool); !isDir {
			fileID, _ = entry["ID"].(string)
			break
		}
	}
	if fileID == "" {
		t.Fatalf("no file ID in listing: %v", list.body)
	}

	path := fmt.Sprintf("/storage/file/download/url?fileId=%s&driveId=%s", fileID, driveID)

	t.Run("disk backend returns empty url", func(t *testing.T) {
		r := e.doJSON(t, http.MethodGet, path, nil, token)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		if url, _ := r.body["url"].(string); url != "" {
			t.Fatalf("expected empty url on disk backend, got %q", url)
		}
	})

	t.Run("requires authentication", func(t *testing.T) {
		r := e.doJSON(t, http.MethodGet, path, nil, "")
		// The auth middleware answers unauthenticated requests with 403.
		if r.status != http.StatusForbidden {
			t.Fatalf("expected 403, got %d body %v", r.status, r.body)
		}
	})

	t.Run("rejects a drive the user cannot access", func(t *testing.T) {
		bad := fmt.Sprintf("/storage/file/download/url?fileId=%s&driveId=00000000-0000-0000-0000-000000000000", fileID)
		r := e.doJSON(t, http.MethodGet, bad, nil, token)
		if r.status == http.StatusOK {
			t.Fatalf("expected failure for unknown drive, got 200 body %v", r.body)
		}
	})

	t.Run("streamed download still works", func(t *testing.T) {
		status, content, _ := e.downloadFile(t, fileID, driveID, token)
		if status != http.StatusOK {
			t.Fatalf("status %d", status)
		}
		if string(content) != "direct url test" {
			t.Fatalf("content = %q", content)
		}
	})
}
