package auth

import (
	"archivus/internal/models"
	"archivus/pkg/utils"
	"fmt"
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
		Email:      user.Email,
		InvitedBy:  user.ID,
		DriveID:    drive.ID,
	}
	if _, err := a.Store.CreateUserInvite(invite); err != nil {
		return "", fmt.Errorf("failed to create user invite: %w", err)
	}
	return inviteCode, nil
}
