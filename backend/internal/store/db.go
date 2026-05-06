package store

import (
	"archivus/internal/config"
	archivus_constants "archivus/internal/constants"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Store struct {
	DB *gorm.DB
}

func getStorageDbFile(dbFile string) string {
	return filepath.Join(config.ProjectBaseDir, dbFile)
}

func (s *Store) Init() error {
	dbFile := getStorageDbFile(archivus_constants.StorageDbFile)
	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		return err
	}
	s.DB = db
	return nil
}

func (s *Store) Migrate(models ...interface{}) error {
	return s.DB.AutoMigrate(models...)
}
