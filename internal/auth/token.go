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

func DecodeToken(tokenString string) (string, string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, utils.HandleError("DecodeToken", "Unexpected signing method", nil)
		}
		return []byte(config.Config.SecretKey), nil
	})
	if err != nil || !token.Valid {
		return "", "", utils.HandleError("DecodeToken", "Invalid token", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", utils.HandleError("DecodeToken", "Invalid token claims", nil)
	}

	userID := claims["user_id"].(string)
	username := claims["username"].(string)

	return userID, username, nil
}
