package store

import (
	"archivus/internal/models"
	"fmt"

	"github.com/google/uuid"
)

func (s *Store) CreateApiKey(key models.ApiKey) (models.ApiKey, error) {
	result := s.conn().Create(&key)
	return key, result.Error
}

func (s *Store) GetApiKeyByHash(keyHash string) (models.ApiKey, error) {
	var key models.ApiKey
	result := s.conn().First(&key, "key_hash = ?", keyHash)
	return key, result.Error
}

func (s *Store) GetApiKeysByUserID(userID string) ([]models.ApiKey, error) {
	userIDParsed, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}
	var keys []models.ApiKey
	result := s.conn().Where("user_id = ?", userIDParsed).Order("created_at DESC").Find(&keys)
	return keys, result.Error
}

// DeleteApiKey soft-deletes one of the user's own keys. Scoping by user ID
// makes revoking someone else's key a not-found, not a forbidden, so the
// endpoint does not leak which key IDs exist.
func (s *Store) DeleteApiKey(userID, keyID string) error {
	userIDParsed, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	keyIDParsed, err := uuid.Parse(keyID)
	if err != nil {
		return fmt.Errorf("invalid api key ID: %w", err)
	}
	result := s.conn().Where("user_id = ? AND id = ?", userIDParsed, keyIDParsed).Delete(&models.ApiKey{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRecordNotFound
	}
	return nil
}
