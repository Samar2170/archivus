package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"archivus/internal/config"
	"archivus/internal/models"
	"archivus/internal/services/auth"
	"archivus/internal/services/storagemanager/diskmanager"
	"archivus/internal/store"
	"archivus/server"
)

// testEnv holds a running test server and its base URL.
type testEnv struct {
	url string
	ts  *httptest.Server
}

// newTestServer spins up a full server backed by a temp SQLite DB and temp
// filesystem directory — no network, no shared state between tests.
func newTestServer(t *testing.T) *testEnv {
	t.Helper()

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".archivus")
	storageHome := filepath.Join(tmpDir, "archivus")

	for _, d := range []string{configDir, storageHome} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}

	config.Config = &config.Configuration{
		DefaultWriteAccess: false,
		AllowUserDrive:     true,
		SecretKey:          "integration-test-secret-key-32ch",
		ArchivusHome:       storageHome,
		ServerSalt:         "test-salt-16chr",
		S3Enabled:          false,
	}
	config.ProjectBaseDir = configDir
	config.UsersDir = filepath.Join(storageHome, "users")
	os.MkdirAll(config.UsersDir, 0755)

	s, err := store.GetStore(configDir)
	if err != nil {
		t.Fatalf("GetStore: %v", err)
	}

	dm := diskmanager.GetDiskManager(s, storageHome)
	as := auth.AuthService{
		Store:              s,
		StorageManager:     dm,
		DefaultWriteAccess: false,
		SecretKey:          "integration-test-secret-key-32ch",
	}

	ts := httptest.NewServer(server.GetServer(&as).Handler)
	t.Cleanup(ts.Close)

	return &testEnv{url: ts.URL, ts: ts}
}

// ---- HTTP helpers ----

type resp struct {
	status int
	body   map[string]any
}

func (e *testEnv) doJSON(t *testing.T, method, path string, payload any, token string) resp {
	t.Helper()
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.url+path, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer r.Body.Close()
	var result map[string]any
	json.NewDecoder(r.Body).Decode(&result) //nolint:errcheck
	return resp{status: r.StatusCode, body: result}
}

func (e *testEnv) uploadFiles(t *testing.T, folderPath, driveID, token string, files map[string][]byte) resp {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("folderPath", folderPath) //nolint:errcheck
	mw.WriteField("driveId", driveID)       //nolint:errcheck
	for name, content := range files {
		fw, err := mw.CreateFormFile("files", name)
		if err != nil {
			t.Fatalf("create form file %q: %v", name, err)
		}
		fw.Write(content) //nolint:errcheck
	}
	mw.Close()

	req, err := http.NewRequest(http.MethodPost, e.url+"/storage/file/upload", &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer r.Body.Close()
	var result map[string]any
	json.NewDecoder(r.Body).Decode(&result) //nolint:errcheck
	return resp{status: r.StatusCode, body: result}
}

func (e *testEnv) downloadFile(t *testing.T, fileID, driveID, token string) (statusCode int, content []byte, contentDisposition string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/storage/file/download?fileId=%s&driveId=%s", e.url, fileID, driveID),
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
	content, _ = io.ReadAll(r.Body)
	return r.StatusCode, content, r.Header.Get("Content-Disposition")
}

// ---- Auth helpers ----

func registerUser(t *testing.T, e *testEnv, username, password, pin, email string, isAdmin bool, inviteCode string) {
	t.Helper()
	payload := map[string]any{
		"username":  username,
		"password":  password,
		"pin":       pin,
		"email":     email,
		"user_type": string(models.UserTypeBusiness),
		"is_admin":  isAdmin,
	}
	if inviteCode != "" {
		payload["invite_code"] = inviteCode
	}
	r := e.doJSON(t, http.MethodPost, "/auth/register", payload, "")
	if r.status != http.StatusOK {
		t.Fatalf("register %q: status %d body %v", username, r.status, r.body)
	}
}

func loginUser(t *testing.T, e *testEnv, username, pin string) string {
	t.Helper()
	r := e.doJSON(t, http.MethodPost, "/auth/login", map[string]any{
		"username": username,
		"pin":      pin,
	}, "")
	if r.status != http.StatusOK {
		t.Fatalf("login %q: status %d body %v", username, r.status, r.body)
	}
	tok, _ := r.body["token"].(string)
	if tok == "" {
		t.Fatalf("login %q: missing token in %v", username, r.body)
	}
	return tok
}

func getUserInfo(t *testing.T, e *testEnv, token string) (userID string, driveIDs []string) {
	t.Helper()
	r := e.doJSON(t, http.MethodGet, "/auth/user/info", nil, token)
	if r.status != http.StatusOK {
		t.Fatalf("user info: status %d body %v", r.status, r.body)
	}
	user, _ := r.body["user"].(map[string]any)
	userID, _ = user["ID"].(string)
	drives, _ := r.body["drives"].([]any)
	for _, d := range drives {
		dm, _ := d.(map[string]any)
		id, _ := dm["DriveID"].(string)
		driveIDs = append(driveIDs, id)
	}
	return userID, driveIDs
}

// ====================================================================
// Test: Health Check
// ====================================================================

func TestHealthCheck(t *testing.T) {
	e := newTestServer(t)
	r := e.doJSON(t, http.MethodGet, "/health", nil, "")
	if r.status != http.StatusOK {
		t.Fatalf("expected 200, got %d body %v", r.status, r.body)
	}
}

// ====================================================================
// Test: Auth flow (mirrors api.ipynb)
// ====================================================================

func TestAuthFlow(t *testing.T) {
	e := newTestServer(t)

	var adminToken string
	var driveID string
	var inviteCode string
	var newUserToken string
	var newUserID string

	t.Run("register admin user", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/auth/register", map[string]any{
			"username":  "samar",
			"password":  "password12",
			"pin":       "123456",
			"email":     "samar@example.com",
			"user_type": string(models.UserTypeBusiness),
			"is_admin":  true,
		}, "")
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		if r.body["message"] != "user created" {
			t.Fatalf("unexpected body: %v", r.body)
		}
	})

	t.Run("login admin", func(t *testing.T) {
		adminToken = loginUser(t, e, "samar", "123456")
	})

	t.Run("get user info returns drive", func(t *testing.T) {
		_, driveIDs := getUserInfo(t, e, adminToken)
		if len(driveIDs) == 0 {
			t.Fatal("expected at least one drive")
		}
		driveID = driveIDs[0]
	})

	t.Run("generate invite code with read access", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/auth/drive/invite", map[string]any{
			"drive_id": driveID,
			"access":   string(models.AccessLevelRead),
		}, adminToken)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		code, _ := r.body["invite_code"].(string)
		if code == "" {
			t.Fatalf("missing invite_code in %v", r.body)
		}
		inviteCode = code
	})

	t.Run("register new user with invite code", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/auth/register", map[string]any{
			"username":    "newuser2",
			"password":    "newuserpass12",
			"pin":         "654321",
			"email":       "newuser2@example.com",
			"user_type":   string(models.UserTypeBusiness),
			"is_admin":    false,
			"invite_code": inviteCode,
		}, "")
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		if r.body["message"] != "user created" {
			t.Fatalf("unexpected body: %v", r.body)
		}
	})

	t.Run("login new user", func(t *testing.T) {
		newUserToken = loginUser(t, e, "newuser2", "654321")
	})

	t.Run("new user info shows drive access", func(t *testing.T) {
		userID, driveIDs := getUserInfo(t, e, newUserToken)
		newUserID = userID
		if len(driveIDs) == 0 {
			t.Fatal("new user should have at least one drive in their list")
		}
		found := false
		for _, id := range driveIDs {
			if id == driveID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("new user drive list %v does not contain invited drive %q", driveIDs, driveID)
		}
	})

	t.Run("drive info shows new user in read users", func(t *testing.T) {
		r := e.doJSON(t, http.MethodGet, "/auth/drive/info?drive_id="+driveID, nil, adminToken)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		readUsers, _ := r.body["readUsers"].([]any)
		found := false
		for _, u := range readUsers {
			user, _ := u.(map[string]any)
			if user["Username"] == "newuser2" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("newuser2 not in readUsers: %v", r.body)
		}
	})

	t.Run("remove new user from drive", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/auth/drive/remove", map[string]any{
			"user_id":  newUserID,
			"drive_id": driveID,
		}, adminToken)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		if r.body["message"] != "user removed from drive" {
			t.Fatalf("unexpected body: %v", r.body)
		}
	})

	t.Run("drive info shows user removed", func(t *testing.T) {
		r := e.doJSON(t, http.MethodGet, "/auth/drive/info?drive_id="+driveID, nil, adminToken)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		readUsers, _ := r.body["readUsers"].([]any)
		for _, u := range readUsers {
			user, _ := u.(map[string]any)
			if user["Username"] == "newuser2" {
				t.Fatal("newuser2 still present in readUsers after removal")
			}
		}
	})

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		r := e.doJSON(t, http.MethodGet, "/auth/user/info", nil, "")
		if r.status != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", r.status)
		}
	})

	t.Run("invalid token is rejected", func(t *testing.T) {
		r := e.doJSON(t, http.MethodGet, "/auth/user/info", nil, "not-a-real-token")
		if r.status != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", r.status)
		}
	})

	t.Run("duplicate registration is rejected", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/auth/register", map[string]any{
			"username":  "samar",
			"password":  "password12",
			"pin":       "123456",
			"email":     "samar2@example.com",
			"user_type": string(models.UserTypeBusiness),
			"is_admin":  true,
		}, "")
		if r.status != http.StatusBadRequest {
			t.Fatalf("expected 400 for duplicate username, got %d body %v", r.status, r.body)
		}
	})

	t.Run("login with wrong pin is rejected", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/auth/login", map[string]any{
			"username": "samar",
			"pin":      "000000",
		}, "")
		if r.status != http.StatusForbidden {
			t.Fatalf("expected 403, got %d body %v", r.status, r.body)
		}
	})

	t.Run("get users in drive (admin only)", func(t *testing.T) {
		r := e.doJSON(t, http.MethodGet, "/auth/drive/users", nil, adminToken)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		if _, ok := r.body["users"]; !ok {
			t.Fatalf("missing 'users' key in response: %v", r.body)
		}
	})
}

// ====================================================================
// Test: File management flow (mirrors file_mgmt.ipynb)
// ====================================================================

func TestFileManagement(t *testing.T) {
	e := newTestServer(t)

	// --- setup: register + login ---
	registerUser(t, e, "samar", "password12", "123456", "samar@example.com", true, "")
	token := loginUser(t, e, "samar", "123456")
	adminUserID, driveIDs := getUserInfo(t, e, token)
	if len(driveIDs) == 0 {
		t.Fatal("admin should have at least one drive")
	}
	driveID := driveIDs[0]

	// The drive owner is not stored in any junction table, so we explicitly add
	// them with write access so CheckUserHasDriveAccess covers them via the
	// standard path (the ownership fast-path now handles it, but this is also
	// valid and exercises the add-user endpoint).
	e.doJSON(t, http.MethodPost, "/auth/drive/add", map[string]any{
		"user_id":      adminUserID,
		"drive_id":     driveID,
		"access_level": string(models.AccessLevelWrite),
	}, token)

	var fileID string

	t.Run("create folder", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/storage/folder/create", map[string]any{
			"path":    "documents/reports",
			"driveId": driveID,
		}, token)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
	})

	t.Run("upload single file", func(t *testing.T) {
		r := e.uploadFiles(t, "documents/reports", driveID, token, map[string][]byte{
			"test.txt": []byte("Hello from integration test"),
		})
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		if r.body["message"] != "files uploaded successfully" {
			t.Fatalf("unexpected body: %v", r.body)
		}
	})

	t.Run("upload multiple files", func(t *testing.T) {
		r := e.uploadFiles(t, "documents/reports", driveID, token, map[string][]byte{
			"alpha.txt": []byte("file alpha content"),
			"beta.txt":  []byte("file beta content"),
		})
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		if r.body["message"] != "files uploaded successfully" {
			t.Fatalf("unexpected body: %v", r.body)
		}
	})

	t.Run("list files in folder", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/storage/files", map[string]any{
			"path":    "documents/reports",
			"driveId": driveID,
		}, token)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		files, ok := r.body["files"].([]any)
		if !ok || len(files) == 0 {
			t.Fatalf("expected non-empty files list, got %v", r.body)
		}
		// grab a non-directory file ID for the download test
		for _, f := range files {
			entry, _ := f.(map[string]any)
			if isDir, _ := entry["IsDir"].(bool); !isDir {
				fileID, _ = entry["ID"].(string)
				break
			}
		}
	})

	t.Run("download file", func(t *testing.T) {
		if fileID == "" {
			t.Skip("no file ID from list step; skipping download test")
		}
		status, content, cd := e.downloadFile(t, fileID, driveID, token)
		if status != http.StatusOK {
			t.Fatalf("status %d body %s", status, content)
		}
		if len(content) == 0 {
			t.Fatal("downloaded file content is empty")
		}
		if !strings.Contains(cd, "attachment") {
			t.Fatalf("Content-Disposition should contain 'attachment', got %q", cd)
		}
	})

	t.Run("list root of drive shows documents folder", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/storage/files", map[string]any{
			"path":    "",
			"driveId": driveID,
		}, token)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		files, _ := r.body["files"].([]any)
		found := false
		for _, f := range files {
			entry, _ := f.(map[string]any)
			if entry["Name"] == "documents" {
				isDir, _ := entry["IsDir"].(bool)
				if !isDir {
					t.Fatal("'documents' entry should be a directory")
				}
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("'documents' directory not found in root listing: %v", files)
		}
	})

	t.Run("read user cannot upload", func(t *testing.T) {
		// Register a read-only user via invite
		r := e.doJSON(t, http.MethodPost, "/auth/drive/invite", map[string]any{
			"drive_id": driveID,
			"access":   string(models.AccessLevelRead),
		}, token)
		if r.status != http.StatusOK {
			t.Fatalf("invite status %d body %v", r.status, r.body)
		}
		code, _ := r.body["invite_code"].(string)

		registerUser(t, e, "readonly", "readonly12", "111111", "ro@example.com", false, code)
		roToken := loginUser(t, e, "readonly", "111111")

		ro := e.uploadFiles(t, "documents/reports", driveID, roToken, map[string][]byte{
			"sneaky.txt": []byte("should not upload"),
		})
		if ro.status == http.StatusOK {
			t.Fatal("read-only user should not be able to upload files")
		}
	})

	t.Run("delete folder", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/storage/folder/delete", map[string]any{
			"path":    "documents/reports",
			"driveId": driveID,
		}, token)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		if r.body["message"] != "folder deleted" {
			t.Fatalf("unexpected body: %v", r.body)
		}
	})

	t.Run("deleted subfolder no longer appears in parent listing", func(t *testing.T) {
		// We deleted documents/reports, not documents itself.
		// Listing documents/ should now return an empty (or error) result for reports.
		r := e.doJSON(t, http.MethodPost, "/storage/files", map[string]any{
			"path":    "documents",
			"driveId": driveID,
		}, token)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		files, _ := r.body["files"].([]any)
		for _, f := range files {
			entry, _ := f.(map[string]any)
			if entry["Name"] == "reports" {
				t.Fatal("'reports' subfolder should be gone after deletion")
			}
		}
	})
}
