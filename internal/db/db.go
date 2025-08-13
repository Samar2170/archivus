package db

import (
	"archivus/config"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var StorageDB *gorm.DB

func InitDB() {
	var err error
	storageDbFile := filepath.Join(config.BaseDir, config.Config.StorageDbFile)
	StorageDB, err = gorm.Open(sqlite.Open(storageDbFile), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
}

func InitTestDB() {
	var err error
	testDbFilePath := filepath.Join(config.BaseDir, config.TestDbFile)
	StorageDB, err = gorm.Open(sqlite.Open(testDbFilePath), &gorm.Config{})
	if err != nil {
		panic("failed to connect test database")
	}
}
