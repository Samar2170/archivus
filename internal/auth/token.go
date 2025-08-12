package auth

import (
	"time"

	"archivus/config"
	"archivus/internal/utils"

	"github.com/golang-jwt/jwt/v5"
)

func createToken(userID, username string) (string, error) {
	claims := jwt.MapClaims{
		"user_id":    userID,
		"username":   username,
		"issued_at":  time.Now().Unix(),
		"expires_at": time.Now().Add(24 * 20 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.Config.SecretKey))
	if err != nil {
		return "", utils.HandleError("createToken", "Failed to sign token", err)
	}
	return tokenString, nil
}
