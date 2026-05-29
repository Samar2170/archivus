package auth

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/models"
	"archivus/internal/services/storagemanager"
	"archivus/internal/store"
	"archivus/pkg/utils"
	"fmt"
)

type AuthService struct {
	Store              *store.Store
	DirManager         storagemanager.StorageManager
	DefaultWriteAccess bool
	SecretKey          string
}

func (a *AuthService) CreateUser(username, password, pin, email string, isMaster bool) (models.User, error) {
	var user models.User
	var err error

	user, err = a.Store.GetUserByUsername(username)
	if err == nil {
		return models.User{}, fmt.Errorf("username already exists")
	}
	if len(username) < 3 {
		return models.User{}, fmt.Errorf("username must be at least 3 characters long")
	}
	if len(password) < archivus_constants.MinPasswordLength {
		return models.User{}, fmt.Errorf("password must be at least %d characters long", archivus_constants.MinPasswordLength)
	}
	if len(pin) != archivus_constants.PINLength {
		return models.User{}, fmt.Errorf("pin must be exactly %d digits long", archivus_constants.PINLength)
	}

	hashedPassword := utils.HashString(password)
	hashedPIN := utils.HashString(pin)

	var writeAccess bool
	if isMaster {
		writeAccess = true
	} else {
		writeAccess = a.DefaultWriteAccess
	}
	user = models.User{
		Username:    username,
		Password:    hashedPassword,
		PIN:         hashedPIN,
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

	slug, absPath, err := a.DirManager.CreateDriveDir(name)
	if err != nil {
		return models.Drive{}, err
	}

	drive, err := a.Store.CreateDrive(name, userID, slug, absPath)
	if err != nil {
		if cleanupErr := a.DirManager.DeleteDriveDir(name); cleanupErr != nil {
			fmt.Printf("warning: failed to clean up drive directory after db error: %v\n", cleanupErr)
		}
		return models.Drive{}, err
	}

	return drive, nil
}

func (a *AuthService) Login(username, password, pin string) (token string, err error) {
	user, err := a.Store.GetUserByUsername(username)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if password == "" && pin == "" {
		return "", fmt.Errorf("password or PIN required")
	}
	hashedPassword := utils.HashString(password)
	if password != "" && user.Password != hashedPassword {
		return "", fmt.Errorf("invalid password")
	}
	hashedPIN := utils.HashString(pin)
	if pin != "" && user.PIN != hashedPIN {
		return "", fmt.Errorf("invalid PIN")
	}
	token, err = a.createToken(user.ID.String(), user.Username)
	if err != nil {
		return "", fmt.Errorf("failed to create token: %w", err)
	}
	return token, nil
}

func (a *AuthService) CheckMasterUser() (bool, error) {
	exists, err := a.Store.CheckMasterUserExists()
	if err != nil {
		return false, fmt.Errorf("failed to check master user: %w", err)
	}
	return exists, nil
}
