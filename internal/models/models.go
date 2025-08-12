package models

import (
	"archivus/internal/db"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func init() {
	db.StorageDB.AutoMigrate(&User{}, &FileMetadata{}, &Directory{}, &Tags{}, &UserPreference{})
}

type User struct {
	*gorm.Model
	ID       uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	Username string    `gorm:"uniqueIndex;not null"`
	Password string    `gorm:"not null"`
	Email    string    `gorm:"uniqueIndex;"`
	APIKey   string    `gorm:"uniqueIndex;not null"`
	PIN      string    `gorm:"not null"` // Personal Identification Number for user authentication
}

func GetUserById(id uint) (User, error) {
	var user User
	err := db.StorageDB.Where("id = ?", id).First(&user).Error
	return user, err
}
func GetUserByUsername(username string) (User, error) {
	var user User
	err := db.StorageDB.Where("username = ?", username).First(&user).Error
	return user, err
}

func GetUserByApiKey(apiKey string) (User, error) {
	var user User
	err := db.StorageDB.Where("api_key = ?", apiKey).First(&user).Error
	return user, err
}

type UserPreference struct {
	*gorm.Model
	User           User `gorm:"foreignKey:UserID"`
	UserID         uuid.UUID
	CompressImages bool `gorm:"default:false"`
	AddWebpVersion bool `gorm:"default:false"`
}
