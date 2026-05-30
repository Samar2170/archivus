package store

import (
	"archivus/internal/models"
	"context"
	"fmt"

	"github.com/google/uuid"
)

func (s *Store) CreateDrive(name, ownerID, slug, prefix string) (models.Drive, error) {
	id, err := uuid.Parse(ownerID)
	if err != nil {
		return models.Drive{}, fmt.Errorf("invalid owner ID: %w", err)
	}
	drive := models.Drive{
		Name:    name,
		OwnerID: id,
		Slug:    slug,
		Prefix:  prefix,
		Path:    prefix + "/" + slug,
	}
	result := s.conn().Create(&drive)
	return drive, result.Error
}

func (s *Store) GetDriveByOwnerID(ownerID string) ([]models.Drive, error) {
	var drives []models.Drive
	result := s.conn().Find(&drives, "owner_id = ?", ownerID)
	return drives, result.Error
}

func (s *Store) GetDriveByUserID(userID string) ([]models.Drive, error) {
	var drives []models.Drive
	result := s.conn().Joins("JOIN drive_users ON drive_users.drive_id = drives.id").
		Where("drive_users.user_id = ?", userID).
		Find(&drives)
	return drives, result.Error
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

func (s *Store) addUserToDriveRead(userIDParsed, driveIDParsed uuid.UUID) error {
	return s.conn().Exec(
		"INSERT OR IGNORE INTO drive_users (drive_id, user_id) VALUES (?, ?)",
		driveIDParsed, userIDParsed,
	).Error
}

func (s *Store) addUserToDriveWrite(userIDParsed, driveIDParsed uuid.UUID) error {
	return s.conn().Exec(
		"INSERT OR IGNORE INTO drive_write_users (drive_id, user_id) VALUES (?, ?)",
		driveIDParsed, userIDParsed,
	).Error
}

func (s *Store) addUserToDriveManager(userIDParsed, driveIDParsed uuid.UUID) error {
	return s.conn().Exec(
		"INSERT OR IGNORE INTO drive_manager_users (drive_id, user_id) VALUES (?, ?)",
		driveIDParsed, userIDParsed,
	).Error
}

func (s *Store) AddUserToDrive(ctx context.Context, userID, driveID string, access models.AccessLevel) error {
	userIDParsed, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return fmt.Errorf("invalid drive ID: %w", err)
	}

	return s.Transaction(func(tx *Store) error {
		if err := tx.addUserToDriveRead(userIDParsed, driveIDParsed); err != nil {
			return fmt.Errorf("granting read: %w", err)
		}
		if access == models.AccessLevelRead {
			return nil
		}
		if err := tx.addUserToDriveWrite(userIDParsed, driveIDParsed); err != nil {
			return fmt.Errorf("granting write: %w", err)
		}
		if access == models.AccessLevelWrite {
			return nil
		}
		if err := tx.addUserToDriveManager(userIDParsed, driveIDParsed); err != nil {
			return fmt.Errorf("granting manager: %w", err)
		}
		return nil
	})
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
	return drive.ReadUsers, nil
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
	fmt.Println(driveID, driveIDParsed, userID, userIDParsed)
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

func (s *Store) ResolveDriveBySlugOrID(slug, driveId string) (models.Drive, error) {
	if driveId != "" {
		return s.GetDriveByID(driveId)
	}
	if slug != "" {
		return s.GetDriveBySlug(slug)
	}
	return models.Drive{}, fmt.Errorf("either drive slug or drive ID must be provided")
}
