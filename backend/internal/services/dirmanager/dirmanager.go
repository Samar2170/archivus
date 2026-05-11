package dirmanager

import (
	"archivus/internal/models"
	"archivus/internal/store"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type DirManager struct {
	Home      string
	UsersHome string
	Store     *store.Store
}

func GetDirManager(store *store.Store, home string) *DirManager {
	return &DirManager{
		Home:  home,
		Store: store,
	}
}

var ErrMasterUserPersonalDrive = errors.New("master users cannot have a personal drive")

// CreateDriveDir creates the filesystem directory for a shared drive.
func (dm *DirManager) CreateDriveDir(driveName string) (string, string, error) {
	slug := createSlug(driveName)
	absPath := filepath.Join(dm.Home, slug)
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return "", "", fmt.Errorf("dirmanager: create drive dir %q: %w", absPath, err)
	}
	return slug, absPath, nil
}

func (dm *DirManager) DeleteDriveDir(driveName string) error {
	slug := createSlug(driveName)
	absPath := filepath.Join(dm.Home, slug)
	if err := os.RemoveAll(absPath); err != nil {
		return fmt.Errorf("dirmanager: delete drive dir %q: %w", absPath, err)
	}
	return nil
}

// CreateUserDriveDir creates a personal drive directory for a non-master user.
func (dm *DirManager) CreateUserDriveDir(user *models.User) error {
	if user.IsMaster {
		return ErrMasterUserPersonalDrive
	}
	path := filepath.Join(dm.UsersHome, user.Username)
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("dirmanager: create user drive dir %q: %w", path, err)
	}
	return nil
}

func (dm *DirManager) CreateDir(subFolder string, userId string) error {
	var dirPath string
	if subFolder == "" {
		return errors.New("subFolder cannot be empty")
	}

	dirPath = filepath.Join(dm.Home, subFolder)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("dirmanager: create dir %q: %w", dirPath, err)
	}
	return nil
}
