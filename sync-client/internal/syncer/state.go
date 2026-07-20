package syncer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"archivus-sync/internal/config"
)

const stateFileName = "state.json"

// fileState records what the client last uploaded for a given local file, so a
// subsequent run can tell whether the content changed.
type fileState struct {
	Size     int64  `json:"size"`
	ModTime  int64  `json:"modTime"` // unix nanoseconds
	Checksum string `json:"checksum"`
}

// state is the persisted sync state, keyed by absolute local file path.
type state struct {
	Files map[string]fileState `json:"files"`
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
		return &state{Files: map[string]fileState{}}, nil
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
	return &s, nil
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
