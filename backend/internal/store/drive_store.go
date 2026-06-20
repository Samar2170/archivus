package store

import (
	"archivus/internal/models"
	"context"
	"fmt"

	"github.com/google/uuid"
)

func (s *Store) CreateDrive(name, ownerID, slug, prefix string, ownerType models.UserType) (models.Drive, error) {
	id, err := uuid.Parse(ownerID)
	if err != nil {
		return models.Drive{}, fmt.Errorf("invalid owner ID: %w", err)
	}
	drive := models.Drive{
		Name:      name,
		OwnerID:   id,
		OwnerType: ownerType,
		Slug:      slug,
		Prefix:    prefix,
		Path:      prefix + "/" + slug,
	}
	result := s.conn().Create(&drive)
	return drive, result.Error
}

func (s *Store) GetDriveByOwnerID(ownerID string) ([]models.Drive, error) {
	var drives []models.Drive
	result := s.conn().Find(&drives, "owner_id = ?", ownerID)
	return drives, result.Error
}

type DriveUser struct {
	UserID      string
	DriveID     string
	DriveName   string
	AccessLevel models.AccessLevel
}

func (s *Store) GetDriveByUserID(userID string) ([]DriveUser, error) {
	var readdrives []models.Drive
	var writedrives []models.Drive
	var managerdrives []models.Drive
	result := s.conn().Joins("JOIN drive_read_users ON drive_read_users.drive_id = drives.id").
		Where("drive_read_users.user_id = ?", userID).
		Find(&readdrives)
	result = s.conn().Joins("JOIN drive_write_users ON drive_write_users.drive_id = drives.id").
		Where("drive_write_users.user_id = ?", userID).
		Find(&writedrives)
	result = s.conn().Joins("JOIN drive_manager_users ON drive_manager_users.drive_id = drives.id").
		Where("drive_manager_users.user_id = ?", userID).
		Find(&managerdrives)
	var driveUserData []DriveUser
	for _, drive := range readdrives {
		driveUserData = append(driveUserData, DriveUser{
			UserID:      userID,
			DriveID:     drive.ID.String(),
			DriveName:   drive.Name,
			AccessLevel: models.AccessLevelRead,
		})
	}
	for _, drive := range writedrives {
		driveUserData = append(driveUserData, DriveUser{
			UserID:      userID,
			DriveID:     drive.ID.String(),
			DriveName:   drive.Name,
			AccessLevel: models.AccessLevelWrite,
		})
	}
	for _, drive := range managerdrives {
		driveUserData = append(driveUserData, DriveUser{
			UserID:      userID,
			DriveID:     drive.ID.String(),
			DriveName:   drive.Name,
			AccessLevel: models.AccessLevelManager,
		})
	}
	return driveUserData, result.Error
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
		"INSERT OR IGNORE INTO drive_read_users (drive_id, user_id) VALUES (?, ?)",
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
		if access == models.AccessLevelRead {
			if err := tx.addUserToDriveRead(userIDParsed, driveIDParsed); err != nil {
				return fmt.Errorf("granting read: %w", err)
			}
			return nil
		}
		if access == models.AccessLevelWrite {
			if err := tx.addUserToDriveWrite(userIDParsed, driveIDParsed); err != nil {
				return fmt.Errorf("granting write: %w", err)
			}
			return nil
		}
		if access == models.AccessLevelManager {
			if err := tx.addUserToDriveManager(userIDParsed, driveIDParsed); err != nil {
				return fmt.Errorf("granting manager: %w", err)
			}
			return nil
		}
		return fmt.Errorf("invalid access level: %s", access)
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
