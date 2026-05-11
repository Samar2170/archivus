package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Drive struct {
	*gorm.Model
	ID      uuid.UUID `gorm:"type:uuid;primaryKeyIndex"`
	Name    string    `gorm:"not null;index"`
	Slug    string    `gorm:"unique;not null;index"`
	AbsPath string    `gorm:"not null;index"`
	Owner   User      `gorm:"foreignKey:OwnerID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	OwnerID uuid.UUID `gorm:"type:uuid;not null"`
	Users   []User    `gorm:"many2many:drive_users;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

func (d *Drive) BeforeCreate(tx *gorm.DB) (err error) {
	d.ID = uuid.New()
	return
}

type DirectoryMetadata struct {
	*gorm.Model
	ID      int64     `gorm:"type:uuid;primaryKeyIndex"`
	Name    string    `gorm:"not null;index"`
	AbsPath string    `gorm:"not null;index"`
	RelPath string    `gorm:"not null;index"`
	Drive   Drive     `gorm:"foreignKey:DriveID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	DriveID uuid.UUID `gorm:"type:uuid;not null"`

	SizeInMb float64 `gorm:"not null"`
}

type FileMetadata struct {
	*gorm.Model
	ID      int64     `gorm:"type:uuid;primaryKeyIndex"`
	Name    string    `gorm:"not null;index"`
	AbsPath string    `gorm:"not null;index"`
	RelPath string    `gorm:"not null;index"`
	DirPath string    `gorm:"not null;index"`
	Drive   Drive     `gorm:"foreignKey:DriveID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	DriveID uuid.UUID `gorm:"type:uuid;not null"`

	UploadedBy   User      `gorm:"foreignKey:UploadedByID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	UploadedByID uuid.UUID `gorm:"type:uuid;not null"`

	SizeInMb             float64 `gorm:"not null"`
	RequireSpecialAccess bool    `gorm:"not null;default:false"`

	// implement later
	IsImage                    bool `gorm:"not null;default:false"`
	CompressedVersionAvailable bool `gorm:"not null;default:false"`

	Tags string `gorm:"type:text"`

	Encrypted bool `gorm:"not null;default:false"`

	HasAccess []User `gorm:"many2many:file_access_users;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}
