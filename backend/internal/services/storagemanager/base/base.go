package base

import (
	"archivus/internal/models"
	storage_types "archivus/internal/services/storagemanager/types"
	"archivus/internal/store"
	"errors"
	"fmt"
)

type BaseManager struct {
	Store *store.Store
}

func (b *BaseManager) CheckUserDriveWriteAccess(userID, driveID string) (bool, error) {
	user, err := b.Store.GetUserByID(userID)
	if err != nil {
		return false, fmt.Errorf("storagemanager: get user %q: %w", userID, err)
	}
	drive, err := b.Store.GetDriveByID(driveID)
	if err != nil {
		return false, fmt.Errorf("storagemanager: get drive %q: %w", driveID, err)
	}
	if drive.OwnerID == user.ID {
		return true, nil
	}
	inDrive, accessLevel, err := b.Store.CheckIfUserInDrive(userID, driveID)
	if err != nil {
		return false, fmt.Errorf("storagemanager: check if user in drive: %w", err)
	}
	if inDrive && models.CompareAccessLevels(accessLevel, models.AccessLevelWrite) {
		return true, nil
	}
	return false, nil
}

func (b *BaseManager) CheckUserHasDriveAccess(userID, driveID string) (bool, error) {
	drive, err := b.Store.GetDriveByID(driveID)
	if err != nil {
		return false, fmt.Errorf("storagemanager: get drive %q: %w", driveID, err)
	}
	if drive.OwnerID.String() == userID {
		return true, nil
	}
	inDrive, _, err := b.Store.CheckIfUserInDrive(userID, driveID)
	if err != nil {
		return false, fmt.Errorf("storagemanager: check if user in drive: %w", err)
	}
	return inDrive, nil
}

// ListRecycleBin returns a drive's recycle bin contents. It is storage-backend
// independent (metadata lives in the DB), so both disk and S3 managers inherit
// it from the base.
func (b *BaseManager) ListRecycleBin(driveId, userId string) ([]storage_types.RecycleEntry, error) {
	hasAccess, err := b.CheckUserHasDriveAccess(userId, driveId)
	if err != nil {
		return nil, err
	}
	if !hasAccess {
		return nil, errors.New("user does not have access to this drive")
	}
	items, err := b.Store.ListRecycleBinItemsByDrive(driveId)
	if err != nil {
		return nil, fmt.Errorf("storagemanager: list recycle bin for drive %q: %w", driveId, err)
	}
	entries := make([]storage_types.RecycleEntry, 0, len(items))
	for _, it := range items {
		entries = append(entries, storage_types.RecycleEntry{
			ID:           it.ID.String(),
			Name:         it.Name,
			Size:         it.SizeInMb,
			ContentType:  it.ContentType,
			OriginalPath: it.OriginalPathKey,
			DeletedAt:    it.CreatedAt,
			ExpiresAt:    it.ExpiresAt,
		})
	}
	return entries, nil
}
