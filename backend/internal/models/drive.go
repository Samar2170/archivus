package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Drive struct {
	*gorm.Model
	ID      uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	Name    string    `gorm:"not null"`
	Slug    string    `gorm:"unique;not null"`
	Owner   User      `gorm:"foreignKey:OwnerID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	OwnerID uuid.UUID `gorm:"type:uuid;not null"`
	Users   []User    `gorm:"many2many:drive_users;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

func (d *Drive) BeforeCreate(tx *gorm.DB) (err error) {
	d.ID = uuid.New()
	return
}
