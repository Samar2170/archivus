package store

import (
	"archivus/internal/models"
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Store) CreateDirectoryMetadata(name, absPath, relPath, driveID string) (models.DirectoryMetadata, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return models.DirectoryMetadata{}, fmt.Errorf("invalid drive ID: %w", err)
	}
	directoryMetadata := models.DirectoryMetadata{
		Name:    name,
		PathKey: relPath,
		DriveID: driveIDParsed,
	}
	result := s.conn().Create(&directoryMetadata)
	return directoryMetadata, result.Error
}

func (s *Store) CreateFileMetadata(name, relPath, dirPath, contentType, driveID, uploadedByID string, sizeInMb float64) (models.FileMetadata, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return models.FileMetadata{}, fmt.Errorf("invalid drive ID: %w", err)
	}
	uploadedByIDParsed, err := uuid.Parse(uploadedByID)
	if err != nil {
		return models.FileMetadata{}, fmt.Errorf("invalid uploaded by ID: %w", err)
	}
	fileMetadata := models.FileMetadata{
		Name:         name,
		PathKey:      relPath,
		Prefix:       dirPath,
		ContentType:  contentType,
		DriveID:      driveIDParsed,
		UploadedByID: uploadedByIDParsed,
		SizeInMb:     sizeInMb,
	}
	result := s.conn().Create(&fileMetadata)
	return fileMetadata, result.Error
}

func (s *Store) GetDirectoryMetadataByID(id string) (models.DirectoryMetadata, error) {
	var directoryMetadata models.DirectoryMetadata
	result := s.conn().First(&directoryMetadata, "id = ?", id)
	return directoryMetadata, result.Error
}

func (s *Store) GetFileMetadataByID(id string) (models.FileMetadata, error) {
	var fileMetadata models.FileMetadata
	result := s.conn().First(&fileMetadata, "id = ?", id)
	return fileMetadata, result.Error
}

func (s *Store) DeleteDirectoryMetadataByRelPath(relPath string) error {
	result := s.conn().Where("path_key = ?", relPath).Delete(&models.DirectoryMetadata{})
	return result.Error
}

func (s *Store) DeleteFileMetadataByRelPath(relPath string) error {
	result := s.conn().Where("path_key = ?", relPath).Delete(&models.FileMetadata{})
	return result.Error
}

// DeleteFileMetadataByID permanently removes a file metadata row. Used when a
// file is moved into the recycle bin (the RecycleBinItem row then owns it).
func (s *Store) DeleteFileMetadataByID(id string) error {
	result := s.conn().Unscoped().Where("id = ?", id).Delete(&models.FileMetadata{})
	return result.Error
}

// UpdateFileMetadataLocation moves a file row to a new key/prefix after its
// bytes have been relocated on disk/S3.
func (s *Store) UpdateFileMetadataLocation(id, pathKey, prefix string) error {
	result := s.conn().Model(&models.FileMetadata{}).Where("id = ?", id).Updates(map[string]interface{}{
		"path_key": pathKey,
		"prefix":   prefix,
	})
	return result.Error
}

func (s *Store) ListFilesByRelPath(dirAbsPath string) ([]models.FileMetadata, error) {
	var files []models.FileMetadata
	result := s.conn().Where("path_key LIKE ?", dirAbsPath+"/%").Find(&files)
	if result.Error != nil {
		return nil, result.Error
	}
	return files, nil
}

func (s *Store) GetFileMetadataByRelPath(relPath string) (models.FileMetadata, error) {
	var fileMetadata models.FileMetadata
	result := s.conn().Where("rel_path = ?", relPath).First(&fileMetadata)
	return fileMetadata, result.Error
}

func (s *Store) UpdateFileMetadataPaths(id, absPath, s3Key, relPath, dirPath string) error {
	result := s.conn().Model(&models.FileMetadata{}).Where("id = ?", id).Updates(map[string]interface{}{
		"abs_path": absPath,
		"s3_key":   s3Key,
		"rel_path": relPath,
		"dir_path": dirPath,
	})
	return result.Error
}

// V2 methods: PathKey and Prefix are stored correctly.
// PathKey = the object/file key (S3 key or absolute disk path).
// Prefix  = the parent directory key with trailing slash (S3) or absolute dir path (disk).

func (s *Store) CreateFileMetadataV2(name, pathKey, prefix, contentType, driveID, uploadedByID string, sizeInMb float64) (models.FileMetadata, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return models.FileMetadata{}, fmt.Errorf("invalid drive ID: %w", err)
	}
	uploadedByIDParsed, err := uuid.Parse(uploadedByID)
	if err != nil {
		return models.FileMetadata{}, fmt.Errorf("invalid uploaded by ID: %w", err)
	}
	fm := models.FileMetadata{
		Name:         name,
		PathKey:      pathKey,
		Prefix:       prefix,
		ContentType:  contentType,
		DriveID:      driveIDParsed,
		UploadedByID: uploadedByIDParsed,
		SizeInMb:     sizeInMb,
	}
	result := s.conn().Create(&fm)
	return fm, result.Error
}

func (s *Store) CreateDirectoryMetadataV2(name, pathKey, prefix, driveID string) (models.DirectoryMetadata, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return models.DirectoryMetadata{}, fmt.Errorf("invalid drive ID: %w", err)
	}
	dm := models.DirectoryMetadata{
		Name:    name,
		PathKey: pathKey,
		Prefix:  prefix,
		DriveID: driveIDParsed,
	}
	result := s.conn().Create(&dm)
	return dm, result.Error
}

// GetFileMetadataByDrivePathKey looks up a single file by its exact key within a
// drive. Returns ErrRecordNotFound when no such file exists, which upload paths
// use to distinguish a first upload from an overwrite/new-version.
func (s *Store) GetFileMetadataByDrivePathKey(driveID, pathKey string) (models.FileMetadata, error) {
	var fm models.FileMetadata
	result := s.conn().Where("drive_id = ? AND path_key = ?", driveID, pathKey).First(&fm)
	return fm, result.Error
}

// UpdateFileMetadataContent updates a file row in place after its bytes were
// overwritten (home mode) or replaced by a newer version (biz mode). The
// thumbnail path is cleared so the thumbnail job regenerates it from the new
// content.
func (s *Store) UpdateFileMetadataContent(id string, sizeInMb float64, contentType string) error {
	result := s.conn().Model(&models.FileMetadata{}).Where("id = ?", id).Updates(map[string]interface{}{
		"size_in_mb":     sizeInMb,
		"content_type":   contentType,
		"thumbnail_path": "",
	})
	return result.Error
}

func (s *Store) GetFileMetadataByDirPrefix(driveID string, prefixes [2]string) ([]models.FileMetadata, error) {
	var files []models.FileMetadata
	result := s.conn().Where("drive_id = ? AND prefix IN ?", driveID, prefixes).Find(&files)
	return files, result.Error
}

func (s *Store) GetDirectoriesByParentPrefix(driveID string, prefixes [2]string) ([]models.DirectoryMetadata, error) {
	var dirs []models.DirectoryMetadata
	result := s.conn().Where("drive_id = ? AND prefix IN ?", driveID, prefixes).Find(&dirs)
	return dirs, result.Error
}

// CountFileMetadataByDirPrefix returns the number of files directly under the
// given directory prefixes, used to page the combined file/directory listing.
func (s *Store) CountFileMetadataByDirPrefix(driveID string, prefixes [2]string) (int64, error) {
	var count int64
	result := s.conn().Model(&models.FileMetadata{}).Where("drive_id = ? AND prefix IN ?", driveID, prefixes).Count(&count)
	return count, result.Error
}

// CountDirectoriesByParentPrefix returns the number of directories directly
// under the given parent prefixes.
func (s *Store) CountDirectoriesByParentPrefix(driveID string, prefixes [2]string) (int64, error) {
	var count int64
	result := s.conn().Model(&models.DirectoryMetadata{}).Where("drive_id = ? AND prefix IN ?", driveID, prefixes).Count(&count)
	return count, result.Error
}

// GetFileMetadataByDirPrefixPaged returns files under the given prefixes ordered
// by name, sliced by limit/offset. A negative limit disables the limit.
func (s *Store) GetFileMetadataByDirPrefixPaged(driveID string, prefixes [2]string, limit, offset int) ([]models.FileMetadata, error) {
	var files []models.FileMetadata
	result := s.conn().Where("drive_id = ? AND prefix IN ?", driveID, prefixes).
		Order("name ASC").Limit(limit).Offset(offset).Find(&files)
	return files, result.Error
}

// GetDirectoriesByParentPrefixPaged returns directories under the given prefixes
// ordered by name, sliced by limit/offset. A negative limit disables the limit.
func (s *Store) GetDirectoriesByParentPrefixPaged(driveID string, prefixes [2]string, limit, offset int) ([]models.DirectoryMetadata, error) {
	var dirs []models.DirectoryMetadata
	result := s.conn().Where("drive_id = ? AND prefix IN ?", driveID, prefixes).
		Order("name ASC").Limit(limit).Offset(offset).Find(&dirs)
	return dirs, result.Error
}

func (s *Store) GetFileMetadatasWoThumbnails(ctx context.Context, limit int) ([]models.FileMetadata, error) {
	var files []models.FileMetadata
	result := s.conn().Where("thumbnail_path IS NULL OR thumbnail_path = '' AND is_image = true").Limit(limit).Find(&files)
	return files, result.Error
}

func (s *Store) UpdateFileMetadataThumbnailPath(id, thumbnailPath string) error {
	result := s.conn().Model(&models.FileMetadata{}).Where("id = ?", id).Update("thumbnail_path", thumbnailPath)
	return result.Error
}

func (s *Store) GetFileMetadatasWoExtension(limit int) ([]models.FileMetadata, error) {
	var files []models.FileMetadata
	result := s.conn().Where("extension = ''").Limit(limit).Find(&files)
	return files, result.Error
}

// MarkFileMetadatasAsImages flags the given file rows as images in a single
// query.
func (s *Store) MarkFileMetadatasAsImages(ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	result := s.conn().Model(&models.FileMetadata{}).Where("id IN ?", ids).Update("is_image", true)
	return result.Error
}

// UpdateFileMetadataExtensions sets the extension for many file rows in a
// single query using a CASE expression keyed by file ID.
func (s *Store) UpdateFileMetadataExtensions(extensionsByID map[uuid.UUID]string) error {
	if len(extensionsByID) == 0 {
		return nil
	}
	caseSQL := "CASE id "
	args := make([]interface{}, 0, len(extensionsByID)*2)
	ids := make([]uuid.UUID, 0, len(extensionsByID))
	for id, ext := range extensionsByID {
		caseSQL += "WHEN ? THEN ? "
		args = append(args, id, ext)
		ids = append(ids, id)
	}
	caseSQL += "END"
	result := s.conn().Model(&models.FileMetadata{}).
		Where("id IN ?", ids).
		Update("extension", gorm.Expr(caseSQL, args...))
	return result.Error
}
