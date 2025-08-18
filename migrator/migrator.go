package migrator

import (
	"archivus/config"
	"archivus/internal/db"
	"archivus/internal/models"
	"archivus/internal/service"
	"archivus/internal/utils"
	"archivus/pkg/logging"
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/gorm"
)

func createUser(userFolder string, tx *gorm.DB) error {
	apiKey, err := utils.GenerateAPIKey(config.ApiKeyLength)
	if err != nil {
		logging.Errorlogger.Error().Msgf("Error generating API key: %v", err)
		fmt.Printf("Error generating %s API key: %s", userFolder, err)
		return err
	}
	password, err := utils.GenerateAPIKey(config.PasswordMinLength)
	if err != nil {
		logging.Errorlogger.Error().Msgf("Error generating password: %v", err)
		fmt.Printf("Error generating %s password: %s", userFolder, err)
		return err
	}
	pin, err := utils.GenerateRandomNumber(config.PinLelength)
	if err != nil {
		logging.Errorlogger.Error().Msgf("Error generating PIN: %v", err)
		fmt.Printf("Error generating %s PIN: %s", userFolder, err)
		return err
	}

	err = tx.Model(&models.User{}).Where("username = ?", userFolder).FirstOrCreate(&models.User{
		Username: userFolder,
		Email:    userFolder + "@placeholder.com",
		PIN:      utils.HashString(pin),
		Password: utils.HashString(password),
		APIKey:   apiKey,
	}).Error
	if err != nil {
		logging.Errorlogger.Error().Msgf("Error creating user for %s: %v", userFolder, err)
		fmt.Printf("Error creating user for %s: %s\n", userFolder, err)
		return err
	} else {
		fmt.Printf("User created for %s with API key: %s\n", userFolder, apiKey)
		return err
	}
}

func Migrate() error {
	service.Setup(false)
	userFolders, err := os.ReadDir(config.Config.UploadsDir)
	if err != nil {
		return err
	}
	for _, userFolder := range userFolders {
		if userFolder.IsDir() {
			txn := db.StorageDB.Begin()
			if err := createUser(userFolder.Name(), txn); err != nil {
				txn.Rollback()
				return fmt.Errorf("failed to create user for %s: %w", userFolder.Name(), err)
			}
			var user models.User
			if err := txn.Where("username = ?", userFolder.Name()).First(&user).Error; err != nil {
				txn.Rollback()
				return fmt.Errorf("failed to find user %s: %w", userFolder.Name(), err)
			}
			filepath.Walk(filepath.Join(config.Config.UploadsDir, userFolder.Name()), func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return fmt.Errorf("error walking through directory %s: %w", path, err)
				}
				if info.IsDir() {
					return nil // Skip directories
				} else {
					// Process files if needed
					// For example, you can log the file name or perform some action
					rel, err := filepath.Rel(config.Config.UploadsDir, path)
					if err != nil {
						return fmt.Errorf("error getting relative path for %s: %w", path, err)
					}
					fmd := models.FileMetadata{
						Name:     info.Name(),
						FilePath: rel,
						UserID:   user.ID,
						SizeInMb: float64(info.Size()) / (1024 * 1024), // Convert bytes to MB
					}
					if err := txn.Create(&fmd).Error; err != nil {
						txn.Rollback()
						return fmt.Errorf("failed to create file metadata for %s: %w", rel, err)
					}
					fmt.Printf("File metadata created for %s: %s\n", userFolder.Name(), rel)
				}
				return nil
			})
			txn.Commit()
			fmt.Printf("Migration completed for user: %s\n", userFolder.Name())
		}
	}
	return nil

}
