package dirmanager

import (
	"archivus/internal/config"
	"archivus/internal/models"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrMasterUserPersonalDrive = errors.New("master users cannot have a personal drive")

func drivesRoot() string {
	return config.Config.ArchivusHome
}

func usersRoot(user string) string {
	return filepath.Join(config.Config.ArchivusHome)
}

// CreateDriveDir creates the filesystem directory for a shared drive.
func CreateDriveDir(driveName string) (string, string, error) {
	slug := createSlug(driveName)
	absPath := filepath.Join(drivesRoot(), slug)
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return "", "", fmt.Errorf("dirmanager: create drive dir %q: %w", absPath, err)
	}
	return slug, absPath, nil
}

func DeleteDriveDir(driveName string) error {
	slug := createSlug(driveName)
	absPath := filepath.Join(drivesRoot(), slug)
	if err := os.RemoveAll(absPath); err != nil {
		return fmt.Errorf("dirmanager: delete drive dir %q: %w", absPath, err)
	}
	return nil
}

// CreateUserDriveDir creates a personal drive directory for a non-master user.
func CreateUserDriveDir(user *models.User) error {
	if user.IsMaster {
		return ErrMasterUserPersonalDrive
	}
	// username, parentDrive := user.Username, "drive"
	// path := filepath.Join(usersRoot(username), user.ID.String())
	// if err := os.MkdirAll(path, 0755); err != nil {
	// return fmt.Errorf("dirmanager: create user drive dir %q: %w", path, err)
	// }
	return nil
}
