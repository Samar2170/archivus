package s3manager

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"
)

func newPresignClient(t *testing.T) *Client {
	t.Helper()
	c, err := New("test-account", "test-access-key", "test-secret-key", "test-bucket")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// TestPresignGetObjectDownload checks the save-as flavour pins an attachment
// disposition and is a valid signed URL.
func TestPresignGetObjectDownload(t *testing.T) {
	c := newPresignClient(t)
	raw, err := c.PresignGetObjectDownload(context.Background(), c.BucketName, "drive/docs/report.pdf", "report.pdf", time.Minute)
	if err != nil {
		t.Fatalf("PresignGetObjectDownload: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	q := u.Query()
	if q.Get("X-Amz-Signature") == "" {
		t.Fatalf("missing X-Amz-Signature in %q", raw)
	}
	if !strings.Contains(raw, "drive/docs/report.pdf") {
		t.Fatalf("key missing from url %q", raw)
	}
	if got := q.Get("response-content-disposition"); got != "attachment; filename=report.pdf" {
		t.Fatalf("response-content-disposition = %q", got)
	}
}

// TestPresignGetObjectPreview checks the inline flavour renders in place and
// pins the content type so octet-stream uploads still preview.
func TestPresignGetObjectPreview(t *testing.T) {
	c := newPresignClient(t)
	raw, err := c.PresignGetObjectPreview(context.Background(), c.BucketName, "drive/docs/photo.png", "image/png", time.Minute)
	if err != nil {
		t.Fatalf("PresignGetObjectPreview: %v", err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	q := u.Query()
	if q.Get("X-Amz-Signature") == "" {
		t.Fatalf("missing X-Amz-Signature in %q", raw)
	}
	if got := q.Get("response-content-disposition"); got != "inline" {
		t.Fatalf("response-content-disposition = %q", got)
	}
	if got := q.Get("response-content-type"); got != "image/png" {
		t.Fatalf("response-content-type = %q", got)
	}
}
