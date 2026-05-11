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
		AbsPath: absPath,
		DriveID: driveIDParsed,
	}
	result := s.conn().Create(&directoryMetadata)
	return directoryMetadata, result.Error
}

func (s *Store) CreateFileMetadata(name, absPath, relPath, dirPath, driveID, uploadedByID string, sizeInMb float64) (models.FileMetadata, error) {
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
		AbsPath:      absPath,
		RelPath:      relPath,
		DirPath:      dirPath,
		DriveID:      driveIDParsed,
		UploadedByID: uploadedByIDParsed,
		SizeInMb:     sizeInMb,
	}
	result := s.conn().Create(&fileMetadata)
	return fileMetadata, result.Error
}

func (s *Store) GetDirectoryMetadataByID(id int64) (models.DirectoryMetadata, error) {
	var directoryMetadata models.DirectoryMetadata
	result := s.conn().First(&directoryMetadata, "id = ?", id)
	return directoryMetadata, result.Error
}

func (s *Store) GetFileMetadataByID(id int64) (models.FileMetadata, error) {
	var fileMetadata models.FileMetadata
	result := s.conn().First(&fileMetadata, "id = ?", id)
	return fileMetadata, result.Error
}

func (s *Store) DeleteDirectoryMetadataByRelPath(relPath string) error {
	result := s.conn().Where("rel_path = ?", relPath).Delete(&models.DirectoryMetadata{})
	return result.Error
}

func (s *Store) DeleteFileMetadataByRelPath(relPath string) error {
	result := s.conn().Where("rel_path = ?", relPath).Delete(&models.FileMetadata{})
	return result.Error
}

func (s *Store) ListFilesByRelPath(relPath string) ([]models.FileMetadata, error) {
	var files []models.FileMetadata
	result := s.conn().Where("rel_path LIKE ?", relPath+"/%").Find(&files)
	if result.Error != nil {
		return nil, result.Error
	}
	return files, nil
}
