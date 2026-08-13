package chunkupload

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// TestHeadroomBytesReportsFreeSpace guards the monitored precondition: a Manager
// on a usable filesystem must be able to report how much free space backs its
// staging root, so operators can watch it.
func TestHeadroomBytesReportsFreeSpace(t *testing.T) {
	m := newTestManager(t)
	free, err := m.HeadroomBytes()
	if err != nil {
		t.Fatalf("HeadroomBytes: %v", err)
	}
	if free == 0 {
		t.Error("a staging root on a real filesystem reported zero free space")
	}
}

// TestAssembleRefusesWhenAssemblyWouldOverrunDisk verifies the size-aware
// headroom guard in Assemble: a session whose declared Size is far larger than
// anything the filesystem could hold must be refused with ErrInsufficientDiskSpace
// before a truncated assembled file is produced. The chunks are all present—this
// is purely the free-space precondition rejecting the write.
func TestAssembleRefusesWhenAssemblyWouldOverrunDisk(t *testing.T) {
	m := newTestManager(t)
	sess, err := m.Init(Session{
		UserID: uuid.NewString(), DriveID: uuid.NewString(),
		Filename: "huge.bin", Size: 1 << 40, // 1 TiB — far beyond any free space
		TotalChunks: 2,
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := m.SaveChunk(sess, 0, strings.NewReader("chunk zero")); err != nil {
		t.Fatalf("SaveChunk(0): %v", err)
	}
	if err := m.SaveChunk(sess, 1, strings.NewReader("chunk one")); err != nil {
		t.Fatalf("SaveChunk(1): %v", err)
	}

	// Rewrite the manifest with the impossible size (Init checked headroom for a
	// *new* session against the static floor, not against a full-size assembly).
	sess.Size = 1 << 40
	dir := filepath.Join(m.stagingRoot, sess.UploadID)
	if err := m.writeManifest(dir, sess); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}

	if _, err := m.Assemble(sess); !errors.Is(err, ErrInsufficientDiskSpace) {
		t.Fatalf("Assemble error = %v, want ErrInsufficientDiskSpace", err)
	}

	// The refusal must not have left a partial assembled file behind.
	entries, err := filepath.Glob(filepath.Join(m.stagingRoot, assembledPrefix+"*"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a refused assembly left %d assembled file(s) on disk: %v", len(entries), entries)
	}
}
