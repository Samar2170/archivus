package store

import (
	"archivus/internal/models"
	"fmt"

	"github.com/google/uuid"
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

func (s *Store) GetFileMetadataByPathKey(driveID, pathKey string) (models.FileMetadata, error) {
	var fileMetadata models.FileMetadata
	result := s.conn().Where("drive_id = ? AND path_key = ?", driveID, pathKey).First(&fileMetadata)
	return fileMetadata, result.Error
}

func (s *Store) GetDirectoryMetadataByPathKey(driveID, pathKey string) (models.DirectoryMetadata, error) {
	var dirMetadata models.DirectoryMetadata
	result := s.conn().Where("drive_id = ? AND path_key = ?", driveID, pathKey).First(&dirMetadata)
	return dirMetadata, result.Error
}

func (s *Store) UpdateFileMetadataByID(id string, fields map[string]interface{}) error {
	result := s.conn().Model(&models.FileMetadata{}).Where("id = ?", id).Updates(fields)
	return result.Error
}

func (s *Store) UpdateDirectoryMetadataByID(id string, fields map[string]interface{}) error {
	result := s.conn().Model(&models.DirectoryMetadata{}).Where("id = ?", id).Updates(fields)
	return result.Error
}

// ListFileMetadataBySubtree returns the file at pathKey plus every file beneath it.
func (s *Store) ListFileMetadataBySubtree(driveID, pathKey string) ([]models.FileMetadata, error) {
	var files []models.FileMetadata
	result := s.conn().
		Where("drive_id = ? AND (path_key = ? OR path_key LIKE ?)", driveID, pathKey, pathKey+"/%").
		Find(&files)
	return files, result.Error
}

// ListDirectoryMetadataBySubtree returns the directory at pathKey plus every directory beneath it.
func (s *Store) ListDirectoryMetadataBySubtree(driveID, pathKey string) ([]models.DirectoryMetadata, error) {
	var dirs []models.DirectoryMetadata
	result := s.conn().
		Where("drive_id = ? AND (path_key = ? OR path_key LIKE ?)", driveID, pathKey, pathKey+"/%").
		Find(&dirs)
	return dirs, result.Error
}

// DeleteSubtreeMetadata removes the file/directory rows at pathKey and everything beneath it.
func (s *Store) DeleteSubtreeMetadata(driveID, pathKey string) error {
	return s.Transaction(func(tx *Store) error {
		if err := tx.conn().
			Where("drive_id = ? AND (path_key = ? OR path_key LIKE ?)", driveID, pathKey, pathKey+"/%").
			Delete(&models.FileMetadata{}).Error; err != nil {
			return err
		}
		return tx.conn().
			Where("drive_id = ? AND (path_key = ? OR path_key LIKE ?)", driveID, pathKey, pathKey+"/%").
			Delete(&models.DirectoryMetadata{}).Error
	})
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
