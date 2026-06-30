package auth

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/models"
	"archivus/internal/services/storagemanager"
	"archivus/internal/store"
	storage_utils "archivus/internal/utils"
	"archivus/pkg/utils"
	"fmt"
)

type AuthService struct {
	Store              *store.Store
	StorageManager     storagemanager.StorageManager
	DefaultWriteAccess bool
	SecretKey          string
}

func (a *AuthService) CreateUser(username, password, pin, email string, userType models.UserType, isAdmin bool) (models.User, error) {
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

	user = models.User{
		Username: username,
		Password: hashedPassword,
		PIN:      hashedPIN,
		Email:    email,
		IsAdmin:  isAdmin,
		Type:     userType,
	}
	return a.Store.CreateUser(user)
}

func (a *AuthService) SetupNewDrive(name, userID string) (models.Drive, error) {
	user, err := a.Store.GetUserByID(userID)
	if err != nil {
		return models.Drive{}, fmt.Errorf("user not found: %w", err)
	}
	if !user.IsAdmin {
		return models.Drive{}, fmt.Errorf("only master users can create drives")
	}
	slug := storage_utils.CreateSlug(name)
	prefix, err := a.StorageManager.CreateDriveDir(slug)
	if err != nil {
		return models.Drive{}, err
	}

	drive, err := a.Store.CreateDrive(name, userID, slug, prefix, user.Type)
	if err != nil {
		if cleanupErr := a.StorageManager.DeleteDriveDir(name); cleanupErr != nil {
			fmt.Printf("warning: failed to clean up drive directory after db error: %v\n", cleanupErr)
		}
		return models.Drive{}, err
	}

	return drive, nil
}

// Authenticate validates a username/password pair (used by the WebDAV Basic
// Auth middleware) and returns the user ID on success.
func (a *AuthService) Authenticate(username, password string) (string, error) {
	user, err := a.Store.GetUserByUsername(username)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}
	if password == "" {
		return "", fmt.Errorf("password required")
	}
	if user.Password != utils.HashString(password) {
		return "", fmt.Errorf("invalid password")
	}
	return user.ID.String(), nil
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
