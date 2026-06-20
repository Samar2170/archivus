package auth

import (
	"archivus/internal/models"
	"archivus/internal/store"
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

func (a *AuthService) RemoveUserFromDrive(userID, driveID, username, driveSlug string) error {
	var drive models.Drive
	var err error
	var user models.User
	user, err = a.Store.ResolveUserByUsernameOrId(username, userID)
	if err != nil {
		return fmt.Errorf("invalid user: %w", err)
	}
	drive, err = a.Store.ResolveDriveBySlugOrID(driveSlug, driveID)
	if err != nil {
		return fmt.Errorf("invalid drive: %w", err)
	}
	return a.Store.RemoveUserFromDrive(user.ID.String(), drive.ID.String())
}

type UsersInDriveResponse struct {
	Drive models.Drive  `json:"drive"`
	Users []models.User `json:"users"`
}

func (a *AuthService) GetUsersInDrive(userId string) ([]UsersInDriveResponse, error) {
	user, err := a.Store.GetUserByID(userId)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	if !user.IsAdmin {
		return nil, fmt.Errorf("only master users can view users in drive")
	}
	drives, err := a.Store.GetDriveByOwnerID(user.ID.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get drive for user: %w", err)
	}
	if len(drives) == 0 {
		return nil, fmt.Errorf("no drive found for user")
	}
	var driveUserResponses []UsersInDriveResponse
	for _, drive := range drives {
		users, err := a.Store.GetUsersByDriveID(drives[0].ID.String())
		if err != nil {
			return nil, fmt.Errorf("failed to get users in drive: %w", err)
		}
		driveUserResponses = append(driveUserResponses, UsersInDriveResponse{
			Drive: drive,
			Users: users,
		})
	}

	return driveUserResponses, nil
}

type UserInfoResponse struct {
	User   models.User       `json:"user"`
	Drives []store.DriveUser `json:"drives"`
}

func (h *AuthService) GetUserInfo(userID string) (UserInfoResponse, error) {
	user, err := h.Store.GetUserByID(userID)
	if err != nil {
		return UserInfoResponse{}, fmt.Errorf("user not found: %w", err)
	}
	drives, err := h.Store.GetDriveByUserID(user.ID.String())
	if err != nil {
		if err != store.ErrRecordNotFound {
			return UserInfoResponse{}, fmt.Errorf("failed to get drives for user: %w", err)
		}
	}

	ownedDrives, err := h.Store.GetDriveByOwnerID(user.ID.String())
	if err != nil {
		if err != store.ErrRecordNotFound {
			return UserInfoResponse{}, fmt.Errorf("failed to get owned drives for user: %w", err)
		}
	}
	for _, drive := range ownedDrives {
		drives = append(drives, store.DriveUser{
			UserID:      user.ID.String(),
			DriveID:     drive.ID.String(),
			DriveName:   drive.Name,
			AccessLevel: models.AccessLevelOwner,
		})
	}
	return UserInfoResponse{
		User:   user,
		Drives: drives,
	}, nil
}
