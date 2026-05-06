package store

import (
	"archivus/internal/config"
	archivus_constants "archivus/internal/constants"
	"archivus/internal/models"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func (s *Store) GetUserByID(userID string) (models.User, error) {
	var user models.User
	result := s.DB.First(&user, "id = ?", userID)
	return user, result.Error
}

func (s *Store) CreateUser(username, password, pin, email string, isMaster bool) (models.User, error) {
	if len(username) < 3 {
		return models.User{}, fmt.Errorf("username must be at least 3 characters long")
	}
	if len(password) < archivus_constants.MinPasswordLength {
		return models.User{}, fmt.Errorf("password must be at least %d characters long", archivus_constants.MinPasswordLength)
	}
	if len(pin) != archivus_constants.PINLength {
		return models.User{}, fmt.Errorf("pin must be exactly %d digits long", archivus_constants.PINLength)
	}

	writeAccess := isMaster || config.Config.DefaultWriteAccess
	user := models.User{
		Username:    username,
		Password:    password,
		PIN:         pin,
		Email:       email,
		IsMaster:    isMaster,
		WriteAccess: writeAccess,
	}
	result := s.DB.Create(&user)
	return user, result.Error
}

func (s *Store) SetupNewDrive(name, userID string) error {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return err
	}
	if !user.IsMaster {
		return fmt.Errorf("only master users can create drives")
	}
	d := models.Drive{
		Name:    name,
		OwnerID: user.ID,
		Slug:    createSlug(name),
	}
	result := s.DB.Create(&d)
	return result.Error
}

func createSlug(name string) string {
	slug := strings.ReplaceAll(name, " ", "-")
	slug = strings.ToLower(slug)
	slug = fmt.Sprintf("%s-%s", slug, uuid.New().String()[:8])
	return slug
}
