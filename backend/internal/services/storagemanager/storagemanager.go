package storagemanager

import (
	"archivus/internal/models"
	storage_types "archivus/internal/services/storagemanager/types"
	"mime/multipart"
	"os"
)

type StorageManager interface {
	CreateDriveDir(driveName string) (string, error)
	DeleteDriveDir(driveName string) error

	CreateDir(subFolder, driveId, userId string) error
	DeleteDir(relPath, driveId, userId string) error
	UploadFile(relPath, driveId, userId string, file multipart.File, fileHeader *multipart.FileHeader) error
	// MoveFile(srcRelPath, dstRelPath, driveId, userId string) error
	DownloadFile(fileId string, driveId, userId string) (*os.File, *models.FileMetadata, error)
	GetFiles(relPath, driveId, userId string) ([]storage_types.DirEntry, error)

	// V2: stores PathKey/Prefix correctly and returns full metadata from DB.
	CreateDirV2(subFolder, driveId, userId string) error
	UploadFileV2(relPath, driveId, userId string, file multipart.File, fileHeader *multipart.FileHeader) error
	GetFilesV2(relPath, driveId, userId string) ([]storage_types.DirEntry, error)
}
