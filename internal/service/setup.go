package service

import (
	"archivus/config"
	"archivus/internal/db"
	"archivus/internal/models"
	"os"
)

func Setup(testEnv bool) {
	if !testEnv {
		db.InitDB()
	}
	// This function is intentionally left empty.
	// It can be used to initialize any global variables or configurations
	// that are needed for the service to run.
	// Currently, it serves as a placeholder for future setup logic.
	db.StorageDB.AutoMigrate(
		&models.User{},
		&models.UserPreference{},
		&models.Tags{},
		&models.FileMetadata{},
		&models.Directory{},
	)
	CheckArchivusDirs()
}

func CheckArchivusDirs() {
	if _, err := os.Stat(config.Config.UploadsDir); err != nil {
		os.Mkdir(config.Config.UploadsDir, os.ModePerm)
	}
	if _, err := os.Stat(config.Config.LogsDir); err != nil {
		os.Mkdir(config.Config.UploadsDir, os.ModePerm)
	}
}
