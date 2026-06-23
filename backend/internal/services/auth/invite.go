package auth

import (
	"archivus/internal/models"
	"archivus/pkg/utils"
	"context"
	"fmt"
	"time"
)

func (a *AuthService) InviteUser(invitor models.User, driveId string, accessLevel models.AccessLevel) (string, error) {
	if !invitor.IsAdmin {
		return "", fmt.Errorf("only master users can invite other users")
	}
	drive, err := a.Store.GetDriveByID(driveId)
	if err != nil {
		return "", fmt.Errorf("failed to get drive: %w", err)
	}
	inviteCode, err := utils.GenerateRandomAlphaNumericString(16)
	if err != nil {
		return "", fmt.Errorf("failed to generate invite code: %w", err)
	}
	invite := models.UserInvite{
		InviteCode:  inviteCode,
		InvitedBy:   invitor.ID,
		DriveID:     drive.ID,
		ExpiresAt:   time.Now().Add(24 * 3 * time.Hour),
		AccessLevel: accessLevel,
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

func (a *AuthService) AddUserToDrive(userID, driveID, username, driveSlug string, accessLevel models.AccessLevel) error {
	var drive models.Drive
	var user models.User
	var err error
	if accessLevel == "" {
		accessLevel = models.AccessLevelRead
	}
	user, err = a.Store.ResolveUserByUsernameOrId(username, userID)
	if err != nil {
		return fmt.Errorf("invalid user: %w", err)
	}
	drive, err = a.Store.ResolveDriveBySlugOrID(driveSlug, driveID)
	if err != nil {
		return fmt.Errorf("invalid drive: %w", err)
	}
	return a.Store.AddUserToDrive(context.Background(), user.ID.String(), drive.ID.String(), accessLevel)
}

func (a *AuthService) RemoveUserFromDrive(removeUserID, driveID, username, driveSlug, userId string) error {
	var drive models.Drive
	var err error
	drive, err = a.Store.ResolveDriveBySlugOrID(driveSlug, driveID)
	if err != nil {
		return fmt.Errorf("invalid drive: %w", err)
	}
	inDrive, accessLevel, err := a.Store.CheckIfUserInDrive(userId, driveID)
	if err != nil {
		return fmt.Errorf("failed to check if user is in drive: %w", err)
	}
	if !inDrive && drive.OwnerID.String() != userId {
		return fmt.Errorf("user not in drive")
	}
	removeUserInDrive, removeUserAccessLevel, err := a.Store.CheckIfUserInDrive(removeUserID, driveID)
	if err != nil {
		return fmt.Errorf("failed to check if user is in drive: %w", err)
	}
	if !removeUserInDrive {
		return fmt.Errorf("user not in drive")
	}
	if removeUserID == userId {
		return fmt.Errorf("cannot remove self")
	}
	isOwner := drive.OwnerID.String() == userId
	if removeUserAccessLevel == models.AccessLevelManager {
		if accessLevel != models.AccessLevelOwner && !isOwner {
			return fmt.Errorf("only owner can remove manager")
		}
	}
	if accessLevel != models.AccessLevelManager && accessLevel != models.AccessLevelOwner && !isOwner {
		return fmt.Errorf("only drive managers can remove users")
	}
	return a.Store.RemoveUserFromDrive(removeUserID, driveID)
}
