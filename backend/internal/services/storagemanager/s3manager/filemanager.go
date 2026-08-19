package s3manager

import (
	"archivus/internal/config"
	archivus_constants "archivus/internal/constants"
	"archivus/internal/models"
	storage_types "archivus/internal/services/storagemanager/types"
	"archivus/internal/store"
	"archivus/pkg/logging"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func (s *S3Manager) UploadFile(relPath, driveId, userId string, file multipart.File, fileHeader *multipart.FileHeader) error {
	hasAccess, err := s.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	// key is namespaced by drive slug within the shared bucket
	key := drive.Slug + "/" + filepath.Join(strings.Trim(relPath, "/"), fileHeader.Filename)
	contentType := fileHeader.Header.Get("Content-Type")
	if err := s.Client.PutObject(context.Background(), s.Client.BucketName, key, contentType, fileHeader.Size, file); err != nil {
		return fmt.Errorf("s3manager: upload %q: %w", key, err)
	}
	dbAbsPath := fmt.Sprintf("s3://%s/%s", s.Client.BucketName, key)
	dbDirPath := fmt.Sprintf("s3://%s/%s", s.Client.BucketName, filepath.Dir(key))
	sizeInMb := float64(fileHeader.Size) / (1 << 20)
	_, err = s.Store.CreateFileMetadata(fileHeader.Filename, dbAbsPath, key, dbDirPath, driveId, userId, sizeInMb)
	if err != nil {
		_ = s.Client.DeleteObject(context.Background(), s.Client.BucketName, key)
		return fmt.Errorf("s3manager: save file metadata for %q: %w", key, err)
	}
	return nil
}

// func (s *S3Manager) MoveFile(srcRelPath, dstRelPath, driveId, userId string) error {
// 	hasAccess, err := s.CheckUserDriveWriteAccess(userId, driveId)
// 	if err != nil {
// 		return err
// 	}
// 	if !hasAccess {
// 		return errors.New("user does not have write access to this drive")
// 	}
// 	drive, err := s.Store.GetDriveByID(driveId)
// 	if err != nil {
// 		return fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
// 	}
// 	srcKey := s3Key(srcRelPath, drive.Slug)
// 	dstKey := s3Key(dstRelPath, drive.Slug)
// 	md, err := s.Store.GetFileMetadataByRelPath(filepath.Join(drive.Slug, srcRelPath))
// 	if err != nil {
// 		return fmt.Errorf("s3manager: get metadata for %q: %w", srcRelPath, err)
// 	}
// 	ctx := context.Background()
// 	if err := s.Client.CopyObject(ctx, drive.Slug, srcKey, dstKey); err != nil {
// 		return fmt.Errorf("s3manager: copy %q to %q: %w", srcKey, dstKey, err)
// 	}
// 	if err := s.Client.DeleteObject(ctx, drive.Slug, srcKey); err != nil {
// 		_ = s.Client.DeleteObject(ctx, drive.Slug, dstKey)
// 		return fmt.Errorf("s3manager: delete source %q after move: %w", srcKey, err)
// 	}
// 	newRelPath := filepath.Join(drive.Slug, dstRelPath)
// 	newAbsPath := fmt.Sprintf("s3://%s/%s", drive.Slug, dstKey)
// 	newDirPath := fmt.Sprintf("s3://%s/%s", drive.Slug, filepath.Dir(dstKey))
// 	return s.Store.UpdateFileMetadataPaths(md.ID, newAbsPath, newRelPath, newDirPath)
// }

func (s *S3Manager) DownloadFile(fileId string, driveId, userId string) (*os.File, *models.FileMetadata, error) {
	hasAccess, err := s.CheckUserHasDriveAccess(userId, driveId)
	if err != nil {
		return nil, nil, err
	}
	if !hasAccess {
		return nil, nil, errors.New("user does not have access to this drive")
	}
	md, err := s.Store.GetFileMetadataByID(fileId)
	if err != nil {
		return nil, nil, fmt.Errorf("s3manager: get file metadata %q: %w", fileId, err)
	}
	if md.UploadStatus == models.UploadStatusPending || md.UploadStatus == models.UploadStatusUploading {
		return nil, nil, fmt.Errorf("s3manager: file %q is still being uploaded", md.Name)
	}
	if md.UploadStatus == models.UploadStatusFailed {
		return nil, nil, fmt.Errorf("s3manager: upload of file %q did not complete", md.Name)
	}
	// PathKey = drive.Slug/dir/filename, i.e. the full key in the shared bucket
	out, err := s.Client.GetObject(context.Background(), s.Client.BucketName, md.PathKey)
	if err != nil {
		return nil, nil, fmt.Errorf("s3manager: get object %q: %w", md.PathKey, err)
	}
	defer out.Body.Close()

	tmp, err := os.CreateTemp("", "archivus-download-*")
	if err != nil {
		return nil, nil, fmt.Errorf("s3manager: create temp file: %w", err)
	}
	if _, err := io.Copy(tmp, out.Body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, nil, fmt.Errorf("s3manager: write to temp file: %w", err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, nil, fmt.Errorf("s3manager: seek temp file: %w", err)
	}
	return tmp, &md, nil
}

// UploadFileV2 stores a file in a drive. Biz mode keeps every version: if a file
// already exists at the same key, its current bytes and metadata are first
// archived under a timestamped key, then the new content is written to the
// canonical key (so the canonical key is always the latest). This suits
// multi-contributor setups that must never lose a previous copy.
func (s *S3Manager) UploadFileV2(relPath, driveId, userId string, file multipart.File, fileHeader *multipart.FileHeader) error {
	hasAccess, err := s.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	trimmed := strings.Trim(relPath, "/")
	var pathKey string
	if trimmed == "" {
		pathKey = drive.Slug + "/" + fileHeader.Filename
	} else {
		pathKey = drive.Slug + "/" + trimmed + "/" + fileHeader.Filename
	}
	prefix := filepath.Dir(pathKey) + "/"
	contentType := fileHeader.Header.Get("Content-Type")
	sizeInMb := float64(fileHeader.Size) / (1 << 20)
	ctx := context.Background()

	existing, lookupErr := s.Store.GetFileMetadataByDrivePathKey(driveId, pathKey)
	if lookupErr != nil && !errors.Is(lookupErr, store.ErrRecordNotFound) {
		return fmt.Errorf("s3manager: lookup existing file %q: %w", pathKey, lookupErr)
	}

	if lookupErr == nil {
		// Archive the current version (object + metadata) before overwriting.
		archiveKey := versionedKey(pathKey)
		if err := s.Client.CopyObject(ctx, s.Client.BucketName, pathKey, archiveKey); err != nil {
			return fmt.Errorf("s3manager: archive previous version of %q: %w", pathKey, err)
		}
		archiveName := filepath.Base(archiveKey)
		_, err = s.Store.CreateFileMetadataV2(archiveName, archiveKey, prefix, existing.ContentType, driveId, existing.UploadedByID.String(), existing.SizeInMb)
		if err != nil {
			_ = s.Client.DeleteObject(ctx, s.Client.BucketName, archiveKey)
			return fmt.Errorf("s3manager: save archived version metadata for %q: %w", archiveKey, err)
		}
		if err := s.Client.PutObjectMultipart(ctx, s.Client.BucketName, pathKey, contentType, file); err != nil {
			return fmt.Errorf("s3manager: upload %q: %w", pathKey, err)
		}
		if err := s.Store.UpdateFileMetadataContent(existing.ID.String(), sizeInMb, contentType); err != nil {
			return fmt.Errorf("s3manager: update current version metadata for %q: %w", pathKey, err)
		}
		return nil
	}

	// First upload of this key.
	if err := s.Client.PutObjectMultipart(ctx, s.Client.BucketName, pathKey, contentType, file); err != nil {
		return fmt.Errorf("s3manager: upload %q: %w", pathKey, err)
	}
	_, err = s.Store.CreateFileMetadataV2(fileHeader.Filename, pathKey, prefix, contentType, driveId, userId, sizeInMb)
	if err != nil {
		_ = s.Client.DeleteObject(ctx, s.Client.BucketName, pathKey)
		return fmt.Errorf("s3manager: save file metadata for %q: %w", pathKey, err)
	}
	return nil
}

// EnqueueChunkedUpload records an assembled chunked upload as a pending file and
// returns immediately, deferring the slow multipart push to R2 to a background
// worker (ProcessPendingUploads). This keeps the client's /complete request fast
// even for multi-hundred-MB files: assembly is local and quick, while the
// bandwidth-bound upload happens off the request path. Write access is validated
// up front so an unauthorized caller still fails fast.
func (s *S3Manager) EnqueueChunkedUpload(relPath, driveId, userId, contentType string, size int64, filename, localPath string) (models.FileMetadata, error) {
	hasAccess, err := s.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return models.FileMetadata{}, err
	}
	if !hasAccess {
		return models.FileMetadata{}, errors.New("user does not have write access to this drive")
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return models.FileMetadata{}, fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	trimmed := strings.Trim(relPath, "/")
	var pathKey string
	if trimmed == "" {
		pathKey = drive.Slug + "/" + filename
	} else {
		pathKey = drive.Slug + "/" + trimmed + "/" + filename
	}
	prefix := filepath.Dir(pathKey) + "/"
	sizeInMb := float64(size) / (1 << 20)

	existing, lookupErr := s.Store.GetFileMetadataByDrivePathKey(driveId, pathKey)
	if lookupErr != nil && !errors.Is(lookupErr, store.ErrRecordNotFound) {
		return models.FileMetadata{}, fmt.Errorf("s3manager: lookup existing file %q: %w", pathKey, lookupErr)
	}
	if lookupErr == nil {
		// Overwrite: reuse the existing row so we never have two rows for one key.
		// Its current bytes stay live in R2 until the worker archives them and
		// swaps in the new content, so the previous version remains downloadable.
		if err := s.Store.MarkFileMetadataPending(existing.ID.String(), localPath); err != nil {
			return models.FileMetadata{}, fmt.Errorf("s3manager: mark %q pending: %w", pathKey, err)
		}
		existing.UploadStatus = models.UploadStatusPending
		existing.PendingSourcePath = localPath
		return existing, nil
	}
	fm, err := s.Store.CreatePendingFileMetadataV2(filename, pathKey, prefix, contentType, driveId, userId, sizeInMb, localPath)
	if err != nil {
		return models.FileMetadata{}, fmt.Errorf("s3manager: create pending metadata for %q: %w", pathKey, err)
	}
	return fm, nil
}

// pendingDrainRunning keeps one process to a single drain at a time. Every
// /complete kicks off a drain and the cron fires one every 30 seconds, so
// without this the worker pools would stack and the real concurrency limit
// would be however many callers happened to arrive at once.
//
// It is package-level rather than a field because callers build a fresh
// S3Manager per invocation (see cmd/celery), which would make an instance flag
// guard nothing. Correctness across processes — the API and the cron worker are
// separate binaries — rests on ClaimPendingUpload being atomic, not on this;
// all this bounds is how much work one process takes on.
var pendingDrainRunning atomic.Bool

// ProcessPendingUploads drains the pending-upload queue: it claims rows (so no
// other worker touches them), pushes the staged bytes to R2 through a pool of
// concurrent workers, and marks each one ready. Transient failures go back to
// the queue for a later pass, up to models.MaxUploadAttempts, after which the
// row is marked failed.
//
// It keeps taking passes until one turns up nothing new, because stopping after
// a single batch would cap throughput at PendingUploadBatchSize per cron tick no
// matter how many workers were idle.
func (s *S3Manager) ProcessPendingUploads(ctx context.Context) error {
	if !pendingDrainRunning.CompareAndSwap(false, true) {
		// A drain is already going and re-queries as it works, so it will pick
		// up whatever this call was going to handle.
		return nil
	}
	defer pendingDrainRunning.Store(false)

	attempted := make(map[string]bool)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		batch, err := s.nextPendingBatch(attempted)
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		s.uploadBatch(ctx, batch)
	}
}

// nextPendingBatch returns the next candidate rows for this drain to work on.
//
// Rows already tried in this drain are skipped: a transient failure puts a row
// straight back to pending, and picking it up again on the very next pass would
// spend its whole retry budget inside one drain instead of across the retries
// the schedule is meant to spread out. Since a candidate is never handed to more
// than one worker, this is also what makes the loop terminate.
func (s *S3Manager) nextPendingBatch(attempted map[string]bool) ([]models.FileMetadata, error) {
	pending, err := s.Store.GetPendingFileUploads(archivus_constants.PendingUploadBatchSize)
	if err != nil {
		return nil, fmt.Errorf("s3manager: list pending uploads: %w", err)
	}
	var batch []models.FileMetadata
	for _, fm := range pending {
		id := fm.ID.String()
		if attempted[id] {
			continue
		}
		attempted[id] = true
		batch = append(batch, fm)
	}
	return batch, nil
}

// uploadBatch pushes a batch of candidate rows through the worker pool and
// returns once they are all resolved. Each row is independent and records its
// own outcome, so one failure never holds up or cancels the rest.
//
// Each row is claimed by the worker that is about to upload it, not up front by
// the dispatcher. That keeps "uploading" meaning a file somebody is actually
// pushing — which is what makes the status readable while a drain runs — and
// caps what a hard crash can strand at the number of workers rather than a whole
// batch.
func (s *S3Manager) uploadBatch(ctx context.Context, batch []models.FileMetadata) {
	// Buffered to the full batch so dispatch never blocks on a worker.
	jobs := make(chan models.FileMetadata, len(batch))
	for _, fm := range batch {
		jobs <- fm
	}
	close(jobs)

	var wg sync.WaitGroup
	for range min(archivus_constants.PendingUploadWorkers, len(batch)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fm := range jobs {
				if ctx.Err() != nil {
					// Shutting down. Nothing is claimed yet, so the remaining
					// rows are simply left pending for the next run.
					return
				}
				won, err := s.Store.ClaimPendingUpload(fm.ID.String())
				if err != nil {
					logging.CronErrorLogger.Error().Err(err).Str("pathKey", fm.PathKey).
						Msg("cron: claim pending upload failed")
					continue
				}
				if !won {
					continue // another process got it first
				}
				s.finalizeClaimed(ctx, fm)
			}
		}()
	}
	wg.Wait()
}

// finalizeClaimed pushes one claimed row and records how it went: ready on
// success, back to pending while retries remain, failed once they run out.
func (s *S3Manager) finalizeClaimed(ctx context.Context, fm models.FileMetadata) {
	err := s.finalizePendingUpload(ctx, fm)
	if err == nil {
		return
	}
	// fm.UploadAttempts reflects the count before this claim incremented it.
	attempt := fm.UploadAttempts + 1
	if attempt >= models.MaxUploadAttempts {
		_ = s.Store.MarkFileUploadFailed(fm.ID.String())
	} else {
		_ = s.Store.RevertUploadToPending(fm.ID.String())
	}
	logging.CronErrorLogger.Error().Err(err).Str("pathKey", fm.PathKey).Int("attempt", attempt).
		Msg("cron: finalize pending upload failed")
}

// PendingBacklogFull reports whether the staging area is holding as much
// not-yet-pushed data as it is allowed to. Upload paths check it before
// assembling another file so that clients are told to slow down rather than
// filling the disk faster than the workers drain it.
func (s *S3Manager) PendingBacklogFull() (bool, error) {
	backlogMB, err := s.Store.PendingUploadBacklogMB()
	if err != nil {
		return false, fmt.Errorf("s3manager: measure pending upload backlog: %w", err)
	}
	return backlogMB >= archivus_constants.MaxPendingUploadBacklogMB, nil
}

// finalizePendingUpload pushes one staged file to R2. If an object already
// exists at the key (an overwrite) its current bytes and metadata are archived
// under a versioned key first, mirroring the synchronous UploadFileV2 path.
func (s *S3Manager) finalizePendingUpload(ctx context.Context, fm models.FileMetadata) error {
	f, err := os.Open(fm.PendingSourcePath)
	if err != nil {
		return fmt.Errorf("open staged file %q: %w", fm.PendingSourcePath, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat staged file %q: %w", fm.PendingSourcePath, err)
	}
	sizeInMb := float64(info.Size()) / (1 << 20)

	// Archive an existing object before overwriting it, so no version is lost.
	if _, headErr := s.Client.HeadObject(ctx, s.Client.BucketName, fm.PathKey); headErr == nil {
		archiveKey := versionedKey(fm.PathKey)
		if err := s.Client.CopyObject(ctx, s.Client.BucketName, fm.PathKey, archiveKey); err != nil {
			return fmt.Errorf("archive previous version of %q: %w", fm.PathKey, err)
		}
		archiveName := filepath.Base(archiveKey)
		if _, err := s.Store.CreateFileMetadataV2(archiveName, archiveKey, fm.Prefix, fm.ContentType, fm.DriveID.String(), fm.UploadedByID.String(), fm.SizeInMb); err != nil {
			_ = s.Client.DeleteObject(ctx, s.Client.BucketName, archiveKey)
			return fmt.Errorf("save archived version metadata for %q: %w", archiveKey, err)
		}
	}

	if err := s.Client.PutObjectMultipart(ctx, s.Client.BucketName, fm.PathKey, fm.ContentType, f); err != nil {
		return fmt.Errorf("upload %q: %w", fm.PathKey, err)
	}
	if err := s.Store.MarkFileUploadReady(fm.ID.String(), sizeInMb); err != nil {
		return fmt.Errorf("mark %q ready: %w", fm.PathKey, err)
	}
	if err := os.Remove(fm.PendingSourcePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Printf("warning: failed to remove staged file %q: %v\n", fm.PendingSourcePath, err)
	}
	return nil
}

// versionedKey derives an archival key for the current contents of key by
// inserting a nanosecond timestamp before the extension, e.g.
// "drive/dir/report.pdf" -> "drive/dir/report.v1721512345678901234.pdf".
func versionedKey(key string) string {
	ext := filepath.Ext(key)
	base := strings.TrimSuffix(key, ext)
	return fmt.Sprintf("%s.v%d%s", base, time.Now().UnixNano(), ext)
}

func (s *S3Manager) GetFilesV2(relPath, driveId, userId string, page, pageSize int, opts storage_types.ListOptions) (storage_types.PagedDirEntries, error) {
	var out storage_types.PagedDirEntries
	hasAccess, err := s.CheckUserHasDriveAccess(userId, driveId)
	if err != nil {
		return out, err
	}
	if !hasAccess {
		return out, errors.New("user does not have access to this drive")
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return out, fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	trimmed := strings.Trim(relPath, "/")
	var dirPrefixes [2]string
	if trimmed == "" {
		dirPrefixes = [2]string{drive.Slug + "/", drive.Slug} // Include the root directory prefix
	} else {
		dirPrefixes = [2]string{drive.Slug + "/" + trimmed + "/", drive.Slug + "/" + trimmed}
	}
	ctx := context.Background()

	limit, offset := storage_types.PageBounds(page, pageSize)
	dirCount, err := s.Store.CountDirectoriesByParentPrefix(drive.ID.String(), dirPrefixes)
	if err != nil {
		return out, fmt.Errorf("s3manager: count dirs for prefix %q: %w", dirPrefixes, err)
	}
	fileCount, err := s.Store.CountFileMetadataByDirPrefix(drive.ID.String(), dirPrefixes, opts.ContentType)
	if err != nil {
		return out, fmt.Errorf("s3manager: count files for prefix %q: %w", dirPrefixes, err)
	}
	window := storage_types.PageWindow(int(dirCount), limit, offset)

	entries := make([]storage_types.DirEntry, 0, limit)
	if window.DirLimit != 0 {
		dirs, err := s.Store.GetDirectoriesByParentPrefixPaged(drive.ID.String(), dirPrefixes, window.DirLimit, window.DirOffset, opts.SortBy, opts.SortOrder)
		if err != nil {
			return out, fmt.Errorf("s3manager: list dirs for prefix %q: %w", dirPrefixes, err)
		}
		for _, d := range dirs {
			entries = append(entries, storage_types.DirEntry{
				ID:             d.ID.String(),
				Name:           d.Name,
				IsDir:          true,
				Path:           d.PathKey,
				CreatedAt:      d.CreatedAt,
				NavigationPath: filepath.Join(relPath, d.Name),
			})
		}
	}
	if window.FileLimit != 0 {
		files, err := s.Store.GetFileMetadataByDirPrefixPaged(drive.ID.String(), dirPrefixes, window.FileLimit, window.FileOffset, opts.SortBy, opts.SortOrder, opts.ContentType)
		if err != nil {
			return out, fmt.Errorf("s3manager: list files for prefix %q: %w", dirPrefixes, err)
		}
		for _, f := range files {
			signedURL, _ := s.Client.PresignGetObject(ctx, s.Client.BucketName, f.PathKey, 15*time.Minute)
			entries = append(entries, storage_types.DirEntry{
				ID:             f.ID.String(),
				Name:           f.Name,
				IsDir:          false,
				Extension:      filepath.Ext(f.Name),
				SignedUrl:      signedURL,
				Size:           f.SizeInMb,
				Path:           f.PathKey,
				Thumbnail:      storage_types.ThumbnailURL(f.ThumbnailPath, config.Config.ThumbnailDir),
				CreatedAt:      f.CreatedAt,
				ContentType:    f.ContentType,
				UploadStatus:   f.UploadStatus,
				NavigationPath: filepath.Join(relPath, f.Name),
			})
		}
	}

	out.Entries = entries
	out.Total = dirCount + fileCount
	out.PageSize = limit
	out.Page = offset/limit + 1
	return out, nil
}

func (s *S3Manager) GetFiles(relPath, driveId, userId string) ([]storage_types.DirEntry, error) {
	hasAccess, err := s.CheckUserHasDriveAccess(userId, driveId)
	if err != nil {
		return nil, err
	}
	if !hasAccess {
		return nil, errors.New("user does not have access to this drive")
	}
	drive, err := s.Store.GetDriveByID(driveId)
	if err != nil {
		return nil, fmt.Errorf("s3manager: get drive %q: %w", driveId, err)
	}
	trimmed := strings.Trim(relPath, "/")
	var prefix string
	if trimmed == "" {
		prefix = drive.Slug + "/"
	} else {
		prefix = drive.Slug + "/" + trimmed + "/"
	}
	ctx := context.Background()
	entries, err := s.Client.ListObjectsOnelevel(ctx, s.Client.BucketName, prefix)
	if err != nil {
		return nil, fmt.Errorf("s3manager: list %q: %w", prefix, err)
	}
	var dirEntries []storage_types.DirEntry
	for _, e := range entries {
		name := filepath.Base(strings.TrimSuffix(e.Key, "/"))
		signedURL := ""
		if !e.IsDir {
			signedURL, _ = s.Client.PresignGetObject(ctx, s.Client.BucketName, e.Key, 15*time.Minute)
		}
		dirEntries = append(dirEntries, storage_types.DirEntry{
			ID:             "",
			Name:           name,
			IsDir:          e.IsDir,
			Extension:      filepath.Ext(name),
			SignedUrl:      signedURL,
			Path:           e.Key, // full S3 key = DB PathKey
			NavigationPath: filepath.Join(relPath, name),
		})
	}
	return dirEntries, nil
}
