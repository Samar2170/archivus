package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ApiKey is a long-lived credential a user can hand to a CLI or script
// instead of their password. Only the hash of the key is stored; the
// plaintext value is returned once, at creation time.
type ApiKey struct {
	*gorm.Model
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID  `gorm:"type:uuid;not null"`
	Name        string     `gorm:"not null"`
	KeyHash     string     `gorm:"unique;not null"`
	AccessLevel AccessLevel `gorm:"not null"`
	ExpiresAt   time.Time  `gorm:"not null"`
}

func (ak *ApiKey) BeforeCreate(tx *gorm.DB) (err error) {
	ak.ID = uuid.New()
	return
}
