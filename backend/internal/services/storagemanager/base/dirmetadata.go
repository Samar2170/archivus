package base

import (
	"archivus/internal/models"
	"archivus/internal/store"
	"archivus/internal/utils"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (b *BaseManager) EnsureDirectoryMetadata(userID string, drive models.Drive, segments []string) ([]models.DirectoryMetadata, error) {
	userIDParsed, err := utils.ParseUUID(userID)
	if err != nil {
		return nil, err
	}
	var result []models.DirectoryMetadata
	err = b.Store.Transaction(func(tx *store.Store) error {
		var parentID *uuid.UUID = nil
		prefix := drive.Slug
		for _, seg := range segments {
			seg = strings.Trim(seg, "/")
			if seg == "" {
				continue
			}
			var existing models.DirectoryMetadata
			// parentID == nil must be matched with "IS NULL"; "parent_id = NULL"
			// is never true in SQL and would always create a duplicate root dir.
			query := tx.DB.Where("drive_id = ? AND name = ?", drive.ID, seg)
			if parentID == nil {
				query = query.Where("parent_id IS NULL")
			} else {
				query = query.Where("parent_id = ?", parentID)
			}
			err := query.First(&existing).Error
			switch {
			case err == nil:
				result = append(result, existing)
				parentID = &existing.ID
				prefix = existing.PathKey
			case errors.Is(err, gorm.ErrRecordNotFound):
				newDir := models.DirectoryMetadata{
					Name:        seg,
					DriveID:     drive.ID,
					ParentID:    parentID,
					CreatedByID: userIDParsed,
					Prefix:      prefix,
					PathKey:     prefix + "/" + seg,
				}
				if err := tx.DB.Create(&newDir).Error; err != nil {
					return fmt.Errorf("failed to create directory %q: %w", seg, err)
				}
				result = append(result, newDir)
				parentID = &newDir.ID
				prefix = newDir.PathKey
			default:
				return fmt.Errorf("failed to query directory %q: %w", seg, err)
			}
		}
		return nil
	})
	return result, err
}
