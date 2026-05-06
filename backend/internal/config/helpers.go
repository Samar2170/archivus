package config

import (
	"crypto/rand"
	"errors"
)

func generateRandomAlphaNumericString(length int) (string, error) {
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
