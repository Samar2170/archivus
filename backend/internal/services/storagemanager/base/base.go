package base

import (
	"archivus/internal/models"
	"archivus/internal/store"
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
	inDrive, _, err := b.Store.CheckIfUserInDrive(userID, driveID)
	if err != nil {
		return false, fmt.Errorf("storagemanager: check if user in drive: %w", err)
	}
	return inDrive, nil
}
