package integration_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	archivus_constants "archivus/internal/constants"
	"archivus/internal/models"
)

// doJSONWithAPIKey mirrors testEnv.doJSON but authenticates with an API key
// instead of a Bearer token.
func doJSONWithAPIKey(t *testing.T, e *testEnv, method, path string, payload any, apiKey string) resp {
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
	req.Header.Set(archivus_constants.ApiKeyHeader, apiKey)

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer r.Body.Close()
	var result map[string]any
	json.NewDecoder(r.Body).Decode(&result) //nolint:errcheck
	return resp{status: r.StatusCode, body: result}
}

// ====================================================================
// Test: API key authentication
// ====================================================================

func TestApiKeyAuth(t *testing.T) {
	e, s := newTestServerWithStore(t)

	registerUser(t, e, "samar", "password12", "123456", "samar@example.com", true, "")
	token := loginUser(t, e, "samar", "123456")
	adminUserID, driveIDs := getUserInfo(t, e, token)
	if len(driveIDs) == 0 {
		t.Fatal("admin should have at least one drive")
	}
	driveID := driveIDs[0]

	// Mirror TestFileManagement: give the owner explicit write access on the
	// drive so folder creation is granted via the standard junction path.
	e.doJSON(t, http.MethodPost, "/auth/drive/add", map[string]any{
		"user_id":      adminUserID,
		"drive_id":     driveID,
		"access_level": string(models.AccessLevelWrite),
	}, token)

	var writeKey, writeKeyID, readKey, readKeyID string

	t.Run("create write api key", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/auth/apikey/create", map[string]any{
			"name":   "ci-deploy",
			"access": string(models.AccessLevelWrite),
		}, token)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		writeKey, _ = r.body["api_key"].(string)
		writeKeyID, _ = r.body["id"].(string)
		if writeKey == "" || writeKeyID == "" {
			t.Fatalf("missing api_key/id in %v", r.body)
		}
		if r.body["access_level"] != string(models.AccessLevelWrite) {
			t.Fatalf("unexpected access_level: %v", r.body["access_level"])
		}
		expiresAt, _ := r.body["expires_at"].(string)
		exp, err := time.Parse(time.RFC3339, expiresAt)
		if err != nil {
			t.Fatalf("invalid expires_at %q: %v", expiresAt, err)
		}
		days := time.Until(exp).Hours() / 24
		if days < 89 || days > 91 {
			t.Fatalf("expected ~90 day validity, got %.1f days", days)
		}
		if createdAt, _ := r.body["created_at"].(string); createdAt == "" || strings.HasPrefix(createdAt, "0001-") {
			t.Fatalf("created_at not populated: %v", r.body["created_at"])
		}
	})

	t.Run("access level defaults to read", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/auth/apikey/create", map[string]any{
			"name": "backup",
		}, token)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		readKey, _ = r.body["api_key"].(string)
		readKeyID, _ = r.body["id"].(string)
		if readKey == "" || readKeyID == "" {
			t.Fatalf("missing api_key/id in %v", r.body)
		}
		if r.body["access_level"] != string(models.AccessLevelRead) {
			t.Fatalf("expected default read access, got %v", r.body["access_level"])
		}
	})

	t.Run("unsupported access level is rejected", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/auth/apikey/create", map[string]any{
			"name":   "oops",
			"access": string(models.AccessLevelOwner),
		}, token)
		if r.status != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body %v", r.status, r.body)
		}
	})

	t.Run("oversized name is rejected", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/auth/apikey/create", map[string]any{
			"name": string(make([]byte, archivus_constants.ApiKeyNameMaxLength+1)),
		}, token)
		if r.status != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body %v", r.status, r.body)
		}
	})

	t.Run("api key authenticates a protected route", func(t *testing.T) {
		r := doJSONWithAPIKey(t, e, http.MethodGet, "/auth/user/info", nil, writeKey)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
	})

	t.Run("read api key cannot perform write requests", func(t *testing.T) {
		r := doJSONWithAPIKey(t, e, http.MethodPost, "/storage/folder/create", map[string]any{
			"path":    "apikey-folder",
			"driveId": driveID,
		}, readKey)
		if r.status != http.StatusForbidden {
			t.Fatalf("expected 403 for write with read key, got %d body %v", r.status, r.body)
		}
	})

	t.Run("write api key can perform write requests", func(t *testing.T) {
		r := doJSONWithAPIKey(t, e, http.MethodPost, "/storage/folder/create", map[string]any{
			"path":    "apikey-folder",
			"driveId": driveID,
		}, writeKey)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
	})

	t.Run("invalid api key is rejected", func(t *testing.T) {
		r := doJSONWithAPIKey(t, e, http.MethodGet, "/auth/user/info", nil, "not-a-real-key")
		if r.status != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", r.status)
		}
	})

	t.Run("list api keys hides the plaintext", func(t *testing.T) {
		r := e.doJSON(t, http.MethodGet, "/auth/apikey/list", nil, token)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		keys, _ := r.body["api_keys"].([]any)
		if len(keys) != 2 {
			t.Fatalf("expected 2 api keys, got %d: %v", len(keys), r.body)
		}
		for _, k := range keys {
			km, _ := k.(map[string]any)
			if km["api_key"] != nil {
				t.Fatal("list response must not contain the api_key plaintext")
			}
		}
	})

	t.Run("revoked api key is rejected", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/auth/apikey/revoke", map[string]any{
			"key_id": readKeyID,
		}, token)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		r = doJSONWithAPIKey(t, e, http.MethodGet, "/auth/user/info", nil, readKey)
		if r.status != http.StatusForbidden {
			t.Fatalf("expected 403 after revocation, got %d", r.status)
		}
	})

	t.Run("cannot revoke another user's api key", func(t *testing.T) {
		registerUser(t, e, "intruder", "intruder12", "999999", "intruder@example.com", true, "")
		intruderToken := loginUser(t, e, "intruder", "999999")

		r := e.doJSON(t, http.MethodPost, "/auth/apikey/revoke", map[string]any{
			"key_id": writeKeyID,
		}, intruderToken)
		if r.status != http.StatusNotFound {
			t.Fatalf("expected 404, got %d body %v", r.status, r.body)
		}

		r = doJSONWithAPIKey(t, e, http.MethodGet, "/auth/user/info", nil, writeKey)
		if r.status != http.StatusOK {
			t.Fatalf("write key should still work after foreign revocation attempt, got %d", r.status)
		}
	})

	t.Run("expired api key is rejected", func(t *testing.T) {
		r := e.doJSON(t, http.MethodPost, "/auth/apikey/create", map[string]any{
			"name":   "old-key",
			"access": string(models.AccessLevelRead),
		}, token)
		if r.status != http.StatusOK {
			t.Fatalf("status %d body %v", r.status, r.body)
		}
		keyID, _ := r.body["id"].(string)
		keyPlain, _ := r.body["api_key"].(string)

		// Backdate the expiry; the API itself only issues 90-day keys.
		if err := s.DB.Model(&models.ApiKey{}).Where("id = ?", keyID).
			Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
			t.Fatalf("backdate expiry: %v", err)
		}

		r = doJSONWithAPIKey(t, e, http.MethodGet, "/auth/user/info", nil, keyPlain)
		if r.status != http.StatusForbidden {
			t.Fatalf("expected 403 for expired key, got %d", r.status)
		}
	})
}
