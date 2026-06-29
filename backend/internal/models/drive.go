package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Drive struct {
	*gorm.Model
	ID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name    string    `gorm:"not null;index"`
	Slug    string    `gorm:"unique;not null;index"`
	Owner   User      `gorm:"foreignKey:OwnerID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	OwnerID uuid.UUID `gorm:"type:uuid;not null"`

	ReadUsers    []User `gorm:"many2many:drive_read_users;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	WriteUsers   []User `gorm:"many2many:drive_write_users;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	ManagerUsers []User `gorm:"many2many:drive_manager_users;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	OwnerType UserType `gorm:"not null"`

	Prefix string `gorm:"index"` // archivus directory path for local disk , //  prefix in case of S3

	DefaultWriteAccess bool `gorm:"not null;default:false"`
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
	Prefix  string    `gorm:"index"` // virtual directory prefix (trailing slash) for S3, doesnot include bucketname

	Drive   Drive     `gorm:"foreignKey:DriveID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	DriveID uuid.UUID `gorm:"type:uuid;not null"`

	SizeInMb float64 `gorm:"not null"`

	ParentID *uuid.UUID         `gorm:"type:uuid;index"` // self-referencing foreign key for parent directory
	Parent   *DirectoryMetadata `gorm:"foreignKey:ParentID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	CreatedBy   User      `gorm:"foreignKey:CreatedByID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	CreatedByID uuid.UUID `gorm:"type:uuid"`
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
