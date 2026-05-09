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

func (s *Store) GetUserByUsername(username string) (models.User, error) {
	var user models.User
	result := s.conn().First(&user, "username = ?", username)
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

func (s *Store) GetDriveByOwnerID(ownerID string) (models.Drive, error) {
	var drive models.Drive
	result := s.conn().First(&drive, "owner_id = ?", ownerID)
	return drive, result.Error
}

func (s *Store) GetDriveByID(driveID string) (models.Drive, error) {
	var drive models.Drive
	result := s.conn().First(&drive, "id = ?", driveID)
	return drive, result.Error
}

func (s *Store) CreateUserInvite(invite models.UserInvite) (models.UserInvite, error) {
	result := s.conn().Create(&invite)
	return invite, result.Error
}

func (s *Store) GetUserInviteByCode(inviteCode string) (models.UserInvite, error) {
	var invite models.UserInvite
	result := s.conn().First(&invite, "invite_code = ?", inviteCode)
	return invite, result.Error
}

func (s *Store) AddUserToDrive(userID, driveID string) error {
	userIDParsed, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	driveIDParsed, err := uuid.Parse(driveID)
	if err != nil {
		return fmt.Errorf("invalid drive ID: %w", err)
	}
	drive, err := s.GetDriveByID(driveIDParsed.String())
	if err != nil {
		return fmt.Errorf("drive not found: %w", err)
	}
	user := models.User{ID: userIDParsed}
	return s.conn().Model(&drive).Association("Users").Append(&user)
}
