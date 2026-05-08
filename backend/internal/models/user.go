package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	*gorm.Model
	ID       uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	Username string    `gorm:"unique;not null"`
	Password string    `gorm:"not null"`
	PIN      string    `gorm:"not null"`
	Email    string    `gorm:"not null"`

	IsMaster    bool `gorm:"default:false"`
	WriteAccess bool `gorm:"default:false"`
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	u.ID = uuid.New()
	return
}

type UserInvite struct {
	*gorm.Model
	ID         uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	InviteCode string    `gorm:"unique;not null"`
	Email      string    `gorm:"not null"`
	InvitedBy  uuid.UUID `gorm:"type:uuid;not null"`
	Drive      Drive     `gorm:"foreignKey:InvitedBy;constraint:OnDelete:CASCADE;"`
	DriveID    uuid.UUID `gorm:"type:uuid;not null"`
}

func (ui *UserInvite) BeforeCreate(tx *gorm.DB) (err error) {
	ui.ID = uuid.New()
	return
}
