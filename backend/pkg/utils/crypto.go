package utils

import (
	"archivus/internal/config"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

func HashString(input string) string {
	combined := append([]byte(input), []byte(config.Config.SecretKey)...)
	hash := sha256.New()
	hash.Write(combined)
	hashedBytes := hash.Sum(nil)
	return hex.EncodeToString(hashedBytes)
}

func GenerateRandomNumber(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("length must be greater than 0")
	}
	num := make([]byte, length)
	_, err := rand.Read(num)
	if err != nil {
		return "", errors.New("failed to generate random bytes")
	}
	for i := 0; i < length; i++ {
		num[i] = '0' + (num[i] % 10) // Convert to a digit
	}
	return string(num), nil
}

func GenerateRandomAlphaNumericString(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("length must be greater than 0")
	}
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", errors.New("failed to generate random bytes")
	}
	for i := 0; i < length; i++ {
		result[i] = charset[randomBytes[i]%byte(len(charset))]
	}
	return string(result), nil
}
