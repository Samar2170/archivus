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
	UploadFile(relPath, driveId, userId string, file multipart.File, fileHeader *multipart.FileHeader) error
	DownloadFile(fileId string, driveId, userId string) (*os.File, *models.FileMetadata, error)
	GetFiles(relPath, driveId, userId string) ([]storage_types.DirEntry, error)

	// V2: stores PathKey/Prefix correctly and returns full metadata from DB.
	CreateDirV2(subFolder, driveId, userId string) error
	UploadFileV2(relPath, driveId, userId string, file multipart.File, fileHeader *multipart.FileHeader) error
	GetFilesV2(relPath, driveId, userId string) ([]storage_types.DirEntry, error)

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
