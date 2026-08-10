package chunkupload

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	archivus_constants "archivus/internal/constants"

	"github.com/google/uuid"
)

// now is the fixed clock every case sweeps against, so ages are exact rather
// than "however long the test took".
var now = time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(t.TempDir())
}

// stageSession creates a session and back-dates its manifest, standing in for
// an upload started `age` ago.
func stageSession(t *testing.T, m *Manager, age time.Duration) Session {
	t.Helper()
	sess, err := m.Init(Session{
		UserID: uuid.NewString(), DriveID: uuid.NewString(),
		Filename: "big.bin", Size: 1024, TotalChunks: 2,
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := m.SaveChunk(sess, 0, strings.NewReader("chunk zero")); err != nil {
		t.Fatalf("SaveChunk: %v", err)
	}
	sess.CreatedAt = now.Add(-age)
	dir := filepath.Join(m.stagingRoot, sess.UploadID)
	if err := m.writeManifest(dir, sess); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}
	return sess
}

// stageFile writes a file directly into the staging root with a given age.
func stageFile(t *testing.T, m *Manager, name string, age time.Duration) string {
	t.Helper()
	if err := os.MkdirAll(m.stagingRoot, 0o755); err != nil {
		t.Fatalf("mkdir staging root: %v", err)
	}
	path := filepath.Join(m.stagingRoot, name)
	if err := os.WriteFile(path, []byte("assembled bytes"), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	stamp := now.Add(-age)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatalf("chtimes %s: %v", name, err)
	}
	return path
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sweep(t *testing.T, m *Manager, referenced map[string]bool) StaleSweep {
	t.Helper()
	swept, err := m.PurgeStale(now, referenced)
	if err != nil {
		t.Fatalf("PurgeStale: %v", err)
	}
	return swept
}

// TestPurgeStaleRemovesOnlyExpiredSessions is the core of the GC: a session is
// judged by when the client started it, so an upload still inside its window
// survives and one past it does not.
func TestPurgeStaleRemovesOnlyExpiredSessions(t *testing.T) {
	m := newTestManager(t)
	fresh := stageSession(t, m, time.Hour)
	justInside := stageSession(t, m, archivus_constants.ChunkSessionTTL-time.Hour)
	expired := stageSession(t, m, archivus_constants.ChunkSessionTTL+time.Hour)

	swept := sweep(t, m, nil)
	if swept.Sessions != 1 {
		t.Errorf("swept %d sessions, want 1", swept.Sessions)
	}

	for _, keep := range []Session{fresh, justInside} {
		if _, err := m.Load(keep.UploadID); err != nil {
			t.Errorf("session %s (still within its window) was removed: %v", keep.UploadID, err)
		}
	}
	if _, err := m.Load(expired.UploadID); err == nil {
		t.Error("expired session survived the sweep")
	}
}

// TestPurgeStaleHandlesSessionsWithoutManifest covers a crash between creating
// the session directory and writing its manifest: there is no CreatedAt to
// judge, so the directory's own timestamp has to stand in.
func TestPurgeStaleHandlesSessionsWithoutManifest(t *testing.T) {
	m := newTestManager(t)

	mkBare := func(age time.Duration) string {
		dir := filepath.Join(m.stagingRoot, uuid.NewString())
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		stamp := now.Add(-age)
		if err := os.Chtimes(dir, stamp, stamp); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
		return dir
	}
	recent := mkBare(time.Hour)
	old := mkBare(archivus_constants.ChunkSessionTTL + time.Hour)

	sweep(t, m, nil)

	if !exists(recent) {
		t.Error("a manifest-less directory younger than the TTL was removed")
	}
	if exists(old) {
		t.Error("a manifest-less directory past the TTL survived")
	}
}

// TestPurgeStaleKeepsReferencedAssembledFiles is the property that keeps the GC
// from eating live work: an assembled file belongs to whichever row points at
// it, no matter how long the upload to storage has been dragging on.
func TestPurgeStaleKeepsReferencedAssembledFiles(t *testing.T) {
	m := newTestManager(t)
	veryOld := archivus_constants.AssembledUploadTTL * 10
	claimed := stageFile(t, m, assembledPrefix+"claimed.bin", veryOld)
	orphan := stageFile(t, m, assembledPrefix+"orphan.bin", veryOld)
	fresh := stageFile(t, m, assembledPrefix+"fresh.bin", time.Minute)

	swept := sweep(t, m, map[string]bool{claimed: true})
	if swept.Assembled != 1 {
		t.Errorf("swept %d assembled files, want 1", swept.Assembled)
	}

	if !exists(claimed) {
		t.Error("an assembled file a row still points at was deleted")
	}
	if !exists(fresh) {
		t.Error("an assembled file inside its grace period was deleted")
	}
	if exists(orphan) {
		t.Error("an unreferenced, long-expired assembled file survived")
	}
}

// TestPurgeStaleLeavesForeignEntriesAlone: the staging root is a directory on a
// real filesystem, and the GC must not treat anything it did not create as
// garbage.
func TestPurgeStaleLeavesForeignEntriesAlone(t *testing.T) {
	m := newTestManager(t)
	veryOld := archivus_constants.ChunkSessionTTL * 10
	strayFile := stageFile(t, m, "notes.txt", veryOld)

	strayDir := filepath.Join(m.stagingRoot, "not-a-uuid")
	if err := os.MkdirAll(strayDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	stamp := now.Add(-veryOld)
	if err := os.Chtimes(strayDir, stamp, stamp); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	swept := sweep(t, m, nil)
	if !swept.Empty() {
		t.Errorf("sweep reclaimed %+v, want nothing", swept)
	}
	if !exists(strayFile) {
		t.Error("an unrelated file in the staging root was deleted")
	}
	if !exists(strayDir) {
		t.Error("a non-session directory in the staging root was deleted")
	}
}

// TestPurgeStaleOnMissingStagingRoot: a server that has never taken a chunked
// upload has no staging directory, which is not an error.
func TestPurgeStaleOnMissingStagingRoot(t *testing.T) {
	m := newTestManager(t)
	swept, err := m.PurgeStale(now, nil)
	if err != nil {
		t.Fatalf("PurgeStale on missing staging root: %v", err)
	}
	if !swept.Empty() {
		t.Errorf("swept %+v from a directory that does not exist", swept)
	}
}

// TestPurgeStaleSpareseLiveSession guards the interaction that matters most in
// practice: a sweep running while an upload is in progress must not disturb it.
func TestPurgeStaleSparesLiveSession(t *testing.T) {
	m := newTestManager(t)
	sess, err := m.Init(Session{
		UserID: uuid.NewString(), DriveID: uuid.NewString(),
		Filename: "live.bin", Size: 20, TotalChunks: 2,
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := m.SaveChunk(sess, 0, strings.NewReader("first half ")); err != nil {
		t.Fatalf("SaveChunk: %v", err)
	}

	if swept := sweep(t, m, nil); !swept.Empty() {
		t.Fatalf("sweep reclaimed %+v from a live session", swept)
	}

	// The upload carries on and completes as if nothing happened.
	if err := m.SaveChunk(sess, 1, strings.NewReader("second half")); err != nil {
		t.Fatalf("SaveChunk after sweep: %v", err)
	}
	f, err := m.Assemble(sess)
	if err != nil {
		t.Fatalf("Assemble after sweep: %v", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
}
