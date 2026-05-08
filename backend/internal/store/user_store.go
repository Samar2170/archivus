package store

import (
	"archivus/internal/models"
	"fmt"

	"github.com/google/uuid"
)

func (s *Store) GetUserByID(userID string) (models.User, error) {
	var user models.User
	result := s.conn().First(&user, "id = ?", userID)
	return user, result.Error
}

func (s *Store) CreateUser(user models.User) (models.User, error) {

	result := s.conn().Create(&user)
	return user, result.Error
}

func (s *Store) CreateDrive(name, ownerID, slug, absPath string) (models.Drive, error) {
	id, err := uuid.Parse(ownerID)
	if err != nil {
		return models.Drive{}, fmt.Errorf("invalid owner ID: %w", err)
	}
	drive := models.Drive{
		Name:    name,
		OwnerID: id,
		Slug:    slug,
		AbsPath: absPath,
	}
	result := s.conn().Create(&drive)
	return drive, result.Error
}
