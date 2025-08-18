package service

import (
	"archivus/config"
	"archivus/internal/db"
	"archivus/internal/models"
	"archivus/internal/utils"
	"os"
	"path/filepath"
)

// func MoveFolder(userId, folder, dest string) error {
// 	user, err := models.GetUserById(userId)
// 	if err != nil {
// 		return utils.HandleError("MoveFolder", "Failed to get user by ID", err)
// 	}
// 	folderPath := filepath.Join(config.Config.UploadsDir, folder)
// 	destPath := filepath.Join(config.Config.UploadsDir, dest)
// 	var fmds []models.FileMetadata
// 	err = db.StorageDB.Model(&models.FileMetadata{}).Where("user_id = ? AND file_path LIKE ?", user.ID, folderPath+"%").Find(&fmds).Error
// 	if err != nil {
// 		return utils.HandleError("MoveFolder", "Failed to get file metadata records", err)
// 	}
// }

// func moveDir(src, dest string) error {
// 	err := os.MkdirAll(dest, os.ModePerm)
// 	if err != nil {
// 		return utils.HandleError("MoveFolder", "Failed to create destination directory", err)
// 	}
// 	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
// 		if err != nil {
// 			return utils.HandleError("MoveFolder", "Failed to walk source directory", err)
// 		}
// 		relPath, _ := filepath.Rel(src, path)
// 		targetPath := filepath.Join(dest, relPath)
// 		if info.IsDir() {
// 			return os.MkdirAll(targetPath, os.ModePerm)
// 		}
// 		return copyFile(path, targetPath)
// 	})
// }

// func copyFile(src, dst string) error {
// 	sourceFile, err := os.Open(src)
// 	if err != nil {
// 		return err
// 	}
// 	defer sourceFile.Close()

// 	destFile, err := os.Create(dst)
// 	if err != nil {
// 		return err
// 	}
// 	defer destFile.Close()

// 	_, err = io.Copy(destFile, sourceFile)
// 	return err
// }

func DeleteFolder(userId, folder string) error {
	user, err := models.GetUserById(userId)
	if err != nil {
		return utils.HandleError("DeleteFolder", "Failed to get user by ID", err)
	}
	folderPath := filepath.Join(config.Config.UploadsDir, folder)
	var fmds []models.FileMetadata
	err = db.StorageDB.Model(&models.FileMetadata{}).Where("user_id = ? AND file_path LIKE ?", user.ID, folderPath+"%").Find(&fmds).Error
	if err != nil {
		return utils.HandleError("DeleteFolder", "Failed to get file metadata records", err)
	}
	tx := db.StorageDB.Begin()
	for _, fmd := range fmds {
		err = tx.Delete(&fmd).Error
		if err != nil {
			tx.Rollback()
			return utils.HandleError("DeleteFolder", "Failed to delete file metadata record", err)
		}
	}

	err = os.RemoveAll(folderPath)
	if err != nil {
		tx.Rollback()
		return utils.HandleError("DeleteFolder", "Failed to delete folder from filesystem", err)
	}
	err = tx.Commit().Error
	if err != nil {
		return utils.HandleError("DeleteFolder", "Failed to commit transaction", err)
	}
	return nil
}
