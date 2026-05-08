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
	tx *gorm.DB
}

func (s *Store) conn() *gorm.DB {
	if s.tx != nil {
		return s.tx
	}
	return s.DB
}

func (s *Store) WithTx(tx *gorm.DB) *Store {
	return &Store{DB: s.DB, tx: tx}
}

func (s *Store) Transaction(fn func(tx *Store) error) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		return fn(s.WithTx(tx))
	})
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
