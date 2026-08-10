package s3manager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	archivus_constants "archivus/internal/constants"
	"archivus/internal/models"
	"archivus/internal/services/storagemanager/base"
	"archivus/internal/store"

	"github.com/google/uuid"
)

// newTestManager builds an S3Manager over a temp SQLite store. Client is left
// nil on purpose: everything exercised here is queue bookkeeping, which never
// reaches out to object storage.
func newTestManager(t *testing.T) *S3Manager {
	t.Helper()
	s, err := store.GetStore(t.TempDir())
	if err != nil {
		t.Fatalf("GetStore: %v", err)
	}
	return &S3Manager{BaseManager: base.BaseManager{Store: s}}
}

// addPending queues a row backed by a real staged file, the way a completed
// chunked upload leaves things. The file has to actually exist: a finalize that
// cannot open its source fails early and reverts the row, which would mask
// whether a code path reached the upload at all.
func addPending(t *testing.T, m *S3Manager, name string, sizeMB float64) models.FileMetadata {
	t.Helper()
	staged := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(staged, []byte("staged bytes"), 0o644); err != nil {
		t.Fatalf("write staged file: %v", err)
	}
	fm, err := m.Store.CreatePendingFileMetadataV2(
		name, "drive/"+name, "drive/", "application/octet-stream",
		uuid.NewString(), uuid.NewString(), sizeMB, staged,
	)
	if err != nil {
		t.Fatalf("CreatePendingFileMetadataV2(%s): %v", name, err)
	}
	return fm
}

func statusOf(t *testing.T, m *S3Manager, id string) string {
	t.Helper()
	fm, err := m.Store.GetFileMetadataByID(id)
	if err != nil {
		t.Fatalf("GetFileMetadataByID(%s): %v", id, err)
	}
	return fm.UploadStatus
}

// TestNextPendingBatchSkipsRowsAlreadyTried covers the rule that keeps a drain
// from eating a row's whole retry budget: a failure puts the row back to
// pending, and the very next pass of the same drain must leave it alone. It is
// also what terminates the drain loop.
func TestNextPendingBatchSkipsRowsAlreadyTried(t *testing.T) {
	m := newTestManager(t)
	a := addPending(t, m, "a.bin", 10)
	addPending(t, m, "b.bin", 20)

	attempted := map[string]bool{}
	batch, err := m.nextPendingBatch(attempted)
	if err != nil {
		t.Fatalf("nextPendingBatch: %v", err)
	}
	if len(batch) != 2 {
		t.Fatalf("batch has %d rows, want 2", len(batch))
	}
	// Handing a row to a worker must not itself move it out of pending: the
	// worker claims it, and until then the row still counts as queued.
	for _, fm := range batch {
		if got := statusOf(t, m, fm.ID.String()); got != models.UploadStatusPending {
			t.Errorf("row %s status = %q before any worker claimed it, want pending", fm.Name, got)
		}
	}

	if again, err := m.nextPendingBatch(attempted); err != nil || len(again) != 0 {
		t.Fatalf("second pass returned %d rows (err %v), want 0", len(again), err)
	}

	// A row handed back after a transient failure stays untouched for the rest
	// of this drain...
	if err := m.Store.RevertUploadToPending(a.ID.String()); err != nil {
		t.Fatalf("RevertUploadToPending: %v", err)
	}
	if third, err := m.nextPendingBatch(attempted); err != nil || len(third) != 0 {
		t.Fatalf("pass after revert returned %d rows (err %v), want 0", len(third), err)
	}

	// ...but a later drain picks it up again.
	fresh, err := m.nextPendingBatch(map[string]bool{})
	if err != nil {
		t.Fatalf("fresh drain: %v", err)
	}
	if len(fresh) != 2 {
		t.Fatalf("fresh drain saw %d rows, want both back in the queue", len(fresh))
	}
}

// TestUploadBatchClaimsNothingWhenCancelled is the shutdown path: a drain that
// is already cancelled must leave every row pending rather than claiming rows it
// will not upload and stranding them in "uploading" with no owner.
//
// It doubles as the assertion that a cancelled drain starts no upload at all —
// Client is nil, so any attempt to reach object storage crashes the run.
func TestUploadBatchClaimsNothingWhenCancelled(t *testing.T) {
	m := newTestManager(t)
	addPending(t, m, "a.bin", 1)
	addPending(t, m, "b.bin", 1)

	batch, err := m.nextPendingBatch(map[string]bool{})
	if err != nil {
		t.Fatalf("nextPendingBatch: %v", err)
	}
	if len(batch) != 2 {
		t.Fatalf("batch has %d rows, want 2", len(batch))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.uploadBatch(ctx, batch)

	for _, fm := range batch {
		if got := statusOf(t, m, fm.ID.String()); got != models.UploadStatusPending {
			t.Errorf("row %s status = %q after cancellation, want pending", fm.Name, got)
		}
	}
}

// TestUploadBatchClaimsOnlyWhatItWorksOn is the observability property the batch
// shape has to preserve: while a drain runs, "uploading" means a file a worker
// is actually pushing. Claiming a whole batch up front would light up every row
// at once and, on a hard crash, strand all of them instead of a handful.
func TestUploadBatchClaimsOnlyWhatItWorksOn(t *testing.T) {
	m := newTestManager(t)
	const queued = archivus_constants.PendingUploadWorkers * 3
	for i := range queued {
		addPending(t, m, fmt.Sprintf("f%02d.bin", i), 1)
	}

	batch, err := m.nextPendingBatch(map[string]bool{})
	if err != nil {
		t.Fatalf("nextPendingBatch: %v", err)
	}
	if len(batch) != queued {
		t.Fatalf("batch has %d rows, want %d", len(batch), queued)
	}

	// Nothing is claimed before a worker picks the row up, so the whole batch is
	// still pending at dispatch time.
	uploading := 0
	for _, fm := range batch {
		if statusOf(t, m, fm.ID.String()) == models.UploadStatusUploading {
			uploading++
		}
	}
	if uploading != 0 {
		t.Errorf("%d rows already marked uploading at dispatch, want 0", uploading)
	}
}

// TestPendingBacklogFull covers the signal the upload handlers use to push back
// on clients when the drain is falling behind.
func TestPendingBacklogFull(t *testing.T) {
	m := newTestManager(t)

	// An empty queue sums to NULL, which must read as no backlog rather than error.
	full, err := m.PendingBacklogFull()
	if err != nil {
		t.Fatalf("PendingBacklogFull on empty queue: %v", err)
	}
	if full {
		t.Fatal("empty queue reported as full")
	}

	addPending(t, m, "big.bin", archivus_constants.MaxPendingUploadBacklogMB-1)
	if full, err := m.PendingBacklogFull(); err != nil || full {
		t.Fatalf("just under the limit = %v (err %v), want not full", full, err)
	}

	tip := addPending(t, m, "tip.bin", 1)
	if full, err := m.PendingBacklogFull(); err != nil || !full {
		t.Fatalf("at the limit = %v (err %v), want full", full, err)
	}

	// Rows being uploaded still hold their staged bytes on disk, so they have to
	// keep counting against the budget.
	batch, err := m.nextPendingBatch(map[string]bool{})
	if err != nil {
		t.Fatalf("nextPendingBatch: %v", err)
	}
	for _, fm := range batch {
		if _, err := m.Store.ClaimPendingUpload(fm.ID.String()); err != nil {
			t.Fatalf("ClaimPendingUpload: %v", err)
		}
	}
	if full, err := m.PendingBacklogFull(); err != nil || !full {
		t.Fatalf("with rows in flight = %v (err %v), want still full", full, err)
	}

	// Finishing one drops the backlog back under the limit.
	if err := m.Store.MarkFileUploadReady(tip.ID.String(), 1); err != nil {
		t.Fatalf("MarkFileUploadReady: %v", err)
	}
	if full, err := m.PendingBacklogFull(); err != nil || full {
		t.Fatalf("after a completion = %v (err %v), want not full", full, err)
	}
}
