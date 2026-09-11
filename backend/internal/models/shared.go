package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SharedInfo grants one Archivus user access to one folder (and its whole
// subtree) inside a drive. Shares are separate from drive membership: a user
// can hold a share on a folder without being a drive member, and drive members
// do not automatically see every folder share.
//
// RootPathKey is the drive-relative path of the shared folder (slash separated,
// no leading/trailing slash, e.g. "documents/reports"), normalized by
// utils.NormalizeRelPath. Storing it drive-relative keeps shares portable
// across storage backends and immune to storage-root moves; each backend maps
// it onto its own key format when serving shared requests.
type SharedInfo struct {
	*gorm.Model
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	DriveID     uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:idx_shared_info_scope"`
	RootPathKey string    `gorm:"not null;index;uniqueIndex:idx_shared_info_scope"`

	// SharedWithUserID is the user the folder is shared with. Shares may only
	// target existing Archivus users.
	SharedWithUserID uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:idx_shared_info_scope"`

	// AccessLevel is read or write. write is reserved for phase 2 mutations;
	// both levels allow listing and downloads.
	AccessLevel AccessLevel `gorm:"not null"`

	GrantedBy       User      `gorm:"foreignKey:GrantedByUserID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	GrantedByUserID uuid.UUID `gorm:"type:uuid;not null"`

	SharedWith User  `gorm:"foreignKey:SharedWithUserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Drive      Drive `gorm:"foreignKey:DriveID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

func (si *SharedInfo) BeforeCreate(tx *gorm.DB) (err error) {
	si.ID = uuid.New()
	return
}
