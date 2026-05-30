package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Drive struct {
	*gorm.Model
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name      string    `gorm:"not null;index"`
	Slug      string    `gorm:"unique;not null;index"`
	Owner     User      `gorm:"foreignKey:OwnerID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	OwnerID   uuid.UUID `gorm:"type:uuid;not null"`
	Users     []User    `gorm:"many2many:drive_users;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	OwnerType UserType  `gorm:"not null"`

	AbsPath  string `gorm:"index"` // root directory path for local disk
	S3Bucket string `gorm:"index"` // bucket name for S3
}

func (d *Drive) BeforeCreate(tx *gorm.DB) (err error) {
	d.ID = uuid.New()
	return
}

type DirectoryMetadata struct {
	*gorm.Model
	ID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name    string    `gorm:"not null;index"`
	PathKey string    `gorm:"index"` // key for S3, relative path for local disk
	Prefix  string    `gorm:"index"` // virtual directory prefix (trailing slash) for S3

	AbsPath string `gorm:"not null;index"` // path  drive root

	Drive   Drive     `gorm:"foreignKey:DriveID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	DriveID uuid.UUID `gorm:"type:uuid;not null"`

	SizeInMb float64 `gorm:"not null"`
}

func (d *DirectoryMetadata) BeforeCreate(tx *gorm.DB) (err error) {
	d.ID = uuid.New()
	return
}

type FileMetadata struct {
	*gorm.Model
	ID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name    string    `gorm:"not null;index"`
	PathKey string    `gorm:"index"`          // object key for S3, rel path for drive
	Prefix  string    `gorm:"not null;index"` // directory prefix (trailing slash) for S3

	AbsPath string    `gorm:"index"` // absolute path for local disk
	Drive   Drive     `gorm:"foreignKey:DriveID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	DriveID uuid.UUID `gorm:"type:uuid;not null"`

	UploadedBy   User      `gorm:"foreignKey:UploadedByID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	UploadedByID uuid.UUID `gorm:"type:uuid;not null"`

	SizeInMb    float64 `gorm:"not null"`
	ContentType string  `gorm:"not null;default:''"`

	RequireSpecialAccess bool `gorm:"not null;default:false"`

	IsImage                    bool `gorm:"not null;default:false"`
	CompressedVersionAvailable bool `gorm:"not null;default:false"`

	Tags string `gorm:"type:text"`

	Encrypted bool `gorm:"not null;default:false"`

	HasAccess []User `gorm:"many2many:file_access_users;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

func (f *FileMetadata) BeforeCreate(tx *gorm.DB) (err error) {
	f.ID = uuid.New()
	return
}
