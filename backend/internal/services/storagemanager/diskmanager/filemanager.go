package diskmanager

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
	"net/textproto"
	"os"
	"path/filepath"
)

func (dm *DiskManager) UploadFile(relPath, driveId, userId string, file multipart.File, fileHeader *multipart.FileHeader) error {
	hasAccess, err := dm.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}
	dirPath := filepath.Join(dm.Home, drive.Slug, relPath)
	relPath = filepath.Join(drive.Slug, relPath, fileHeader.Filename)
	absPath := filepath.Join(dm.Home, relPath)
	outFile, err := os.Create(absPath)
	if err != nil {
		return fmt.Errorf("diskmanager: create file %q: %w", absPath, err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, file); err != nil {
		return fmt.Errorf("diskmanager: save file %q: %w", absPath, err)
	}

	sizeInMb := float64(fileHeader.Size) / (1 << 20)
	_, err = dm.Store.CreateFileMetadata(fileHeader.Filename, absPath, relPath, dirPath, driveId, userId, sizeInMb)
	if err != nil {
		err = os.Remove(absPath) // cleanup uploaded file on failure
		if err != nil {
			fmt.Printf("warning: failed to clean up file after db error: %v\n", err)
		}
		return fmt.Errorf("diskmanager: create file metadata for file %q: %w", absPath, err)
	}
	return nil
}

// func (dm *DiskManager) MoveFile(srcRelPath, dstRelPath, driveId, userId string) error {
// 	hasAccess, err := dm.CheckUserDriveWriteAccess(userId, driveId)
// 	if err != nil {
// 		return err
// 	}
// 	if !hasAccess {
// 		return errors.New("user does not have write access to this drive")
// 	}
// 	drive, err := dm.Store.GetDriveByID(driveId)
// 	if err != nil {
// 		return fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
// 	}

// 	srcAbs := filepath.Join(dm.Home, drive.Slug, srcRelPath)
// 	dstAbs := filepath.Join(dm.Home, drive.Slug, dstRelPath)

// 	md, err := dm.Store.GetFileMetadataByRelPath(filepath.Join(drive.Slug, srcRelPath))
// 	if err != nil {
// 		return fmt.Errorf("diskmanager: get file metadata for %q: %w", srcRelPath, err)
// 	}

// 	if err := os.Rename(srcAbs, dstAbs); err != nil {
// 		return fmt.Errorf("diskmanager: move file %q to %q: %w", srcAbs, dstAbs, err)
// 	}

// 	newRelPath := filepath.Join(drive.Slug, dstRelPath)
// 	newDirPath := filepath.Join(dm.Home, drive.Slug, filepath.Dir(dstRelPath))

// 	if err := dm.Store.UpdateFileMetadataPaths(md.ID, dstAbs, newRelPath, newDirPath); err != nil {
// 		if rerr := os.Rename(dstAbs, srcAbs); rerr != nil {
// 			fmt.Printf("warning: failed to revert file move after db error: %v\n", rerr)
// 		}
// 		return fmt.Errorf("diskmanager: update file metadata after move: %w", err)
// 	}
// 	return nil
// }

func (dm *DiskManager) DownloadFile(fileId string, driveId, userId string) (*os.File, *models.FileMetadata, error) {
	hasAccess, err := dm.CheckUserHasDriveAccess(userId, driveId)
	if err != nil {
		return nil, nil, err
	}
	if !hasAccess {
		return nil, nil, errors.New("user does not have access to this drive")
	}

	md, err := dm.Store.GetFileMetadataByID(fileId)
	if err != nil {
		return nil, nil, fmt.Errorf("diskmanager: get file metadata by id %q: %w", fileId, err)
	}

	f, err := os.Open(md.PathKey)
	if err != nil {
		return nil, nil, fmt.Errorf("diskmanager: open file %q: %w", md.PathKey, err)
	}
	return f, &md, nil
}

// UploadFileV2 stores a file in a drive. Home mode is single-version: if a file
// already exists at the same key its bytes are overwritten and the existing
// metadata row is updated in place (rather than inserting a duplicate row).
func (dm *DiskManager) UploadFileV2(relPath, driveId, userId string, file multipart.File, fileHeader *multipart.FileHeader) error {
	hasAccess, err := dm.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}
	prefix := filepath.Join(dm.Home, drive.Slug, relPath)
	pathKey := filepath.Join(prefix, fileHeader.Filename)

	// Determine whether this is a first upload or an overwrite before touching disk.
	existing, lookupErr := dm.Store.GetFileMetadataByDrivePathKey(driveId, pathKey)
	isOverwrite := lookupErr == nil
	if lookupErr != nil && !errors.Is(lookupErr, store.ErrRecordNotFound) {
		return fmt.Errorf("diskmanager: lookup existing file %q: %w", pathKey, lookupErr)
	}

	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return fmt.Errorf("diskmanager: create dir %q: %w", prefix, err)
	}
	outFile, err := os.Create(pathKey) // truncates on overwrite
	if err != nil {
		return fmt.Errorf("diskmanager: create file %q: %w", pathKey, err)
	}
	defer outFile.Close()
	if _, err := io.Copy(outFile, file); err != nil {
		return fmt.Errorf("diskmanager: save file %q: %w", pathKey, err)
	}
	contentType := fileHeader.Header.Get("Content-Type")
	sizeInMb := float64(fileHeader.Size) / (1 << 20)

	if isOverwrite {
		if err := dm.Store.UpdateFileMetadataContent(existing.ID.String(), sizeInMb, contentType); err != nil {
			return fmt.Errorf("diskmanager: update file metadata for %q: %w", pathKey, err)
		}
		return nil
	}
	_, err = dm.Store.CreateFileMetadataV2(fileHeader.Filename, pathKey, prefix, contentType, driveId, userId, sizeInMb)
	if err != nil {
		if cleanupErr := os.Remove(pathKey); cleanupErr != nil {
			fmt.Printf("warning: failed to clean up file after db error: %v\n", cleanupErr)
		}
		return fmt.Errorf("diskmanager: create file metadata for file %q: %w", pathKey, err)
	}
	return nil
}

// EnqueueChunkedUpload persists an assembled chunked upload. Writing to local
// disk is fast, so unlike the object-storage backend there is nothing to defer:
// the file is written synchronously through the normal UploadFileV2 path and
// returned already "ready". The staged assembled file is removed afterwards.
func (dm *DiskManager) EnqueueChunkedUpload(relPath, driveId, userId, contentType string, size int64, filename, localPath string) (models.FileMetadata, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return models.FileMetadata{}, fmt.Errorf("diskmanager: open staged file %q: %w", localPath, err)
	}
	defer f.Close()

	header := &multipart.FileHeader{Filename: filename, Size: size, Header: textproto.MIMEHeader{}}
	if contentType != "" {
		header.Header.Set("Content-Type", contentType)
	}
	if err := dm.UploadFileV2(relPath, driveId, userId, f, header); err != nil {
		return models.FileMetadata{}, err
	}
	if err := os.Remove(localPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Printf("warning: failed to remove staged file %q: %v\n", localPath, err)
	}

	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return models.FileMetadata{}, fmt.Errorf("diskmanager: get drive %q: %w", driveId, err)
	}
	pathKey := filepath.Join(dm.Home, drive.Slug, relPath, filename)
	fm, err := dm.Store.GetFileMetadataByDrivePathKey(driveId, pathKey)
	if err != nil {
		return models.FileMetadata{}, fmt.Errorf("diskmanager: lookup persisted file %q: %w", pathKey, err)
	}
	return fm, nil
}

// ProcessPendingUploads is a no-op for local disk: EnqueueChunkedUpload already
// persists synchronously, so there is never a pending queue to drain.
func (dm *DiskManager) ProcessPendingUploads(ctx context.Context) error {
	return nil
}

// PendingBacklogFull is always false for local disk: nothing is ever deferred,
// so there is no staging backlog that could need clients to slow down.
func (dm *DiskManager) PendingBacklogFull() (bool, error) {
	return false, nil
}

func (dm *DiskManager) GetFilesV2(relPath, driveId, userId string, page, pageSize int, opts storage_types.ListOptions) (storage_types.PagedDirEntries, error) {
	var out storage_types.PagedDirEntries
	hasAccess, err := dm.CheckUserHasDriveAccess(userId, driveId)
	if err != nil {
		return out, err
	}
	if !hasAccess {
		return out, errors.New("user does not have access to this drive")
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return out, fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}
	dirPrefixes := [2]string{
		filepath.Join(dm.Home, drive.Slug, relPath),
		filepath.Join(dm.Home, drive.Slug, relPath) + "/",
	}

	limit, offset := storage_types.PageBounds(page, pageSize)
	dirCount, err := dm.Store.CountDirectoriesByParentPrefix(drive.ID.String(), dirPrefixes)
	if err != nil {
		return out, fmt.Errorf("diskmanager: count dirs for prefix %q: %w", dirPrefixes, err)
	}
	fileCount, err := dm.Store.CountFileMetadataByDirPrefix(drive.ID.String(), dirPrefixes, opts.ContentType)
	if err != nil {
		return out, fmt.Errorf("diskmanager: count files for prefix %q: %w", dirPrefixes, err)
	}
	window := storage_types.PageWindow(int(dirCount), limit, offset)

	entries := make([]storage_types.DirEntry, 0, limit)
	if window.DirLimit != 0 {
		dirs, err := dm.Store.GetDirectoriesByParentPrefixPaged(drive.ID.String(), dirPrefixes, window.DirLimit, window.DirOffset, opts.SortBy, opts.SortOrder)
		if err != nil {
			return out, fmt.Errorf("diskmanager: list dirs for prefix %q: %w", dirPrefixes, err)
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
		files, err := dm.Store.GetFileMetadataByDirPrefixPaged(drive.ID.String(), dirPrefixes, window.FileLimit, window.FileOffset, opts.SortBy, opts.SortOrder, opts.ContentType)
		if err != nil {
			return out, fmt.Errorf("diskmanager: list files for prefix %q: %w", dirPrefixes, err)
		}
		for _, f := range files {
			entries = append(entries, storage_types.DirEntry{
				ID:             f.ID.String(),
				Name:           f.Name,
				IsDir:          false,
				Extension:      filepath.Ext(f.Name),
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

func (dm *DiskManager) GetFiles(relPath, driveId, userId string) ([]storage_types.DirEntry, error) {
	hasAccess, err := dm.CheckUserHasDriveAccess(userId, driveId)
	if err != nil {
		return nil, err
	}
	if !hasAccess {
		return nil, errors.New("user does not have access to this drive")
	}
	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return nil, fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}
	files, err := os.ReadDir(filepath.Join(dm.Home, drive.Slug, relPath))
	if err != nil {
		return nil, fmt.Errorf("diskmanager: read directory %q: %w", filepath.Join(dm.Home, drive.Slug, relPath), err)
	}
	mds, err := dm.Store.ListFilesByRelPath(filepath.Join(dm.Home, drive.Slug, relPath))
	if err != nil {
		return nil, fmt.Errorf("diskmanager: list files by directory path %q: %w", filepath.Join(drive.Slug, relPath), err)
	}
	mdIdMap := make(map[string]string)
	for _, md := range mds {
		mdIdMap[md.Name] = md.ID.String()
	}
	var dirEntries []storage_types.DirEntry
	for _, entry := range files {
		d := storage_types.DirEntry{
			ID:             mdIdMap[entry.Name()],
			Name:           entry.Name(),
			IsDir:          entry.IsDir(),
			Extension:      filepath.Ext(entry.Name()),
			Path:           filepath.Join(drive.Slug, relPath, entry.Name()),
			NavigationPath: filepath.Join(relPath, entry.Name()),
		}
		dirEntries = append(dirEntries, d)
	}
	return dirEntries, nil

}
