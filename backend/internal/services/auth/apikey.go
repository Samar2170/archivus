package auth

import (
	archivus_constants "archivus/internal/constants"
	"archivus/internal/models"
	"archivus/pkg/utils"
	"fmt"
	"strings"
	"time"
)

// ApiKeyInfo is the safe projection of an API key: the key hash is never
// sent to clients, and the plaintext key only appears in CreatedApiKey.
type ApiKeyInfo struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	AccessLevel models.AccessLevel `json:"access_level"`
	CreatedAt   time.Time          `json:"created_at"`
	ExpiresAt   time.Time          `json:"expires_at"`
}

// CreatedApiKey is returned exactly once, when the key is created. The
// plaintext key cannot be recovered later, so clients must store it.
type CreatedApiKey struct {
	ApiKeyInfo
	ApiKey string `json:"api_key"`
}

func (a *AuthService) CreateApiKey(userID, name string, accessLevel models.AccessLevel) (CreatedApiKey, error) {
	user, err := a.Store.GetUserByID(userID)
	if err != nil {
		return CreatedApiKey{}, fmt.Errorf("user not found: %w", err)
	}
	if accessLevel == "" {
		accessLevel = models.AccessLevelRead
	}
	if accessLevel != models.AccessLevelRead && accessLevel != models.AccessLevelWrite {
		return CreatedApiKey{}, fmt.Errorf("api key access level must be read or write")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "api-key"
	}
	if len(name) > archivus_constants.ApiKeyNameMaxLength {
		return CreatedApiKey{}, fmt.Errorf("api key name must be at most %d characters long", archivus_constants.ApiKeyNameMaxLength)
	}

	key, err := utils.GenerateRandomAlphaNumericString(archivus_constants.ApiKeyLength)
	if err != nil {
		return CreatedApiKey{}, fmt.Errorf("failed to generate api key: %w", err)
	}

	apiKey := models.ApiKey{
		UserID:      user.ID,
		Name:        name,
		KeyHash:     utils.HashString(key),
		AccessLevel: accessLevel,
		ExpiresAt:   time.Now().Add(archivus_constants.ApiKeyValidityDays * 24 * time.Hour),
	}
	apiKey, err = a.Store.CreateApiKey(apiKey)
	if err != nil {
		return CreatedApiKey{}, fmt.Errorf("failed to create api key: %w", err)
	}

	return CreatedApiKey{
		ApiKeyInfo: ApiKeyInfo{
			ID:          apiKey.ID.String(),
			Name:        apiKey.Name,
			AccessLevel: apiKey.AccessLevel,
			CreatedAt:   apiKey.CreatedAt,
			ExpiresAt:   apiKey.ExpiresAt,
		},
		ApiKey: key,
	}, nil
}

func (a *AuthService) ValidateApiKey(apiKey string) (models.ApiKey, error) {
	key, err := a.Store.GetApiKeyByHash(utils.HashString(apiKey))
	if err != nil {
		return models.ApiKey{}, fmt.Errorf("invalid api key")
	}
	if key.ExpiresAt.Before(time.Now()) {
		return models.ApiKey{}, fmt.Errorf("api key has expired")
	}
	// The key outlives the user only on paper: stop authenticating deleted
	// users even if their row is merely soft-deleted.
	if _, err := a.Store.GetUserByID(key.UserID.String()); err != nil {
		return models.ApiKey{}, fmt.Errorf("invalid api key")
	}
	return key, nil
}

func (a *AuthService) GetApiKeys(userID string) ([]ApiKeyInfo, error) {
	keys, err := a.Store.GetApiKeysByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list api keys: %w", err)
	}
	infos := make([]ApiKeyInfo, 0, len(keys))
	for _, key := range keys {
		infos = append(infos, ApiKeyInfo{
			ID:          key.ID.String(),
			Name:        key.Name,
			AccessLevel: key.AccessLevel,
			CreatedAt:   key.CreatedAt,
			ExpiresAt:   key.ExpiresAt,
		})
	}
	return infos, nil
}

func (a *AuthService) RevokeApiKey(userID, keyID string) error {
	if err := a.Store.DeleteApiKey(userID, keyID); err != nil {
		return fmt.Errorf("failed to revoke api key: %w", err)
	}
	return nil
}
