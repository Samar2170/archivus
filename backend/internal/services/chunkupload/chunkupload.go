// Package chunkupload manages resumable, chunked file uploads. A client splits a
// large file into fixed-size chunks and uploads them one at a time against an
// upload session. Chunks are staged on local disk (independent of the eventual
// storage backend) so that a network drop mid-upload only loses in-flight
// chunks: the client can query which chunks landed and re-send the rest. Once
// every chunk is present the session is assembled into a single file that the
// caller persists through the normal storage path.
package chunkupload

import (
	archivus_constants "archivus/internal/constants"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const manifestFileName = "manifest.json"

// assembledPrefix names the temp files Assemble writes into the staging root.
// The GC uses it to tell its own leftovers from anything else that may be in
// there, so the two must stay in step.
const assembledPrefix = "assembled-"

// ErrSessionNotFound is returned when an uploadId does not match a live session.
var ErrSessionNotFound = errors.New("chunkupload: session not found")

// Session is the persisted description of an in-progress chunked upload. It is
// written to the session's staging directory as manifest.json at init time and
// read back on every subsequent request.
type Session struct {
	UploadID    string    `json:"uploadId"`
	UserID      string    `json:"userId"`
	DriveID     string    `json:"driveId"`
	FolderPath  string    `json:"folderPath"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"contentType"`
	Size        int64     `json:"size"`
	TotalChunks int       `json:"totalChunks"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Manager stages chunked uploads under a single root directory. It is safe for
// concurrent use across sessions: each session lives in its own directory keyed
// by a server-generated UUID, and per-chunk writes are atomic (temp file +
// rename), so a re-sent chunk simply overwrites the previous one.
type Manager struct {
	stagingRoot string
}

// NewManager returns a Manager staging uploads under archivusHome/.chunks. The
// directory is created lazily as sessions are initialised.
func NewManager(archivusHome string) *Manager {
	return &Manager{
		stagingRoot: filepath.Join(archivusHome, archivus_constants.ChunkStagingDirName),
	}
}

// sessionDir returns the staging directory for uploadId after validating that
// uploadId is a well-formed UUID. The UUID check is what keeps uploadId from
// escaping the staging root via path traversal.
func (m *Manager) sessionDir(uploadID string) (string, error) {
	id, err := uuid.Parse(uploadID)
	if err != nil {
		return "", fmt.Errorf("chunkupload: invalid uploadId: %w", err)
	}
	return filepath.Join(m.stagingRoot, id.String()), nil
}

// Init creates a new upload session from the client-supplied metadata, assigns
// it a fresh uploadId, and persists its manifest. The returned Session carries
// the generated UploadID.
func (m *Manager) Init(s Session) (Session, error) {
	if s.TotalChunks <= 0 {
		return Session{}, errors.New("chunkupload: totalChunks must be positive")
	}
	s.UploadID = uuid.NewString()
	s.CreatedAt = time.Now().UTC()
	// filepath.Base strips any directory component a client may smuggle in the
	// filename; the eventual storage path is derived from FolderPath + this name.
	s.Filename = filepath.Base(s.Filename)

	dir := filepath.Join(m.stagingRoot, s.UploadID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Session{}, fmt.Errorf("chunkupload: create session dir: %w", err)
	}
	if err := m.writeManifest(dir, s); err != nil {
		_ = os.RemoveAll(dir)
		return Session{}, err
	}
	return s, nil
}

func (m *Manager) writeManifest(dir string, s Session) error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("chunkupload: marshal manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), data, 0o644); err != nil {
		return fmt.Errorf("chunkupload: write manifest: %w", err)
	}
	return nil
}

// Load reads the manifest for uploadId. It returns ErrSessionNotFound if the
// session does not exist (e.g. it was already completed, aborted, or never
// created).
func (m *Manager) Load(uploadID string) (Session, error) {
	dir, err := m.sessionDir(uploadID)
	if err != nil {
		return Session{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, manifestFileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, fmt.Errorf("chunkupload: read manifest: %w", err)
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return Session{}, fmt.Errorf("chunkupload: unmarshal manifest: %w", err)
	}
	return s, nil
}

// chunkName is the on-disk filename for a chunk at a given index.
func chunkName(index int) string { return fmt.Sprintf("%d.part", index) }

// SaveChunk stores the chunk at index for the given session. The write is atomic
// and idempotent: re-sending a chunk (e.g. after a client-side retry) overwrites
// the previous copy, so callers can safely replay any chunk. index must be in
// [0, TotalChunks).
func (m *Manager) SaveChunk(sess Session, index int, r io.Reader) error {
	if index < 0 || index >= sess.TotalChunks {
		return fmt.Errorf("chunkupload: chunk index %d out of range [0,%d)", index, sess.TotalChunks)
	}
	dir, err := m.sessionDir(sess.UploadID)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, chunkName(index)+".*.tmp")
	if err != nil {
		return fmt.Errorf("chunkupload: create temp chunk: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("chunkupload: write chunk %d: %w", index, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("chunkupload: close chunk %d: %w", index, err)
	}
	if err := os.Rename(tmpName, filepath.Join(dir, chunkName(index))); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("chunkupload: commit chunk %d: %w", index, err)
	}
	return nil
}

// ReceivedChunks returns the sorted indexes of chunks that have been fully
// received for the session. Clients use this to resume after a disconnect,
// re-sending only the missing indexes.
func (m *Manager) ReceivedChunks(sess Session) ([]int, error) {
	dir, err := m.sessionDir(sess.UploadID)
	if err != nil {
		return nil, err
	}
	received := make([]int, 0, sess.TotalChunks)
	for i := 0; i < sess.TotalChunks; i++ {
		if _, err := os.Stat(filepath.Join(dir, chunkName(i))); err == nil {
			received = append(received, i)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("chunkupload: stat chunk %d: %w", i, err)
		}
	}
	sort.Ints(received)
	return received, nil
}

// Assemble concatenates all chunks of a complete session into a single temp file
// and returns it seeked to the start. It errors if any chunk is missing. The
// caller owns the returned file and must Close it and remove it from disk
// (os.Remove(f.Name())) once done; it lives outside the session directory so it
// survives Discard.
func (m *Manager) Assemble(sess Session) (*os.File, error) {
	received, err := m.ReceivedChunks(sess)
	if err != nil {
		return nil, err
	}
	if len(received) != sess.TotalChunks {
		return nil, fmt.Errorf("chunkupload: incomplete upload: have %d/%d chunks", len(received), sess.TotalChunks)
	}
	dir, err := m.sessionDir(sess.UploadID)
	if err != nil {
		return nil, err
	}
	out, err := os.CreateTemp(m.stagingRoot, assembledPrefix+"*.bin")
	if err != nil {
		return nil, fmt.Errorf("chunkupload: create assembly file: %w", err)
	}
	for i := 0; i < sess.TotalChunks; i++ {
		if err := appendChunk(out, filepath.Join(dir, chunkName(i))); err != nil {
			out.Close()
			os.Remove(out.Name())
			return nil, err
		}
	}
	if _, err := out.Seek(0, io.SeekStart); err != nil {
		out.Close()
		os.Remove(out.Name())
		return nil, fmt.Errorf("chunkupload: seek assembly file: %w", err)
	}
	return out, nil
}

func appendChunk(dst *os.File, chunkPath string) error {
	src, err := os.Open(chunkPath)
	if err != nil {
		return fmt.Errorf("chunkupload: open chunk %q: %w", chunkPath, err)
	}
	defer src.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("chunkupload: append chunk %q: %w", chunkPath, err)
	}
	return nil
}

// StaleSweep counts what a PurgeStale pass reclaimed.
type StaleSweep struct {
	Sessions  int
	Assembled int
}

// Empty reports whether the pass found nothing to reclaim.
func (s StaleSweep) Empty() bool { return s.Sessions == 0 && s.Assembled == 0 }

// PurgeStale reclaims staging that no upload is coming back for. Without it the
// staging directory only ever grows: a client that abandons an upload and never
// returns leaves its chunks behind forever, and nothing else on the server is
// watching for that.
//
// referenced is the set of staging paths that file rows still point at; those
// are never touched however old they are. Passing an empty set would make every
// assembled file look like garbage, so callers must build it from the database,
// not skip it.
//
// A failure on one entry does not stop the sweep: the returned count reflects
// what was actually removed, alongside the joined errors for what was not.
func (m *Manager) PurgeStale(now time.Time, referenced map[string]bool) (StaleSweep, error) {
	entries, err := os.ReadDir(m.stagingRoot)
	if errors.Is(err, os.ErrNotExist) {
		return StaleSweep{}, nil // nothing has ever been staged
	}
	if err != nil {
		return StaleSweep{}, fmt.Errorf("chunkupload: read staging root %q: %w", m.stagingRoot, err)
	}

	var swept StaleSweep
	var errs []error
	for _, entry := range entries {
		path := filepath.Join(m.stagingRoot, entry.Name())
		stale, err := m.isStale(entry, path, now, referenced)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !stale {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			errs = append(errs, fmt.Errorf("chunkupload: remove stale %q: %w", path, err))
			continue
		}
		if entry.IsDir() {
			swept.Sessions++
		} else {
			swept.Assembled++
		}
	}
	return swept, errors.Join(errs...)
}

// isStale decides whether one entry in the staging root is garbage.
//
// A session is judged by the CreatedAt in its manifest — when the client started
// the upload — rather than by whatever last wrote to the directory, so a session
// cannot keep itself alive by dribbling in chunks forever. A directory that
// never got a manifest crashed during Init and has no CreatedAt to go on, so it
// falls back to its own timestamp.
//
// Assembled files work differently. Ownership passes to the storage manager,
// which holds one until the bytes are persisted, so a row pointing at a file is
// authoritative no matter how old it is. Only unreferenced files are
// collectable, and only after a grace period, so one created moments before its
// row is committed is never swept out from under it.
//
// Anything else in the directory belongs to somebody else and is left alone.
func (m *Manager) isStale(entry os.DirEntry, path string, now time.Time, referenced map[string]bool) (bool, error) {
	if !entry.IsDir() {
		if !strings.HasPrefix(entry.Name(), assembledPrefix) || referenced[path] {
			return false, nil
		}
		return olderThan(entry, now, archivus_constants.AssembledUploadTTL)
	}
	if _, err := uuid.Parse(entry.Name()); err != nil {
		return false, nil // not a session directory
	}
	sess, err := m.Load(entry.Name())
	if errors.Is(err, ErrSessionNotFound) {
		return olderThan(entry, now, archivus_constants.ChunkSessionTTL)
	}
	if err != nil {
		return false, err
	}
	return now.Sub(sess.CreatedAt) > archivus_constants.ChunkSessionTTL, nil
}

// olderThan reports whether the entry's own timestamp puts it past ttl. An entry
// that has vanished since the directory was listed is not an error: something
// else already cleaned it up, which is the outcome the caller wanted.
func olderThan(entry os.DirEntry, now time.Time, ttl time.Duration) (bool, error) {
	info, err := entry.Info()
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("chunkupload: stat %q: %w", entry.Name(), err)
	}
	return now.Sub(info.ModTime()) > ttl, nil
}

// Discard removes the session's staging directory and all its chunks. It is used
// both to abort an upload and to clean up after a successful assembly. It is a
// no-op if the session no longer exists.
func (m *Manager) Discard(uploadID string) error {
	dir, err := m.sessionDir(uploadID)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("chunkupload: discard session: %w", err)
	}
	return nil
}
