package storagemanager

import (
	"archivus/internal/models"
	storage_types "archivus/internal/services/storagemanager/types"
	"io"
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

	// WebDAV primitives. relPath is the drive-relative path ("" == drive root).
	// StatPath resolves a path to file-or-directory info.
	StatPath(relPath, driveId, userId string) (*storage_types.StatInfo, error)
	// ReadFile opens a file for reading. Caller must Close the returned reader.
	ReadFile(relPath, driveId, userId string) (io.ReadSeekCloser, *storage_types.StatInfo, error)
	// WriteFileStream creates or overwrites the file at relPath from r.
	WriteFileStream(relPath, driveId, userId string, r io.Reader, size int64, contentType string) error
	// Remove deletes a file or a directory subtree at relPath.
	Remove(relPath, driveId, userId string) error
	// Rename moves a file or directory from oldRelPath to newRelPath.
	Rename(oldRelPath, newRelPath, driveId, userId string) error
}
