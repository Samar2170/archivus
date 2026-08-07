package storagemanager

import (
	"archivus/internal/models"
	storage_types "archivus/internal/services/storagemanager/types"
	"context"
	"mime/multipart"
	"os"
)

type StorageManager interface {
	CreateDriveDir(driveName string) (string, error)
	DeleteDriveDir(driveName string) error

	CreateDir(subFolder, driveId, userId string) error
	DeleteDir(relPath, driveId, userId string) error
	// DirExists reports whether relPath (relative to the drive root) is an
	// existing folder in the drive. An empty relPath means the drive root, which
	// always exists. Write access to the drive is required, so it cannot be used
	// to probe folders in drives the caller cannot write to.
	DirExists(relPath, driveId, userId string) (bool, error)
	UploadFile(relPath, driveId, userId string, file multipart.File, fileHeader *multipart.FileHeader) error
	DownloadFile(fileId string, driveId, userId string) (*os.File, *models.FileMetadata, error)
	GetFiles(relPath, driveId, userId string) ([]storage_types.DirEntry, error)

	// V2: stores PathKey/Prefix correctly and returns full metadata from DB.
	CreateDirV2(subFolder, driveId, userId string) error
	UploadFileV2(relPath, driveId, userId string, file multipart.File, fileHeader *multipart.FileHeader) error
	GetFilesV2(relPath, driveId, userId string, page, pageSize int) (storage_types.PagedDirEntries, error)

	// EnqueueChunkedUpload registers an assembled chunked upload for persistence.
	// localPath is the on-disk assembled file. For backends where writing is slow
	// (object storage) the row is created as pending and the actual push is
	// deferred to ProcessPendingUploads; for fast local backends it may persist
	// synchronously and return a ready row. The caller must not remove localPath;
	// ownership transfers to the storage manager.
	EnqueueChunkedUpload(relPath, driveId, userId, contentType string, size int64, filename, localPath string) (models.FileMetadata, error)
	// ProcessPendingUploads pushes any pending assembled uploads to the backend
	// and flips them to ready. It is safe to call concurrently: rows are claimed
	// atomically, so overlapping callers never upload the same file twice.
	ProcessPendingUploads(ctx context.Context) error

	// MoveFile relocates a file into dstFolderRelPath (relative to the drive
	// root), keeping its name.
	MoveFile(fileId, dstFolderRelPath, driveId, userId string) error

	// Recycle bin: DeleteFileV2 soft-deletes a file into the recycle bin,
	// RestoreFile returns it to its original location, ListRecycleBin lists a
	// drive's deleted files, and PurgeExpiredRecycleBin permanently removes
	// items past their retention window (run by the cron service).
	DeleteFileV2(fileId, driveId, userId string) error
	RestoreFile(recycleBinId, driveId, userId string) error
	ListRecycleBin(driveId, userId string) ([]storage_types.RecycleEntry, error)
	PurgeExpiredRecycleBin(ctx context.Context) error
}
