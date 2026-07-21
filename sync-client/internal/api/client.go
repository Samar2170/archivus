// Package api is a thin HTTP client for the archivus backend. It speaks the same
// endpoints the web frontend uses: /auth/login for a bearer token and
// /storage/file/upload for multipart uploads.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Client is an authenticated backend client.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// New builds a client for baseURL using the given bearer token.
func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: 10 * time.Minute}, // uploads can be large
	}
}

// errorBody is the shape the backend returns for non-2xx responses.
type errorBody struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

func decodeError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	var eb errorBody
	if json.Unmarshal(data, &eb) == nil && eb.Error != "" {
		return fmt.Errorf("%s: %s", resp.Status, eb.Error)
	}
	return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(data)))
}

// Login exchanges credentials for a bearer token. It is a standalone function
// because it runs before a Client (and its token) exist.
func Login(ctx context.Context, baseURL, username, password, pin string) (string, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	body, err := json.Marshal(map[string]string{
		"username": username,
		"password": password,
		"pin":      pin,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/auth/login", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", decodeError(resp)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode login response: %w", err)
	}
	if out.Token == "" {
		return "", fmt.Errorf("login succeeded but no token returned")
	}
	return out.Token, nil
}

// UploadFile streams localPath to the drive under folderPath. folderPath uses
// forward slashes and is relative to the drive root ("" means the drive root).
// The stored filename is the base name of localPath.
func (c *Client) UploadFile(ctx context.Context, driveID, folderPath, localPath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open %q: %w", localPath, err)
	}
	defer f.Close()

	// Stream the multipart body through a pipe so large files are never buffered
	// fully in memory.
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		var werr error
		defer func() { _ = pw.CloseWithError(werr) }()
		if werr = mw.WriteField("driveId", driveID); werr != nil {
			return
		}
		if werr = mw.WriteField("folderPath", folderPath); werr != nil {
			return
		}
		part, err := mw.CreateFormFile("files", filepath.Base(localPath))
		if err != nil {
			werr = err
			return
		}
		if _, werr = io.Copy(part, f); werr != nil {
			return
		}
		werr = mw.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/storage/file/upload", pr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("%w: %s", ErrUnauthorized, decodeError(resp))
	}
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// ErrUnauthorized is returned when the server rejects the token, so callers can
// prompt the user to log in again.
var ErrUnauthorized = fmt.Errorf("unauthorized (token missing, invalid, or expired)")

// Health pings the backend to verify the URL is reachable.
func (c *Client) Health(ctx context.Context) error {
	u, err := url.JoinPath(c.BaseURL, "health")
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return decodeError(resp)
	}
	return nil
}
