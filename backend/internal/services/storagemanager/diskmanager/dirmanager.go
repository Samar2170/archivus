package diskmanager

import (
	"archivus/internal/services/storagemanager/base"
	"archivus/internal/store"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type DiskManager struct {
	base.BaseManager
	Home      string
	UsersHome string
}

func GetDiskManager(s *store.Store, home string) *DiskManager {
	return &DiskManager{
		BaseManager: base.BaseManager{Store: s},
		Home:        home,
	}
}

var ErrMasterUserPersonalDrive = errors.New("master users cannot have a personal drive")

// CreateDriveDir creates the filesystem directory for a shared drive.
func (dm *DiskManager) CreateDriveDir(slug string) (string, error) {
	absPath := filepath.Join(dm.Home, slug)
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return "", fmt.Errorf("diskmanager: create drive dir %q: %w", absPath, err)
	}
	return dm.Home, nil
}

func (dm *DiskManager) DeleteDriveDir(slug string) error {
	absPath := filepath.Join(dm.Home, slug)
	if err := os.RemoveAll(absPath); err != nil {
		return fmt.Errorf("diskmanager: delete drive dir %q: %w", absPath, err)
	}
	return nil
}

// CreateUserDriveDir creates a personal drive directory for a non-master user.
// func (dm *DiskManager) CreateUserDriveDir(user *models.User) error {
// 	if user.IsMaster {
// 		return ErrMasterUserPersonalDrive
// 	}
// 	if dm.S3Enabled {
// 		return nil
// 	}
// 	path := filepath.Join(dm.UsersHome, user.Username)
// 	if err := os.MkdirAll(path, 0755); err != nil {
// 		return fmt.Errorf("diskmanager: create user drive dir %q: %w", path, err)
// 	}
// 	return nil
// }

func (dm *DiskManager) CreateDir(subFolder, driveId, userId string) error {
	var dirPath string
	if subFolder == "" {
		return errors.New("subFolder cannot be empty")
	}
	hasAccess, err := dm.CheckUserDriveWriteAccess(userId, driveId)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.New("user does not have write access to this drive")
	}

	drive, err := dm.Store.GetDriveByID(driveId)
	if err != nil {
		return fmt.Errorf("diskmanager: get drive by id %q: %w", driveId, err)
	}

	relPath := filepath.Join(drive.Slug, subFolder)
	dirPath = filepath.Join(dm.Home, relPath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("diskmanager: create dir %q: %w", dirPath, err)
	}
	subFolderSplit := filepath.SplitList(subFolder)
	name := subFolderSplit[len(subFolderSplit)-1]

	_, err = dm.Store.CreateDirectoryMetadata(name, dirPath, relPath, fmt.Sprintf("%d", drive.ID))
	if err != nil {
		err = dm.DeleteDir(relPath, driveId, userId) // cleanup created directory on failure
		if err != nil {
			fmt.Printf("warning: failed to clean up directory after db error: %v\n", err)
		}
		return fmt.Errorf("diskmanager: create directory metadata for dir %q: %w", dirPath, err)
	}
	return nil
}

func (dm *DiskManager) DeleteDir(relPath, driveId, userId string) error {
	dirPath := filepath.Join(dm.Home, relPath)
	hasAccess, err := dm.CheckUserDriveWriteAccess(userId, driveId)
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
