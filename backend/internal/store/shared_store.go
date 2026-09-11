package store

import (
	"archivus/internal/models"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm/clause"
)

// GrantSharedFolder upserts a folder share. Re-granting an existing share
// (same drive, root and user) updates its access level and granter instead of
// failing on the unique constraint.
func (s *Store) GrantSharedFolder(driveID, rootPathKey, sharedWithUserID, grantedByUserID string, access models.AccessLevel) (models.SharedInfo, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return models.SharedInfo{}, fmt.Errorf("invalid drive ID: %w", err)
	}
	sharedWithParsed, err := uuid.Parse(sharedWithUserID)
	if err != nil {
		return models.SharedInfo{}, fmt.Errorf("invalid shared-with user ID: %w", err)
	}
	grantedByParsed, err := uuid.Parse(grantedByUserID)
	if err != nil {
		return models.SharedInfo{}, fmt.Errorf("invalid granted-by user ID: %w", err)
	}
	share := models.SharedInfo{
		DriveID:          driveIDParsed,
		RootPathKey:      rootPathKey,
		SharedWithUserID: sharedWithParsed,
		AccessLevel:      access,
		GrantedByUserID:  grantedByParsed,
	}
	result := s.conn().Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "drive_id"},
			{Name: "root_path_key"},
			{Name: "shared_with_user_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"access_level", "granted_by_user_id"}),
	}).Create(&share)
	return share, result.Error
}

// RevokeSharedFolderByScope removes a user's share on a folder. Revocation is a
// hard delete so the unique scope frees up for future re-grants and access is
// lost immediately.
func (s *Store) RevokeSharedFolderByScope(driveID, rootPathKey, sharedWithUserID string) error {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return fmt.Errorf("invalid drive ID: %w", err)
	}
	sharedWithParsed, err := uuid.Parse(sharedWithUserID)
	if err != nil {
		return fmt.Errorf("invalid shared-with user ID: %w", err)
	}
	result := s.conn().Unscoped().Where(
		"drive_id = ? AND root_path_key = ? AND shared_with_user_id = ?",
		driveIDParsed, rootPathKey, sharedWithParsed,
	).Delete(&models.SharedInfo{})
	return result.Error
}

// GetSharedFolderByScope returns the share granting userID access to
// rootPathKey in driveID, or ErrRecordNotFound when there is none.
func (s *Store) GetSharedFolderByScope(driveID, rootPathKey, userID string) (models.SharedInfo, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return models.SharedInfo{}, fmt.Errorf("invalid drive ID: %w", err)
	}
	userIDParsed, err := uuid.Parse(userID)
	if err != nil {
		return models.SharedInfo{}, fmt.Errorf("invalid user ID: %w", err)
	}
	var share models.SharedInfo
	result := s.conn().Where(
		"drive_id = ? AND root_path_key = ? AND shared_with_user_id = ?",
		driveIDParsed, rootPathKey, userIDParsed,
	).First(&share)
	return share, result.Error
}

// ListSharedFoldersForDrive returns all of a user's folder shares within one
// drive, for ancestor-match resolution of a requested path.
func (s *Store) ListSharedFoldersForDrive(driveID, userID string) ([]models.SharedInfo, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return nil, fmt.Errorf("invalid drive ID: %w", err)
	}
	userIDParsed, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	var shares []models.SharedInfo
	result := s.conn().Where("drive_id = ? AND shared_with_user_id = ?", driveIDParsed, userIDParsed).Find(&shares)
	return shares, result.Error
}

// ListSharedFoldersForUser returns every folder share granted to a user across
// all drives, with the drive and granter loaded for display.
func (s *Store) ListSharedFoldersForUser(userID string) ([]models.SharedInfo, error) {
	userIDParsed, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	var shares []models.SharedInfo
	result := s.conn().Preload("Drive").Preload("GrantedBy").
		Where("shared_with_user_id = ?", userIDParsed).Find(&shares)
	return shares, result.Error
}

// ListSharedFoldersForFolder returns every share on one folder, with the
// shared-with user loaded, for the share management UI.
func (s *Store) ListSharedFoldersForFolder(driveID, rootPathKey string) ([]models.SharedInfo, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return nil, fmt.Errorf("invalid drive ID: %w", err)
	}
	var shares []models.SharedInfo
	result := s.conn().Preload("SharedWith").
		Where("drive_id = ? AND root_path_key = ?", driveIDParsed, rootPathKey).Find(&shares)
	return shares, result.Error
}

// CountSharedFoldersByDrive is a small helper used by tests and future
// cleanup jobs to see how many shares reference a drive.
func (s *Store) CountSharedFoldersByDrive(driveID string) (int64, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return 0, fmt.Errorf("invalid drive ID: %w", err)
	}
	var count int64
	result := s.conn().Model(&models.SharedInfo{}).Where("drive_id = ?", driveIDParsed).Count(&count)
	return count, result.Error
}
