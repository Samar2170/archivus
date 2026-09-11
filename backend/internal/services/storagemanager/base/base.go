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

// CheckUserDriveManageAccess reports whether the user may administer a drive's
// folder shares: admins, the drive owner and drive managers qualify.
func (b *BaseManager) CheckUserDriveManageAccess(userID string, drive models.Drive) (bool, error) {
	user, err := b.Store.GetUserByID(userID)
	if err != nil {
		return false, fmt.Errorf("storagemanager: get user %q: %w", userID, err)
	}
	if user.IsAdmin || drive.OwnerID == user.ID {
		return true, nil
	}
	inDrive, accessLevel, err := b.Store.CheckIfUserInDrive(userID, drive.ID.String())
	if err != nil {
		return false, fmt.Errorf("storagemanager: check if user in drive: %w", err)
	}
	return inDrive && models.CompareAccessLevels(accessLevel, models.AccessLevelManager), nil
}

// CheckUserHasSharedFolderAccess returns the folder share granting userID
// access to rootPathKey in driveID, or an error when there is none. Both read
// and write level shares qualify; write-gated operations enforce their extra
// requirement separately.
func (b *BaseManager) CheckUserHasSharedFolderAccess(userID, driveID, rootPathKey string) (models.SharedInfo, error) {
	share, err := b.Store.GetSharedFolderByScope(driveID, rootPathKey, userID)
	if err != nil {
		return models.SharedInfo{}, fmt.Errorf("no access to this shared folder")
	}
	return share, nil
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
			IsDir:        it.IsDir,
			Size:         it.SizeInMb,
			ContentType:  it.ContentType,
			OriginalPath: it.OriginalPathKey,
			DeletedAt:    it.CreatedAt,
			ExpiresAt:    it.ExpiresAt,
		})
	}
	return entries, nil
}
