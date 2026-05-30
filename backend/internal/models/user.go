package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserType string

const (
	UserTypePersonal UserType = "personal"
	UserTypeBusiness UserType = "business"
)

type User struct {
	*gorm.Model
	ID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	Username string    `gorm:"unique;not null"`
	Password string    `gorm:"not null"`
	PIN      string    `gorm:"not null"`
	Email    string    `gorm:"not null"`

	IsAdmin     bool     `gorm:"default:false"`
	WriteAccess bool     `gorm:"default:false"`
	Type        UserType `gorm:"not null"`
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	u.ID = uuid.New()
	return
}

type AccessLevel string

const (
	AccessLevelRead    AccessLevel = "read"
	AccessLevelWrite   AccessLevel = "write"
	AccessLevelManager AccessLevel = "manager"
)

type UserInvite struct {
	*gorm.Model
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	InviteCode string    `gorm:"unique;not null"`
	InvitedBy  uuid.UUID `gorm:"type:uuid;not null"`
	Drive      Drive     `gorm:"foreignKey:InvitedBy;constraint:OnDelete:CASCADE;"`
	DriveID    uuid.UUID `gorm:"type:uuid;not null"`

	AccessLevel AccessLevel `gorm:"not null"`
	ExpiresAt   time.Time   `gorm:"not null"`
}

func (ui *UserInvite) BeforeCreate(tx *gorm.DB) (err error) {
	ui.ID = uuid.New()
	return
}
