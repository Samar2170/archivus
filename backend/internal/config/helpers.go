package config

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"strconv"
)

// envString overwrites dst with the value of the environment variable name, if
// it is set to a non-empty value.
func envString(name string, dst *string) {
	if v := os.Getenv(name); v != "" {
		*dst = v
	}
}

// envBool overwrites dst with the value of the environment variable name, if it
// is set. It accepts anything strconv.ParseBool does (1, t, true, 0, f, false).
func envBool(name string, dst *bool) error {
	v := os.Getenv(name)
	if v == "" {
		return nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fmt.Errorf("%s: invalid boolean %q", name, v)
	}
	*dst = b
	return nil
}

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
