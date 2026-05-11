package storagemanager

import (
	"archivus/internal/models"
	"archivus/internal/store"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type StorageManager struct {
	Home      string
	UsersHome string
	Store     *store.Store
}

func GetStorageManager(store *store.Store, home string) *StorageManager {
	return &StorageManager{
		Home:  home,
		Store: store,
	}
}

var ErrMasterUserPersonalDrive = errors.New("master users cannot have a personal drive")

// CreateDriveDir creates the filesystem directory for a shared drive.
func (dm *StorageManager) CreateDriveDir(driveName string) (string, string, error) {
	slug := createSlug(driveName)
	absPath := filepath.Join(dm.Home, slug)
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return "", "", fmt.Errorf("storagemanager: create drive dir %q: %w", absPath, err)
	}
	return slug, absPath, nil
}

func (dm *StorageManager) DeleteDriveDir(driveName string) error {
	slug := createSlug(driveName)
	absPath := filepath.Join(dm.Home, slug)
	if err := os.RemoveAll(absPath); err != nil {
		return fmt.Errorf("storagemanager: delete drive dir %q: %w", absPath, err)
	}
	return nil
}

// CreateUserDriveDir creates a personal drive directory for a non-master user.
func (dm *StorageManager) CreateUserDriveDir(user *models.User) error {
	if user.IsMaster {
		return ErrMasterUserPersonalDrive
	}
	path := filepath.Join(dm.UsersHome, user.Username)
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("storagemanager: create user drive dir %q: %w", path, err)
	}
	return nil
}

func (dm *StorageManager) checkUserDriveWriteAccess(userID, driveID string) (bool, error) {
	user, err := dm.Store.GetUserByID(userID)
	if err != nil {
		return false, fmt.Errorf("storagemanager: get user by id %q: %w", userID, err)
	}
	if !user.WriteAccess {
		return false, nil
	}
	inDrive, err := dm.Store.CheckIfUserInDrive(userID, driveID)
	if err != nil {
		return false, fmt.Errorf("storagemanager: check if user in drive: %w", err)
	}
	return inDrive, nil
}

func (dm *StorageManager) checkUserHasDriveAccess(userID, driveID string) (bool, error) {
	inDrive, err := dm.Store.CheckIfUserInDrive(userID, driveID)
	if err != nil {
		return false, fmt.Errorf("storagemanager: check if user in drive: %w", err)
	}
	return inDrive, nil
}

func (dm *StorageManager) CreateDir(subFolder, driveId, userId string) error {
	var dirPath string
	if subFolder == "" {
		return errors.New("subFolder cannot be empty")
	}
	hasAccess, err := dm.checkUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}

	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("storagemanager: get drive by id %q: %w", driveId, err)
	}

	relPath := filepath.Join(drive.Slug, subFolder)
	dirPath = filepath.Join(dm.Home, relPath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("storagemanager: create dir %q: %w", dirPath, err)
	}
	subFolderSplit := filepath.SplitList(subFolder)
	name := subFolderSplit[len(subFolderSplit)-1]

	_, err = dm.Store.CreateDirectoryMetadata(name, dirPath, relPath, fmt.Sprintf("%d", drive.ID))
	if err != nil {
		err = dm.DeleteDir(relPath, driveId, userId) // cleanup created directory on failure
		if err != nil {
			fmt.Printf("warning: failed to clean up directory after db error: %v\n", err)
		}
		return fmt.Errorf("storagemanager: create directory metadata for dir %q: %w", dirPath, err)
	}
	return nil
}

func (dm *StorageManager) DeleteDir(relPath, driveId, userId string) error {
	dirPath := filepath.Join(dm.Home, relPath)
	hasAccess, err := dm.checkUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}

	if err := os.RemoveAll(dirPath); err != nil {
		return fmt.Errorf("storagemanager: delete dir %q: %w", dirPath, err)
	}
	if err := dm.Store.DeleteDirectoryMetadataByRelPath(relPath); err != nil {
		return fmt.Errorf("storagemanager: delete directory metadata for dir %q: %w", dirPath, err)
	}
	return nil
}
