package syncer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"archivus-sync/internal/api"
	"archivus-sync/internal/config"
)

const stateFileName = "state.json"

// pendingUploadTTL bounds how long a stored chunked-upload session is trusted.
// The server never expires sessions on its own, so past this age the client
// aborts the session instead of resuming it — which is also what keeps
// abandoned staging directories from accumulating server-side.
const pendingUploadTTL = 7 * 24 * time.Hour

// fileState records what the client last uploaded for a given local file, so a
// subsequent run can tell whether the content changed.
type fileState struct {
	Size     int64  `json:"size"`
	ModTime  int64  `json:"modTime"` // unix nanoseconds
	Checksum string `json:"checksum"`
}

// pendingUpload records a chunked upload session that was started but not
// finished, so a later run can resume it instead of re-sending the whole file.
// The recorded content hash and destination are what make resuming safe: if any
// of them no longer match the local file, the session describes different bytes
// and is discarded rather than resumed.
type pendingUpload struct {
	UploadID    string `json:"uploadId"`
	ChunkSize   int64  `json:"chunkSize"`
	TotalChunks int    `json:"totalChunks"`
	Size        int64  `json:"size"`
	Checksum    string `json:"checksum"`
	DriveID     string `json:"driveId"`
	FolderPath  string `json:"folderPath"`
	StartedAt   int64  `json:"startedAt"` // unix seconds
}

// session converts the record back into the form the API client resumes from.
func (p pendingUpload) session() api.ChunkUploadSession {
	return api.ChunkUploadSession{
		UploadID:    p.UploadID,
		ChunkSize:   p.ChunkSize,
		TotalChunks: p.TotalChunks,
	}
}

// resumable reports whether the session still describes this exact file going to
// this exact destination, and is recent enough to be worth trying.
func (p pendingUpload) resumable(size int64, checksum, driveID, folderPath string, now time.Time) bool {
	if p.UploadID == "" || p.Size != size || p.Checksum != checksum {
		return false
	}
	if p.DriveID != driveID || p.FolderPath != folderPath {
		return false
	}
	return now.Sub(time.Unix(p.StartedAt, 0)) < pendingUploadTTL
}

// state is the persisted sync state, keyed by absolute local file path.
type state struct {
	Files   map[string]fileState     `json:"files"`
	Pending map[string]pendingUpload `json:"pending,omitempty"`
}

func statePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, stateFileName), nil
}

func loadState() (*state, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return newState(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state %q: %w", path, err)
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse state %q: %w", path, err)
	}
	if s.Files == nil {
		s.Files = map[string]fileState{}
	}
	if s.Pending == nil {
		s.Pending = map[string]pendingUpload{}
	}
	return &s, nil
}

func newState() *state {
	return &state{
		Files:   map[string]fileState{},
		Pending: map[string]pendingUpload{},
	}
}

func (s *state) save() error {
	path, err := statePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write state %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace state %q: %w", path, err)
	}
	return nil
}

// sha256File returns the hex-encoded SHA-256 of the file's contents.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
