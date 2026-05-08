package auth

import (
	"archivus/internal/config"
	archivus_constants "archivus/internal/constants"
	"archivus/internal/models"
	dirmanager "archivus/internal/services/dirmanager"
	"archivus/internal/store"
	"fmt"
)

type AuthService struct {
	Store *store.Store
}

func (a *AuthService) CreateUser(username, password, pin, email string, isMaster bool) (models.User, error) {
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
	return a.Store.CreateUser(user)
}

func (a *AuthService) SetupNewDrive(name, userID string) (models.Drive, error) {
	user, err := a.Store.GetUserByID(userID)
	if err != nil {
		return models.Drive{}, fmt.Errorf("user not found: %w", err)
	}
	if !user.IsMaster {
		return models.Drive{}, fmt.Errorf("only master users can create drives")
	}

	slug, absPath, err := dirmanager.CreateDriveDir(name)
	if err != nil {
		return models.Drive{}, err
	}

	drive, err := a.Store.CreateDrive(name, userID, slug, absPath)
	if err != nil {
		if cleanupErr := dirmanager.DeleteDriveDir(name); cleanupErr != nil {
			fmt.Printf("warning: failed to clean up drive directory after db error: %v\n", cleanupErr)
		}
		return models.Drive{}, err
	}

	return drive, nil
}
