package auth

import (
	"archivus/internal/models"
	"archivus/pkg/utils"
	"fmt"
	"time"
)

func (a *AuthService) InviteUser(user models.User) (string, error) {
	if !user.IsMaster {
		return "", fmt.Errorf("only master users can invite other users")
	}
	drive, err := a.Store.GetDriveByOwnerID(user.ID.String())
	if err != nil {
		return "", fmt.Errorf("failed to get drive for user: %w", err)
	}
	inviteCode, err := utils.GenerateRandomAlphaNumericString(16)
	if err != nil {
		return "", fmt.Errorf("failed to generate invite code: %w", err)
	}
	invite := models.UserInvite{
		InviteCode: inviteCode,
		InvitedBy:  user.ID,
		DriveID:    drive.ID,
		ExpiresAt:  time.Now().Add(24 * 3 * time.Hour),
	}
	if _, err := a.Store.CreateUserInvite(invite); err != nil {
		return "", fmt.Errorf("failed to create user invite: %w", err)
	}
	return inviteCode, nil
}

func (a *AuthService) ValidateInviteCode(inviteCode string) (models.UserInvite, error) {
	invite, err := a.Store.GetUserInviteByCode(inviteCode)
	if err != nil {
		return models.UserInvite{}, fmt.Errorf("invalid invite code: %w", err)
	}
	if invite.ExpiresAt.Before(time.Now()) {
		return models.UserInvite{}, fmt.Errorf("invite code has expired")
	}
	return invite, nil
}

func (a *AuthService) AddUserToDrive(userID, driveID string) error {

	return a.Store.AddUserToDrive(userID, driveID)
}

func (a *AuthService) RemoveUserFromDrive(userID, driveID, username, driveSlug string) error {
	var drive models.Drive
	var err error
	if driveID == "" && driveSlug == "" {
		return fmt.Errorf("either drive ID or slug must be provided")
	}
	if userID == "" && username == "" {
		return fmt.Errorf("either user ID or username must be provided")
	}
	if driveID != "" {
		drive, err = a.Store.GetDriveByID(driveID)
	} else {
		drive, err = a.Store.GetDriveBySlug(driveSlug)
	}
	if err != nil {
		return fmt.Errorf("invalid drive ID or slug: %w", err)
	}
	var user models.User
	if userID != "" {
		user, err = a.Store.GetUserByID(userID)
	} else {
		user, err = a.Store.GetUserByUsername(username)
	}
	if err != nil {
		return fmt.Errorf("invalid user ID or username: %w", err)
	}
	return a.Store.RemoveUserFromDrive(user.ID.String(), drive.ID.String())
}
