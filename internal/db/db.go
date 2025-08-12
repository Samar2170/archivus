package db

import (
	"archivus/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var StorageDB *gorm.DB

func connectToStorageDB() {
	var err error
	StorageDB, err = gorm.Open(sqlite.Open(config.Config.StorageDbFile), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
}

func init() {
	connectToStorageDB()
}

func InitTestDB() {
	var err error
	StorageDB, err = gorm.Open(sqlite.Open(config.TestDbFile), &gorm.Config{})
	if err != nil {
		panic("failed to connect test database")
	}
}
