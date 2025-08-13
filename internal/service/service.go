package service

import (
	"archivus/config"
	"archivus/internal/auth"
	"archivus/internal/db"
	"archivus/internal/models"
	"archivus/internal/utils"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

func GetTags(tagString string, tx *gorm.DB) ([]models.Tags, error) {
	var err error
	tagsSplit := strings.Split(tagString, ",")
	var tagsArray []models.Tags
	for _, tag := range tagsSplit {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		var tagModel models.Tags
		// Check if tag already exists
		err = tx.Where("tag = ?", tag).First(&tagModel).Error
		if err != nil {
			if err.Error() == "record not found" {
				tagModel = models.Tags{Tag: tag}
				err = tx.Create(&tagModel).Error
				if err != nil {
					tx.Rollback()
					return tagsArray, utils.HandleError("GetTags", "Failed to create tag", err)
				}
			}
		}
		tagsArray = append(tagsArray, tagModel)
	}
	if len(tagsArray) == 0 {
		tagsArray = nil // No tags provided, set to nil
	}
	return tagsArray, nil
}

func getAbsoluteFilePath(fileName, username, folderPath string) (string, string, error) {
	filenameCleand := strings.Replace(fileName, " ", "_", -1)
	pathFromUploadsDir := filepath.Join(username, folderPath, filenameCleand)
	filePath := filepath.Join(config.Config.UploadsDir, pathFromUploadsDir)
	return filePath, pathFromUploadsDir, nil
}

func getNewFileName(fileName, username, folderPath string) (string, string, error) {
	absFilePath, pathFromUploadsDir, err := getAbsoluteFilePath(fileName, username, folderPath)
	if err != nil {
		return "", pathFromUploadsDir, utils.HandleError("getNewFileName", "Failed to get absolute file path", err)
	}
	if _, err := os.Stat(absFilePath); os.IsNotExist(err) {
		return absFilePath, pathFromUploadsDir, nil
	} else {
		// File already exists, generate a new name
		ext := filepath.Ext(fileName)
		baseName := strings.TrimSuffix(fileName, ext)
		newFileName := fmt.Sprintf("%s_%d%s", baseName, time.Now().Unix(), ext)
		return getAbsoluteFilePath(newFileName, username, folderPath)
	}
}

func SaveFile(file multipart.File, fileHeader *multipart.FileHeader, username, folderPath, tags string) error {
	filePath, pathFromUploadsDir, err := getNewFileName(fileHeader.Filename, username, folderPath)
	if err != nil {
		return utils.HandleError("SaveFile", "Failed to get new file name", err)
	}
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return utils.HandleError("SaveFile", "Failed to open file for writing", err)
	}
	defer f.Close()
	tx := db.StorageDB.Begin()
	user, err := models.GetUserByUsername(username)
	if err != nil {
		tx.Rollback()
		return utils.HandleError("SaveFile", "Failed to get user by username", err)
	}
	tagsArray, err := GetTags(tags, tx)
	if err != nil {
		tx.Rollback()
		return utils.HandleError("SaveFile", "Failed to get tags", err)
	}
	fmd := models.FileMetadata{
		Name:     fileHeader.Filename,
		FilePath: pathFromUploadsDir,
		UserID:   user.ID,
		SizeInMb: float64(fileHeader.Size) / 1024 / 1024,
	}
	fmd.Tags = tagsArray
	err = tx.Create(&fmd).Error
	if err != nil {
		tx.Rollback()
		return utils.HandleError("SaveFile", "Failed to create file metadata record", err)
	}
	reader := bufio.NewReader(file)
	writer := io.Writer(f)
	_, err = io.Copy(writer, reader)
	if err != nil {
		tx.Rollback()
		return utils.HandleError("SaveFile", "Failed to copy file content", err)
	}
	tx.Commit()
	return nil
}

func GetSignedUrl(filePath string, userId string) (string, error) {
	var fmd models.FileMetadata
	db.StorageDB.Where("file_path = ? AND user_id = ?", filePath, userId).First(&fmd)

	expiresAt := time.Now().Add(600 * time.Minute).Unix()
	expriresAtStr := fmt.Sprintf("%d", expiresAt)
	content := fmt.Sprintf("%s%s%s", config.Config.SecretKey, expriresAtStr, filePath)

	signature := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%s?signature=%s&expires_at=%d", filePath, hex.EncodeToString(signature[:]), expiresAt), nil
}

func GetFiles(userId string) ([]models.FileMetadata, error) {
	var files []models.FileMetadata
	err := db.StorageDB.Where("user_id = ?", userId).Find(&files).Error
	return files, utils.HandleError("GetFiles", "Failed to get files for user", err)
}

func DownloadFile(filePath, signature, expiresAt string, compressed bool) ([]byte, error) {
	var absPath string
	content := fmt.Sprintf("%s%s%s", config.Config.SecretKey, expiresAt, filePath)
	hash := sha256.Sum256([]byte(content))
	if hex.EncodeToString(hash[:]) != signature {
		return nil, utils.HandleError("DownloadFile", "Invalid signature", errors.New("invalid signature"))
	}
	// var absPath string
	// if compressed {
	// 	absPath = filepath.Join(config.Config.UploadsDir, image.GetCompressedPath(filePath))
	// }
	// f, err := os.ReadFile(absPath)
	// if err != nil {
	absPath = filepath.Join(config.Config.UploadsDir, filePath)
	f, err := os.ReadFile(absPath)
	if err != nil {
		return nil, utils.HandleError("DownloadFile", "Failed to read file", err)
	}
	return f, nil
}

type DirEntry struct {
	Name      string
	IsDir     bool
	Extension string
	SignedUrl string
	Size      float64
	Path      string

	NavigationPath string
}

func GetSizeForDirEntry(file fs.DirEntry) float64 {
	fi, err := file.Info()
	if err != nil {
		return 0
	}
	return float64(fi.Size() / 1024 / 1024)
}

func FindFiles(apiKey string, folder string) ([]DirEntry, float64, error) {
	user, err := auth.GetUserByApiKey(apiKey)
	if err != nil {
		return nil, 0, utils.HandleError("FindFiles", "Failed to get user by API key", err)
	}

	pathFromUploadsDir := filepath.Join(user.Username, folder)
	folderPath := filepath.Join(config.Config.UploadsDir, pathFromUploadsDir)
	files, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, 0, utils.HandleError("FindFiles", "Failed to read directory", err)
	}

	var entries []DirEntry
	var backendAddr string

	if config.Config.Mode == "net" {
		currentIp, err := utils.GetPrivateIp()
		if err != nil {
			return nil, 0, utils.HandleError("FindFiles", "Failed to get current IP address", err)
		}
		backendAddr = fmt.Sprintf("%s://%s:%s", config.GetBackendScheme(), currentIp, config.Config.BackendConfig.Port)
	} else {
		backendAddr = fmt.Sprintf("%s://%s", config.GetBackendScheme(), config.GetBackendAddr())
	}
	for _, file := range files {
		signedUrl, err := GetSignedUrl(pathFromUploadsDir+"/"+file.Name(), user.Username)
		if err != nil {
			signedUrl = ""
		}
		entries = append(entries, DirEntry{
			Name:      file.Name(),
			Path:      folder + "/" + file.Name(),
			IsDir:     file.IsDir(),
			Extension: filepath.Ext(file.Name()),
			SignedUrl: backendAddr + "/files/download/" + signedUrl,
			Size:      GetSizeForDirEntry(file),
		})
	}
	var folderSize float64
	folderData, err := models.GetDirByPathorName(pathFromUploadsDir, folder, user.Username)
	if err == nil {
		folderSize = folderData.SizeInMb
	}
	return entries, folderSize, nil
}

type FolderEntry struct {
	Name string
	Path string
}

func splitPathTillUserDir(path string, username string) string {
	split := strings.Split(path, "/")
	for i := len(split) - 1; i >= 0; i-- {
		if split[i] == username {
			return filepath.Join(split[i+1:]...)
		}
	}
	return path
}

func GetAllFolders(apiKey string) ([]FolderEntry, error) {
	user, err := auth.GetUserByApiKey(apiKey)
	if err != nil {
		return nil, utils.HandleError("GetAllFolders", "Failed to get user by API key", err)
	}
	pathFromUploadsDir := filepath.Join(user.Username)
	folderPath := filepath.Join(config.Config.UploadsDir, pathFromUploadsDir)
	var subDirs []FolderEntry
	err = filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return utils.HandleError("GetAllFolders", "Failed to walk directory", err)
		}
		if info.IsDir() && path != folderPath {
			subDirs = append(subDirs, FolderEntry{
				Name: info.Name(),
				Path: splitPathTillUserDir(path, user.Username),
			})
		}
		return nil
	})
	if err != nil {
		return nil, utils.HandleError("GetAllFolders", "Failed to walk directory", err)
	}

	return subDirs, nil
}
