package s3manager

import (
	"archivus/internal/config"
	"archivus/internal/models"
	storage_types "archivus/internal/services/storagemanager/types"
	"archivus/internal/store"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
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

// ProcessPendingUploads drains the pending-upload queue: for each file it claims
// the row (so no other worker touches it), pushes the staged bytes to R2 via a
// concurrent multipart upload, and marks it ready. Transient failures are
// retried on a later pass up to models.MaxUploadAttempts, after which the row is
// marked failed.
func (s *S3Manager) ProcessPendingUploads(ctx context.Context) error {
	pending, err := s.Store.GetPendingFileUploads(20)
	if err != nil {
		return fmt.Errorf("s3manager: list pending uploads: %w", err)
	}
	for _, fm := range pending {
		claimed, err := s.Store.ClaimPendingUpload(fm.ID.String())
		if err != nil {
			return fmt.Errorf("s3manager: claim pending upload %q: %w", fm.ID, err)
		}
		if !claimed {
			continue // another worker got it first
		}
		if err := s.finalizePendingUpload(ctx, fm); err != nil {
			// fm.UploadAttempts reflects the count before this claim incremented it.
			if fm.UploadAttempts+1 >= models.MaxUploadAttempts {
				_ = s.Store.MarkFileUploadFailed(fm.ID.String())
			} else {
				_ = s.Store.RevertUploadToPending(fm.ID.String())
			}
			fmt.Printf("s3manager: finalize pending upload %q failed (attempt %d): %v\n", fm.PathKey, fm.UploadAttempts+1, err)
		}
	}
	return nil
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

func (s *S3Manager) GetFilesV2(relPath, driveId, userId string, page, pageSize int) (storage_types.PagedDirEntries, error) {
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
	fileCount, err := s.Store.CountFileMetadataByDirPrefix(drive.ID.String(), dirPrefixes)
	if err != nil {
		return out, fmt.Errorf("s3manager: count files for prefix %q: %w", dirPrefixes, err)
	}
	window := storage_types.PageWindow(int(dirCount), limit, offset)

	entries := make([]storage_types.DirEntry, 0, limit)
	if window.DirLimit != 0 {
		dirs, err := s.Store.GetDirectoriesByParentPrefixPaged(drive.ID.String(), dirPrefixes, window.DirLimit, window.DirOffset)
		if err != nil {
			return out, fmt.Errorf("s3manager: list dirs for prefix %q: %w", dirPrefixes, err)
		}
		for _, d := range dirs {
			entries = append(entries, storage_types.DirEntry{
				ID:             d.ID.String(),
				Name:           d.Name,
				IsDir:          true,
				Path:           d.PathKey,
				NavigationPath: filepath.Join(relPath, d.Name),
			})
		}
	}
	if window.FileLimit != 0 {
		files, err := s.Store.GetFileMetadataByDirPrefixPaged(drive.ID.String(), dirPrefixes, window.FileLimit, window.FileOffset)
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
