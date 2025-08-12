package auth

import (
	"archivus/internal/db"
	"archivus/internal/models"
	"archivus/internal/utils"
)

func ReGenerateAPIKey(username, password, pin string) (string, error) {
	if username == "" && pin == "" {
		return "", utils.HandleError("ReGenerateAPIKey", "Username and PIN cannot be empty", nil)
	}
	var user models.User
	newKey, err := utils.GenerateAPIKey(32)
	if err != nil {
		return "", utils.HandleError("ReGenerateAPIKey", "Failed to generate new API key", err)
	}
	err = db.StorageDB.Where("username = ?", username).Where("pin = ?", pin).First(&user).Error
	if err != nil {
		err = db.StorageDB.Where("username = ?", username).Where("password = ?", utils.HashString(password)).First(&user).Error
		if err != nil {
			return "", utils.HandleError("ReGenerateAPIKey", "User not found or invalid credentials", err)
		}
	}
	user.APIKey = utils.HashString(newKey)
	db.StorageDB.Save(&user)
	return newKey, nil
}

func IsKeyValid(key string) bool {
	keyHash := utils.HashString(key)
	var user models.User
	err := db.StorageDB.Where("api_key = ?", keyHash).First(&user).Error
	return err == nil
}
