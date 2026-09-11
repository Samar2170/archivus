package integration_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"archivus/internal/models"
)

// ---- Shared-folder HTTP helpers ----

func sharedRoots(t *testing.T, e *testEnv, token string) []map[string]any {
	t.Helper()
	r := e.doJSON(t, http.MethodGet, "/storage/shared/roots", nil, token)
	if r.status != http.StatusOK {
		t.Fatalf("shared roots: status %d body %v", r.status, r.body)
	}
	raw, _ := r.body["roots"].([]any)
	var out []map[string]any
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// sharedList lists a folder inside a shared subtree. Returns the raw response
// so tests can assert both success and failure shapes.
func sharedList(t *testing.T, e *testEnv, driveID, rootPath, path, token string) resp {
	t.Helper()
	return e.doJSON(t, http.MethodPost, "/storage/shared/list", map[string]any{
		"driveId":  driveID,
		"rootPath": rootPath,
		"path":     path,
	}, token)
}

func sharedDownload(t *testing.T, e *testEnv, fileID, driveID, rootPath, token string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/storage/shared/file/download?fileId=%s&driveId=%s&rootPath=%s",
			e.url, url.QueryEscape(fileID), url.QueryEscape(driveID), url.QueryEscape(rootPath)),
		nil,
	)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer r.Body.Close()
	body := make([]byte, 0, 512)
	buf := make([]byte, 512)
	for {
		n, err := r.Body.Read(buf)
		body = append(body, buf[:n]...)
		if err != nil {
			break
		}
	}
	return r.StatusCode, body
}

func grantShare(t *testing.T, e *testEnv, driveID, rootPath, username, access, token string) resp {
	t.Helper()
	return e.doJSON(t, http.MethodPost, "/storage/shared/grant", map[string]any{
		"driveId":     driveID,
		"rootPath":    rootPath,
		"username":    username,
		"accessLevel": access,
	}, token)
}

func revokeShare(t *testing.T, e *testEnv, driveID, rootPath, username, token string) resp {
	t.Helper()
	return e.doJSON(t, http.MethodPost, "/storage/shared/revoke", map[string]any{
		"driveId":  driveID,
		"rootPath": rootPath,
		"username": username,
	}, token)
}

func sharedUsers(t *testing.T, e *testEnv, driveID, rootPath, token string) resp {
	t.Helper()
	return e.doJSON(t, http.MethodPost, "/storage/shared/list-users", map[string]any{
		"driveId":  driveID,
		"rootPath": rootPath,
	}, token)
}

// setupSharedFixture registers an owner with a drive containing two folders
// ("projects" shared, "secret" not), and a sharee user with no drive
// membership at all. Returns the ids/paths the tests need.
func setupSharedFixture(t *testing.T, e *testEnv) (ownerToken, shareeToken, driveID string) {
	t.Helper()
	registerUser(t, e, "owner", "password12", "123456", "owner@example.com", true, "")
	ownerToken = loginUser(t, e, "owner", "123456")
	_, driveIDs := getUserInfo(t, e, ownerToken)
	if len(driveIDs) == 0 {
		t.Fatal("owner should have a drive")
	}
	driveID = driveIDs[0]

	// A sharee: registers through a read invite, then is removed from the
	// drive, so any access they get must come from folder shares alone.
	invite := e.doJSON(t, http.MethodPost, "/auth/drive/invite", map[string]any{
		"drive_id": driveID,
		"access":   string(models.AccessLevelRead),
	}, ownerToken)
	code, _ := invite.body["invite_code"].(string)
	registerUser(t, e, "sharee", "shareepass1", "222222", "sharee@example.com", false, code)
	shareeToken = loginUser(t, e, "sharee", "222222")
	shareeID, _ := getUserInfo(t, e, shareeToken)
	if r := e.doJSON(t, http.MethodPost, "/auth/drive/remove", map[string]any{
		"user_id":  shareeID,
		"drive_id": driveID,
	}, ownerToken); r.status != http.StatusOK {
		t.Fatalf("remove sharee from drive: status %d body %v", r.status, r.body)
	}

	// Folders and files: "projects" will be shared, "secret" stays private.
	if r := e.doJSON(t, http.MethodPost, "/storage/folder/create", map[string]any{
		"path": "projects/nested", "driveId": driveID,
	}, ownerToken); r.status != http.StatusOK {
		t.Fatalf("create projects/nested: status %d body %v", r.status, r.body)
	}
	if r := e.doJSON(t, http.MethodPost, "/storage/folder/create", map[string]any{
		"path": "secret", "driveId": driveID,
	}, ownerToken); r.status != http.StatusOK {
		t.Fatalf("create secret: status %d body %v", r.status, r.body)
	}
	if r := e.uploadFiles(t, "projects", driveID, ownerToken, map[string][]byte{
		"report.txt": []byte("shared report content"),
	}); r.status != http.StatusOK {
		t.Fatalf("upload report.txt: status %d body %v", r.status, r.body)
	}
	if r := e.uploadFiles(t, "projects/nested", driveID, ownerToken, map[string][]byte{
		"inner.txt": []byte("inner shared content"),
	}); r.status != http.StatusOK {
		t.Fatalf("upload inner.txt: status %d body %v", r.status, r.body)
	}
	if r := e.uploadFiles(t, "secret", driveID, ownerToken, map[string][]byte{
		"hidden.txt": []byte("secret content"),
	}); r.status != http.StatusOK {
		t.Fatalf("upload hidden.txt: status %d body %v", r.status, r.body)
	}
	return ownerToken, shareeToken, driveID
}

// TestSharedFolderAccess covers the whole read-only shared-folder lifecycle:
// grant, browse, navigate, download, isolation from unshared paths and revocation.
func TestSharedFolderAccess(t *testing.T) {
	e := newTestServer(t)
	ownerToken, shareeToken, driveID := setupSharedFixture(t, e)

	var reportID string

	t.Run("granting a share requires an existing folder and user", func(t *testing.T) {
		if r := grantShare(t, e, driveID, "missing", "sharee", "read", ownerToken); r.status == http.StatusOK {
			t.Fatal("grant on a non-existent folder should fail")
		}
		if r := grantShare(t, e, driveID, "projects", "ghost", "read", ownerToken); r.status == http.StatusOK {
			t.Fatal("grant to a non-existent user should fail")
		}
		if r := grantShare(t, e, driveID, "projects", "sharee", "manager", ownerToken); r.status == http.StatusOK {
			t.Fatal("folder shares only accept read and write levels")
		}
		if r := grantShare(t, e, driveID, "projects", "sharee", "read", ownerToken); r.status != http.StatusOK {
			t.Fatalf("grant: status %d body %v", r.status, r.body)
		}
	})

	t.Run("sharee sees the root but not drive membership", func(t *testing.T) {
		roots := sharedRoots(t, e, shareeToken)
		if len(roots) != 1 {
			t.Fatalf("expected exactly 1 shared root, got %d: %v", len(roots), roots)
		}
		if roots[0]["rootPath"] != "projects" {
			t.Fatalf("rootPath = %v, want projects", roots[0]["rootPath"])
		}
		if roots[0]["driveId"] != driveID {
			t.Fatalf("driveId = %v, want %s", roots[0]["driveId"], driveID)
		}

		// No drive membership: the regular drive APIs must reject them.
		if r := e.doJSON(t, http.MethodPost, "/storage/files", map[string]any{
			"path": "", "driveId": driveID,
		}, shareeToken); r.status == http.StatusOK {
			t.Fatal("non-member should not list the drive via /storage/files")
		}
	})

	t.Run("sharee lists and navigates inside the shared root", func(t *testing.T) {
		r := sharedList(t, e, driveID, "projects", "projects", shareeToken)
		if r.status != http.StatusOK {
			t.Fatalf("list root: status %d body %v", r.status, r.body)
		}
		entries, _ := r.body["files"].([]any)
		names := map[string]bool{}
		for _, f := range entries {
			entry, _ := f.(map[string]any)
			name, _ := entry["Name"].(string)
			names[name] = true
			if name == "report.txt" {
				reportID, _ = entry["ID"].(string)
			}
		}
		if !names["report.txt"] || !names["nested"] {
			t.Fatalf("shared root listing should contain report.txt and nested, got %v", names)
		}

		// Navigate into the shared subfolder using the drive-relative path.
		r = sharedList(t, e, driveID, "projects", "projects/nested", shareeToken)
		if r.status != http.StatusOK {
			t.Fatalf("list nested: status %d body %v", r.status, r.body)
		}
		entries = r.body["files"].([]any)
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry in nested, got %v", entries)
		}
		entry, _ := entries[0].(map[string]any)
		if entry["Name"] != "inner.txt" {
			t.Fatalf("nested listing = %v, want inner.txt", entries)
		}
	})

	t.Run("sharee cannot escape the shared subtree", func(t *testing.T) {
		// Path outside the shared root but inside the drive.
		if r := sharedList(t, e, driveID, "projects", "secret", shareeToken); r.status == http.StatusOK {
			t.Fatal("listing a sibling of the shared root must fail")
		}
		// Traversal-ish path that normalizes outside the root.
		if r := sharedList(t, e, driveID, "projects", "projects/../secret", shareeToken); r.status == http.StatusOK {
			t.Fatal("path traversal out of the shared root must fail")
		}
		// A root the user has no share on.
		if r := sharedList(t, e, driveID, "secret", "secret", shareeToken); r.status == http.StatusOK {
			t.Fatal("listing an unshared root must fail")
		}
	})

	t.Run("sharee downloads files inside the shared root", func(t *testing.T) {
		status, content := sharedDownload(t, e, reportID, driveID, "projects", shareeToken)
		if status != http.StatusOK {
			t.Fatalf("download: status %d body %s", status, content)
		}
		if string(content) != "shared report content" {
			t.Fatalf("download content = %q", string(content))
		}
	})

	t.Run("revoked users lose access immediately", func(t *testing.T) {
		if r := revokeShare(t, e, driveID, "projects", "sharee", ownerToken); r.status != http.StatusOK {
			t.Fatalf("revoke: status %d body %v", r.status, r.body)
		}
		if roots := sharedRoots(t, e, shareeToken); len(roots) != 0 {
			t.Fatalf("expected no shared roots after revoke, got %v", roots)
		}
		if r := sharedList(t, e, driveID, "projects", "projects", shareeToken); r.status == http.StatusOK {
			t.Fatal("revoked user should not be able to list the shared folder")
		}
		if status, _ := sharedDownload(t, e, reportID, driveID, "projects", shareeToken); status == http.StatusOK {
			t.Fatal("revoked user should not be able to download from the shared folder")
		}
	})

	t.Run("re-granting upserts instead of duplicating", func(t *testing.T) {
		for _, access := range []string{"read", "write"} {
			if r := grantShare(t, e, driveID, "projects", "sharee", access, ownerToken); r.status != http.StatusOK {
				t.Fatalf("grant %s: status %d body %v", access, r.status, r.body)
			}
		}
		r := sharedUsers(t, e, driveID, "projects", ownerToken)
		if r.status != http.StatusOK {
			t.Fatalf("list-users: status %d body %v", r.status, r.body)
		}
		users, _ := r.body["users"].([]any)
		if len(users) != 1 {
			t.Fatalf("expected 1 shared user after upsert grants, got %v", users)
		}
		user, _ := users[0].(map[string]any)
		if user["accessLevel"] != string(models.AccessLevelWrite) {
			t.Fatalf("accessLevel = %v, want write", user["accessLevel"])
		}

		// Write shares keep read-only operations working.
		if r := sharedList(t, e, driveID, "projects", "projects", shareeToken); r.status != http.StatusOK {
			t.Fatalf("write-level sharee list: status %d body %v", r.status, r.body)
		}
	})

	t.Run("non-managers cannot administer shares", func(t *testing.T) {
		// sharee has no drive membership at all here.
		if r := grantShare(t, e, driveID, "projects", "sharee", "read", shareeToken); r.status == http.StatusOK {
			t.Fatal("non-member should not be able to grant shares")
		}
		if r := sharedUsers(t, e, driveID, "projects", shareeToken); r.status == http.StatusOK {
			t.Fatal("non-manager should not be able to list shared users")
		}
		if r := revokeShare(t, e, driveID, "projects", "sharee", shareeToken); r.status == http.StatusOK {
			t.Fatal("non-manager should not be able to revoke shares")
		}
	})
}
