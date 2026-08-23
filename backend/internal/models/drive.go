package models

import (
	"path/filepath"
	"strings"
	"time"

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

	ThumbnailPath string `gorm:"not null;default:''"` // path to thumbnail image for the file, should contain path after thumbnail dir

	Tags string `gorm:"type:text"`

	Encrypted bool `gorm:"not null;default:false"`

	HasAccess []User `gorm:"many2many:file_access_users;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`

	Extension string `gorm:"not null;default:''"` // file extension, for search and filtering

	// UploadStatus tracks whether the file's bytes have actually landed in the
	// storage backend. Direct uploads are written synchronously and are born
	// "ready"; large chunked uploads finish assembly quickly but defer the slow
	// push to object storage to a background worker, so they start "pending",
	// move to "uploading" while a worker owns them, and become "ready" once the
	// bytes are persisted (or "failed" after too many attempts).
	UploadStatus string `gorm:"not null;default:'ready';index"`
	// PendingSourcePath is the local assembled file awaiting upload, meaningful
	// only while UploadStatus is pending/uploading. Cleared once the file is ready.
	PendingSourcePath string `gorm:"not null;default:''"`
	// UploadAttempts counts how many times a worker has tried to finalize this
	// upload, so a persistently failing file eventually stops being retried.
	UploadAttempts int `gorm:"not null;default:0"`
}

// UploadStatus values for FileMetadata.UploadStatus.
const (
	UploadStatusReady     = "ready"
	UploadStatusPending   = "pending"
	UploadStatusUploading = "uploading"
	UploadStatusFailed    = "failed"

	// MaxUploadAttempts is how many times a pending upload is retried by the
	// worker before it is marked failed.
	MaxUploadAttempts = 5
)

func (f *FileMetadata) BeforeCreate(tx *gorm.DB) (err error) {
	f.ID = uuid.New()
	f.Extension = SplitExtension(f.Name)
	f.IsImage = IsImageName(f.Name)
	return
}

// ImageVideoExtensions are the (dot-less, lowercase) extensions treated as
// image/video files and therefore flagged for thumbnail generation. Shared
// between the upload path (which sets IsImage at creation time) and the oculus
// backfill (which repairs legacy rows).
var ImageVideoExtensions = map[string]bool{
	"jpg":  true,
	"jpeg": true,
	"png":  true,
	"gif":  true,
	"webp": true,
	"bmp":  true,
	"tiff": true,
	"svg":  true,
	"heic": true,

	"mp4":  true,
	"mov":  true,
	"mkv":  true,
	"avi":  true,
	"webm": true,
	"m4v":  true,
}

// SplitExtension returns the lowercase extension of name without the leading
// dot, e.g. "photo.JPG" -> "jpg". Returns "" when name has no extension.
func SplitExtension(name string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
}

// IsImageName reports whether name has an image/video extension.
func IsImageName(name string) bool {
	return ImageVideoExtensions[SplitExtension(name)]
}

// RecycleBinItem records a file that was deleted but not yet permanently
// removed. It holds enough of the original file's metadata to restore it to its
// former location, and a pointer (RecyclePathKey) to where the bytes now live
// inside the recycle bin. The purge job deletes items whose ExpiresAt is in the
// past.
type RecycleBinItem struct {
	*gorm.Model
	ID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Name string    `gorm:"not null;index"`

	// Original location, used to restore the file.
	OriginalPathKey string `gorm:"index"` // object key (S3) or absolute path (disk)
	OriginalPrefix  string `gorm:"index"` // parent directory key/path

	// RecyclePathKey is where the bytes currently live inside the recycle bin.
	RecyclePathKey string `gorm:"not null"`

	Drive   Drive     `gorm:"foreignKey:DriveID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	DriveID uuid.UUID `gorm:"type:uuid;not null"`

	DeletedBy   User      `gorm:"foreignKey:DeletedByID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	DeletedByID uuid.UUID `gorm:"type:uuid"`

	SizeInMb      float64 `gorm:"not null"`
	ContentType   string  `gorm:"not null;default:''"`
	ThumbnailPath string  `gorm:"not null;default:''"` // preserved so purge can clean it up

	// ExpiresAt is when the item becomes eligible for permanent deletion.
	ExpiresAt time.Time `gorm:"index"`
}

func (r *RecycleBinItem) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return
}
