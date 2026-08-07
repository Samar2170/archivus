package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Resumable chunked upload, mirroring the backend's
// /storage/file/upload/chunk/{init,part,status,complete,abort} flow: init a
// session, send each chunk, then complete to have the server assemble and
// persist the file. Chunk writes are idempotent server-side, so a chunk that
// fails mid-flight can simply be re-sent — and because the session survives on
// the server, a caller that persists the ChunkUploadSession can resume it in a
// later process.

const (
	// DefaultChunkSize is how much of a file goes in a single chunk request. It
	// is well under the backend's 64 MB per-request cap so the multipart framing
	// can never push a request over the limit.
	DefaultChunkSize = 8 << 20 // 8 MB

	// maxChunkAttempts is how many times a single chunk is sent before the
	// upload is given up on for this run.
	maxChunkAttempts = 3

	// DefaultRetryBackoff is the base delay between attempts at the same chunk;
	// it is multiplied by the attempt number.
	DefaultRetryBackoff = 2 * time.Second
)

// ErrSessionGone reports that the server no longer knows about an upload
// session — it was completed, aborted, or never existed. A caller holding a
// stored session should discard it and start the file over.
var ErrSessionGone = errors.New("upload session no longer exists on the server")

// ChunkUploadSession identifies an in-progress chunked upload. It is safe to
// persist and use from a later process: everything needed to resume is here,
// and ChunkSize is carried explicitly so a changed client default can never
// silently shift the chunk boundaries of an existing session.
type ChunkUploadSession struct {
	UploadID    string `json:"uploadId"`
	ChunkSize   int64  `json:"chunkSize"`
	TotalChunks int    `json:"totalChunks"`
}

// StartChunkedUpload registers a session for localPath and returns it. No file
// bytes are sent yet; pass the session to ResumeChunkedUpload to transfer them.
// Callers that want to survive a crash should persist the session first.
func (c *Client) StartChunkedUpload(ctx context.Context, driveID, folderPath, localPath string) (ChunkUploadSession, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return ChunkUploadSession{}, fmt.Errorf("stat %q: %w", localPath, err)
	}
	chunkSize := c.chunkSize()
	totalChunks := chunkCount(info.Size(), chunkSize)
	filename := filepath.Base(localPath)

	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req := map[string]any{
		"filename":    filename,
		"driveId":     driveID,
		"folderPath":  folderPath,
		"contentType": contentType,
		"size":        info.Size(),
		"totalChunks": totalChunks,
	}
	var out struct {
		UploadID string `json:"uploadId"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/storage/file/upload/chunk/init", req, &out); err != nil {
		return ChunkUploadSession{}, fmt.Errorf("init chunked upload: %w", err)
	}
	if out.UploadID == "" {
		return ChunkUploadSession{}, errors.New("init chunked upload: server returned no uploadId")
	}
	return ChunkUploadSession{UploadID: out.UploadID, ChunkSize: chunkSize, TotalChunks: totalChunks}, nil
}

// ResumeChunkedUpload sends whatever chunks of localPath the server is still
// missing and then completes the upload. It is the same call whether the
// session was created moments ago or by an earlier run: the server's status
// response is the source of truth for what has already landed.
//
// It returns an error wrapping ErrSessionGone if the session no longer exists
// server-side or no longer describes this file, so callers can drop a stale
// stored session and start over. On any other failure the session is left
// intact on the server, ready for another attempt.
func (c *Client) ResumeChunkedUpload(ctx context.Context, sess ChunkUploadSession, localPath string) error {
	if sess.UploadID == "" || sess.ChunkSize <= 0 || sess.TotalChunks <= 0 {
		return fmt.Errorf("%w: incomplete session record", ErrSessionGone)
	}
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open %q: %w", localPath, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %q: %w", localPath, err)
	}
	// The file must still chunk up exactly the way the session was created for;
	// otherwise the indexes already on the server mean something different.
	if got := chunkCount(info.Size(), sess.ChunkSize); got != sess.TotalChunks {
		return fmt.Errorf("%w: file now needs %d chunks, session has %d", ErrSessionGone, got, sess.TotalChunks)
	}

	received, serverTotal, err := c.chunkStatus(ctx, sess.UploadID)
	if err != nil {
		return err
	}
	if serverTotal != sess.TotalChunks {
		return fmt.Errorf("%w: server reports %d chunks, session has %d", ErrSessionGone, serverTotal, sess.TotalChunks)
	}

	if err := c.sendChunks(ctx, sess, f, info.Size(), received); err != nil {
		return err
	}
	return c.completeChunkUpload(ctx, sess.UploadID)
}

// chunkCount is how many chunks a file of the given size splits into. An empty
// file still needs one (empty) chunk: the server rejects totalChunks <= 0.
func chunkCount(size, chunkSize int64) int {
	n := int((size + chunkSize - 1) / chunkSize)
	if n == 0 {
		n = 1
	}
	return n
}

func (c *Client) chunkSize() int64 {
	if c.ChunkSize > 0 {
		return c.ChunkSize
	}
	return DefaultChunkSize
}

func (c *Client) retryBackoff() time.Duration {
	if c.RetryBackoff > 0 {
		return c.RetryBackoff
	}
	return DefaultRetryBackoff
}

// sendChunks pushes every chunk of f that the server does not already have.
// After a failed attempt it re-reads the session status, so a retry re-sends
// only what is actually missing.
func (c *Client) sendChunks(ctx context.Context, sess ChunkUploadSession, f *os.File, size int64, received map[int]bool) error {
	for i := range sess.TotalChunks {
		if received[i] {
			continue
		}
		offset := int64(i) * sess.ChunkSize
		n := sess.ChunkSize
		if remaining := size - offset; remaining < n {
			n = remaining
		}

		var err error
		for attempt := 1; attempt <= maxChunkAttempts; attempt++ {
			// A fresh SectionReader per attempt: retries must start from the
			// beginning of the chunk, not wherever the failed attempt stopped.
			err = c.uploadChunk(ctx, sess.UploadID, i, io.NewSectionReader(f, offset, n))
			if err == nil {
				break
			}
			if ctx.Err() != nil || errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrSessionGone) {
				return err // retrying cannot help
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.retryBackoff() * time.Duration(attempt)):
			}
			// The failed request may still have landed; ask the server what it
			// has before spending another attempt on this chunk.
			if got, _, statusErr := c.chunkStatus(ctx, sess.UploadID); statusErr == nil {
				received = got
				if received[i] {
					err = nil
					break
				}
			}
		}
		if err != nil {
			return fmt.Errorf("chunk %d/%d: %w", i+1, sess.TotalChunks, err)
		}
	}
	return nil
}

// uploadChunk sends a single chunk. The bytes go in the multipart file field
// "chunk"; re-sending an index simply overwrites it server-side.
func (c *Client) uploadChunk(ctx context.Context, uploadID string, index int, body io.Reader) error {
	// Stream the multipart body through a pipe so a chunk is never held in
	// memory twice.
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		var werr error
		defer func() { _ = pw.CloseWithError(werr) }()
		if werr = mw.WriteField("uploadId", uploadID); werr != nil {
			return
		}
		if werr = mw.WriteField("chunkIndex", strconv.Itoa(index)); werr != nil {
			return
		}
		part, err := mw.CreateFormFile("chunk", fmt.Sprintf("chunk-%d", index))
		if err != nil {
			werr = err
			return
		}
		if _, werr = io.Copy(part, body); werr != nil {
			return
		}
		werr = mw.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/storage/file/upload/chunk/part", pr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("upload chunk request: %w", err)
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return sessionErr(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// chunkStatus returns the set of chunk indexes the server has received, along
// with the chunk count it recorded for the session.
func (c *Client) chunkStatus(ctx context.Context, uploadID string) (map[int]bool, int, error) {
	var out struct {
		Received    []int `json:"received"`
		TotalChunks int   `json:"totalChunks"`
	}
	path := "/storage/file/upload/chunk/status?uploadId=" + url.QueryEscape(uploadID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, 0, fmt.Errorf("chunk upload status: %w", sessionErr(err))
	}
	received := make(map[int]bool, len(out.Received))
	for _, i := range out.Received {
		received[i] = true
	}
	return received, out.TotalChunks, nil
}

// completeChunkUpload asks the server to assemble the chunks and persist the
// file. The server may finish the storage write asynchronously; a successful
// response means the upload was accepted in full.
func (c *Client) completeChunkUpload(ctx context.Context, uploadID string) error {
	req := map[string]string{"uploadId": uploadID}
	if err := c.doJSON(ctx, http.MethodPost, "/storage/file/upload/chunk/complete", req, nil); err != nil {
		return fmt.Errorf("complete chunked upload: %w", sessionErr(err))
	}
	return nil
}

// AbortChunkUpload discards an unfinished session and its staged chunks. A
// session the server has already forgotten is treated as successfully aborted.
func (c *Client) AbortChunkUpload(ctx context.Context, uploadID string) error {
	req := map[string]string{"uploadId": uploadID}
	if err := c.doJSON(ctx, http.MethodPost, "/storage/file/upload/chunk/abort", req, nil); err != nil {
		if errors.Is(sessionErr(err), ErrSessionGone) {
			return nil
		}
		return fmt.Errorf("abort chunked upload: %w", err)
	}
	return nil
}

// sessionErr re-tags a 404 from a chunk endpoint as ErrSessionGone; other
// errors pass through unchanged.
func sessionErr(err error) error {
	var se *statusError
	if errors.As(err, &se) && se.code == http.StatusNotFound {
		return fmt.Errorf("%w: %v", ErrSessionGone, err)
	}
	return err
}

// doJSON sends an optional JSON body to path and decodes the JSON response into
// out (which may be nil to discard it).
func (c *Client) doJSON(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := checkStatus(resp); err != nil {
		return err
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
