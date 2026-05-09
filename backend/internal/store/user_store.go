package store

import (
	"archivus/internal/models"
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

func (s *Store) CreateUserInvite(invite models.UserInvite) (models.UserInvite, error) {
	result := s.conn().Create(&invite)
	return invite, result.Error
}

func (s *Store) GetUserInviteByCode(inviteCode string) (models.UserInvite, error) {
	var invite models.UserInvite
	result := s.conn().First(&invite, "invite_code = ?", inviteCode)
	return invite, result.Error
}
