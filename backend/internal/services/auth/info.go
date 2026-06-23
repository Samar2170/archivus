package auth

import (
	"archivus/internal/models"
	"archivus/internal/store"
	"fmt"
)

type DriveInfoResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	OwnerID   string `json:"ownerId"`
	OwnerName string `json:"ownerName"`

	ReadUsers    []models.User `json:"readUsers"`
	WriteUsers   []models.User `json:"writeUsers"`
	ManagerUsers []models.User `json:"managerUsers"`
}

func (a *AuthService) GetDriveInfo(userId string, driveId string) (DriveInfoResponse, error) {
	user, err := a.Store.GetUserByID(userId)
	if err != nil {
		return DriveInfoResponse{}, err
	}
	if !user.IsAdmin {
		return DriveInfoResponse{}, nil
	}
	drive, err := a.Store.GetDriveByID(driveId)
	if err != nil {
		return DriveInfoResponse{}, err
	}
	users, err := a.Store.GetUsersByDriveID(drive.ID.String())
	if err != nil {
		return DriveInfoResponse{}, err
	}
	driveInfo := DriveInfoResponse{
		ID:           drive.ID.String(),
		Name:         drive.Name,
		Slug:         drive.Slug,
		OwnerID:      drive.OwnerID.String(),
		OwnerName:    drive.Owner.Username,
		ReadUsers:    users["read"],
		WriteUsers:   users["write"],
		ManagerUsers: users["manager"],
	}

	return driveInfo, nil
}

type UsersInDriveResponse struct {
	Drive        models.Drive  `json:"drive"`
	ReadUsers    []models.User `json:"readUsers"`
	WriteUsers   []models.User `json:"writeUsers"`
	ManagerUsers []models.User `json:"managerUsers"`
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
			Drive:        drive,
			ReadUsers:    users["read"],
			WriteUsers:   users["write"],
			ManagerUsers: users["manager"],
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
