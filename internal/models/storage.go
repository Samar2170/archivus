package models

import (
	"archivus/internal/db"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Tags struct {
	ID  uint   `gorm:"primaryKey"`
	Tag string `gorm:"uniqueIndex"`
}

type FileMetadata struct {
	ID       uint      `gorm:"primaryKey"`
	Name     string    `gorm:"index"`
	FilePath string    `gorm:"index"`
	UserID   uuid.UUID `gorm:"index"`
	User     User
	SizeInMb float64
	IsPublic bool

	Tags []Tags `gorm:"many2many:file_metadata_tags;"`

	IsImage                    bool
	CompressedVersionAvailable bool `gorm:"default:false"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

type Directory struct {
	*gorm.Model
	Name      string
	Path      string
	UserID    uuid.UUID
	User      User
	SizeInMb  float64
	CreatedAt time.Time
	UpdatedAt time.Time

	LastError   string
	HasError    bool `gorm:"default:false"`
	IsMasterDir bool `gorm:"default:false"`
}

func GetOrCreateDir(userId uuid.UUID, name string, isMasterDir bool) Directory {
	var dir Directory
	db.StorageDB.FirstOrCreate(&dir, Directory{UserID: userId, Name: name, IsMasterDir: isMasterDir})
	return dir
}

func GetDirByPathorName(path, name, userId string) (Directory, error) {
	var dir Directory
	err := db.StorageDB.Where("name = ? OR path = ?", name, path).Where("user_id = ? ", userId).Find(&dir).Error
	return dir, err

}
