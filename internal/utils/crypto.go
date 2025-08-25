package utils

import (
	"archivus/config"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

func GenerateAPIKey(n int) (string, error) {
	key := make([]byte, n)
	_, err := rand.Read(key)
	if err != nil {
		return "", HandleError("GenerateAPIKey", "Failed to generate random bytes for API key", err)
	}
	apiKey := base64.RawURLEncoding.EncodeToString(key)
	return apiKey, nil
}
func HashString(input string) string {
	combined := append([]byte(input), []byte(config.Config.SecretKey)...)
	hash := sha256.New()
	hash.Write(combined)
	hashedBytes := hash.Sum(nil)
	return hex.EncodeToString(hashedBytes)
}

func GenerateRandomNumber(length int) (string, error) {
	if length <= 0 {
		return "", HandleError("GenerateRandomNumber", "Length must be greater than zero", nil)
	}
	num := make([]byte, length)
	_, err := rand.Read(num)
	if err != nil {
		return "", HandleError("GenerateRandomNumber", "Failed to generate random bytes", err)
	}
	for i := 0; i < length; i++ {
		num[i] = '0' + (num[i] % 10) // Convert to a digit
	}
	return string(num), nil
}
