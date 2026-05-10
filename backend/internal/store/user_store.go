package store

import (
	"archivus/internal/models"
	"fmt"
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

func (s *Store) GetUserByUsernameOrId(usernameOrId string) (models.User, error) {
	var user models.User
	result := s.conn().First(&user, "username = ? OR id = ?", usernameOrId, usernameOrId)
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

func (s *Store) ResolveUserByUsernameOrId(username, userId string) (models.User, error) {
	if userId != "" {
		return s.GetUserByID(userId)
	}
	if username != "" {
		return s.GetUserByUsername(username)
	}
	return models.User{}, fmt.Errorf("either username or user ID must be provided")
}
