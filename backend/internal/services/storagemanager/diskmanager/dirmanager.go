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
	dirPath := filepath.Join(dm.Home, relPath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("diskmanager: create dir %q: %w", dirPath, err)
	}
	name := filepath.Base(subFolder)

	_, err = dm.Store.CreateDirectoryMetadata(name, dirPath, relPath, drive.ID.String())
	if err != nil {
		if cleanupErr := os.RemoveAll(dirPath); cleanupErr != nil {
			fmt.Printf("warning: failed to clean up directory after db error: %v\n", cleanupErr)
		}
		return fmt.Errorf("diskmanager: create directory metadata for dir %q: %w", dirPath, err)
	}
	return nil
}

func (dm *DiskManager) DeleteDir(relPath, driveId, userId string) error {
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

	fullRelPath := filepath.Join(drive.Slug, relPath)
	dirPath := filepath.Join(dm.Home, fullRelPath)

	if err := os.RemoveAll(dirPath); err != nil {
		return fmt.Errorf("storagemanager: delete dir %q: %w", dirPath, err)
	}
	if err := dm.Store.DeleteDirectoryMetadataByRelPath(fullRelPath); err != nil {
		return fmt.Errorf("storagemanager: delete directory metadata for dir %q: %w", dirPath, err)
	}
	return nil
}
