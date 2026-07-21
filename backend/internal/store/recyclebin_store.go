package store

import (
	"archivus/internal/models"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateRecycleBinItem records a deleted file in the recycle bin. expiresAt is
// when it becomes eligible for permanent removal by the purge job.
func (s *Store) CreateRecycleBinItem(name, originalPathKey, originalPrefix, recyclePathKey, contentType, thumbnailPath, driveID, deletedByID string, sizeInMb float64, expiresAt time.Time) (models.RecycleBinItem, error) {
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return models.RecycleBinItem{}, fmt.Errorf("invalid drive ID: %w", err)
	}
	deletedByIDParsed, err := uuid.Parse(deletedByID)
	if err != nil {
		return models.RecycleBinItem{}, fmt.Errorf("invalid deleted by ID: %w", err)
	}
	item := models.RecycleBinItem{
		Name:            name,
		OriginalPathKey: originalPathKey,
		OriginalPrefix:  originalPrefix,
		RecyclePathKey:  recyclePathKey,
		ContentType:     contentType,
		ThumbnailPath:   thumbnailPath,
		DriveID:         driveIDParsed,
		DeletedByID:     deletedByIDParsed,
		SizeInMb:        sizeInMb,
		ExpiresAt:       expiresAt,
	}
	result := s.conn().Create(&item)
	return item, result.Error
}

func (s *Store) GetRecycleBinItemByID(id string) (models.RecycleBinItem, error) {
	var item models.RecycleBinItem
	result := s.conn().First(&item, "id = ?", id)
	return item, result.Error
}

// ListRecycleBinItemsByDrive returns the drive's recycle bin contents, most
// recently deleted first.
func (s *Store) ListRecycleBinItemsByDrive(driveID string) ([]models.RecycleBinItem, error) {
	var items []models.RecycleBinItem
	result := s.conn().Where("drive_id = ?", driveID).Order("created_at DESC").Find(&items)
	return items, result.Error
}

// GetExpiredRecycleBinItems returns items whose retention window has elapsed.
func (s *Store) GetExpiredRecycleBinItems(now time.Time) ([]models.RecycleBinItem, error) {
	var items []models.RecycleBinItem
	result := s.conn().Where("expires_at < ?", now).Find(&items)
	return items, result.Error
}

// DeleteRecycleBinItemByID permanently removes a recycle bin row.
func (s *Store) DeleteRecycleBinItemByID(id string) error {
	result := s.conn().Unscoped().Where("id = ?", id).Delete(&models.RecycleBinItem{})
	return result.Error
}
