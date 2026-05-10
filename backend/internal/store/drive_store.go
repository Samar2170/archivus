package store

import (
	"archivus/internal/models"
	"fmt"

	"github.com/google/uuid"
)

func (s *Store) CreateDrive(name, ownerID, slug, absPath string) (models.Drive, error) {
	id, err := uuid.Parse(ownerID)
	if err != nil {
		return models.Drive{}, fmt.Errorf("invalid owner ID: %w", err)
	}
	drive := models.Drive{
		Name:    name,
		OwnerID: id,
		Slug:    slug,
		AbsPath: absPath,
	}
	result := s.conn().Create(&drive)
	return drive, result.Error
}

func (s *Store) GetDriveByOwnerID(ownerID string) (models.Drive, error) {
	var drive models.Drive
	result := s.conn().First(&drive, "owner_id = ?", ownerID)
	return drive, result.Error
}

func (s *Store) GetDriveByID(driveID string) (models.Drive, error) {
	var drive models.Drive
	result := s.conn().First(&drive, "id = ?", driveID)
	return drive, result.Error
}

func (s *Store) GetDriveBySlug(slug string) (models.Drive, error) {
	var drive models.Drive
	result := s.conn().First(&drive, "slug = ?", slug)
	return drive, result.Error
}

func (s *Store) GetDriveByIDOrSlug(idOrSlug string) (models.Drive, error) {
	var drive models.Drive
	result := s.conn().First(&drive, "id = ? OR slug = ?", idOrSlug, idOrSlug)
	return drive, result.Error
}

func (s *Store) AddUserToDrive(userID, driveID string) error {
	userIDParsed, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return fmt.Errorf("invalid drive ID: %w", err)
	}
	return s.conn().Exec(
		"INSERT OR IGNORE INTO drive_users (drive_id, user_id) VALUES (?, ?)",
		driveIDParsed, userIDParsed,
	).Error
}

func (s *Store) GetUsersByDriveID(driveID string) ([]models.User, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return nil, fmt.Errorf("invalid drive ID: %w", err)
	}
	var drive models.Drive
	result := s.conn().Preload("Users").First(&drive, "id = ?", driveIDParsed.String())
	if result.Error != nil {
		return nil, result.Error
	}
	return drive.Users, nil
}

func (s *Store) CheckIfUserInDrive(userID, driveID string) (bool, error) {
	userIDParsed, err := uuid.Parse(userID)
	if err != nil {
		return false, fmt.Errorf("invalid user ID: %w", err)
	}
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return false, fmt.Errorf("invalid drive ID: %w", err)
	}
	var count int64
	result := s.conn().Table("drive_users").
		Where("drive_id = ? AND user_id = ?", driveIDParsed, userIDParsed).
		Count(&count)
	return count > 0, result.Error
}

func (s *Store) RemoveUserFromDrive(userID, driveID string) error {
	userIDParsed, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return fmt.Errorf("invalid drive ID: %w", err)
	}
	return s.conn().Exec(
		"DELETE FROM drive_users WHERE drive_id = ? AND user_id = ?",
		driveIDParsed, userIDParsed,
	).Error
}

func (s *Store) CreateDirectoryMetadata(name, absPath, driveID string) (models.DirectoryMetadata, error) {
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

func (s *Store) CreateFileMetadata(name, absPath, relPath, driveID, uploadedByID string, sizeInMb float64) (models.FileMetadata, error) {
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

func (s *Store) ResolveDriveBySlugOrID(slug, driveId string) (models.Drive, error) {
	if driveId != "" {
		return s.GetDriveByID(driveId)
	}
	if slug != "" {
		return s.GetDriveBySlug(slug)
	}
	return models.Drive{}, fmt.Errorf("either drive slug or drive ID must be provided")
}
