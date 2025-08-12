package auth

import (
	"archivus/internal/db"
	"archivus/internal/models"
	"archivus/internal/utils"
)

func CreateUser(username, password, pin, email string) (string, string, error) {
	// Validate input
	if username == "" || pin == "" || email == "" || password == "" {
		return "", "", utils.HandleError("CreateUser", "Username, PIN, and email cannot be empty", nil)
	}

	// Generate API key
	apiKey, err := utils.GenerateAPIKey(32)
	if err != nil {
		return "", "", utils.HandleError("CreateUser", "Failed to generate API key", err)
	}

	// Hash the PIN
	hashedPin := utils.HashString(pin)
	hashedPassword := utils.HashString(password)

	// Create user in the database
	user := models.User{
		Username: username,
		PIN:      hashedPin,
		Email:    email,
		APIKey:   apiKey,
		Password: hashedPassword,
	}
	if err := db.StorageDB.Create(&user).Error; err != nil {
		return "", "", utils.HandleError("CreateUser", "Failed to create user in database", err)
	}

	return apiKey, user.ID.String(), nil
}

type LoginUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	PIN      string `json:"pin"`
}

func LoginUser(req LoginUserRequest) (string, string, error) {
	var user models.User
	var err error
	var token string
	var userId string
	if req.Username == "" || req.Password == "" || req.PIN == "" {
		return token, userId, utils.HandleError("LoginUser", "Username, password, and PIN cannot be empty", nil)
	}
	if req.PIN == "" {
		err = db.StorageDB.Where("username = ?", req.Username).
			Where("password = ?", utils.HashString(req.Password)).
			First(&user).Error
	} else {
		err = db.StorageDB.Where("username = ?", req.Username).
			Where("pin = ?", utils.HashString(req.PIN)).
			First(&user).Error
	}
	userId = user.ID.String()
	if err != nil {
		return token, userId, utils.HandleError("LoginUser", "Invalid credentials", err)
	}
	token, err = createToken(user.ID.String(), user.Username)
	if err != nil {
		return token, userId, utils.HandleError("LoginUser", "Failed to create token", err)
	}
	return token, userId, nil
}
