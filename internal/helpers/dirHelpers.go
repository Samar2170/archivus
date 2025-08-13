package helpers

import (
	"archivus/config"
	"archivus/internal/db"
	"archivus/internal/models"
	"archivus/pkg/logging"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func createFolder(dirPath string) error {
	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dirPath, 0755)
		if err != nil {
			return err
		}
	} else if info.IsDir() {
		return nil
	} else {
		return err
	}
	return nil
}

func CreateFolder(username, subFolder string) error {
	var dirPath string
	if subFolder != "" {
		dirPath = filepath.Join(config.Config.UploadsDir, username, subFolder)
	} else {
		dirPath = filepath.Join(config.Config.UploadsDir, username)
	}
	return createFolder(dirPath)
}

func GetFolderSize(folderPath string) (int64, error) {
	var totalSize int64

	err := filepath.WalkDir(folderPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				totalSize += info.Size()
			}
		}
		return nil
	})

	return totalSize, err
}

func UpdateDirsData() error {
	uploadsDir := config.Config.UploadsDir
	fs, err := os.ReadDir(uploadsDir)
	if err != nil {
		return err
	}
	for _, f := range fs {
		if f.IsDir() {
			user, err := models.GetUserByUsername(f.Name())
			if err != nil {
				return fmt.Errorf("error for %s -> %s", err.Error(), user.Username)
			}
			dir := models.GetOrCreateDir(user.ID, f.Name(), true)
			size, err := GetFolderSize(filepath.Join(uploadsDir, f.Name()))
			if err != nil {
				dir.HasError = true
				dir.LastError = err.Error()
			} else {
				dir.SizeInMb = float64(size) / 1024 / 1024
			}
			db.StorageDB.Save(&dir)
		}
	}
	return nil
}
func UpdateUserDirsData() {
	var users []models.User
	db.StorageDB.Find(&users)

	for _, user := range users {
		SubDirsData(user.Username)
	}
}

func cleanPathForSubDir(path, username string) string {
	split := strings.Split(path, "/")
	var ui int
	for i, p := range split {
		if p == username {
			ui = i
		}
	}
	return filepath.Join(split[ui:]...)
}
func SubDirsData(username string) error {
	userDir := filepath.Join(config.Config.UploadsDir, username)
	user, err := models.GetUserByUsername(username)
	if err != nil {
		return err
	}
	err = filepath.WalkDir(userDir, func(path string, d os.DirEntry, err error) error {
		logging.AuditLogger.Println(path)
		if d.IsDir() {
			dir := models.GetOrCreateDir(user.ID, d.Name(), false)
			size, err := GetFolderSize(path)
			dir.Path = cleanPathForSubDir(path, username)
			if err != nil {
				dir.HasError = true
				dir.LastError = err.Error()
				return err
			} else {
				dir.SizeInMb = float64(size) / 1024 / 1024
			}
			db.StorageDB.Save(&dir)
		}
		return nil
	})
	return err
}
