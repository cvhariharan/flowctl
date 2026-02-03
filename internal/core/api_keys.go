package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	// APIKeyPrefix is the prefix used for all API keys
	APIKeyPrefix = "flowctl_"
	// APIKeyLength is the length of the random portion of the key
	APIKeyLength = 32
)

// generateAPIKey generates a new API key with the flowctl_ prefix
func generateAPIKey() (string, error) {
	b := make([]byte, APIKeyLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}
	return APIKeyPrefix + base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b), nil
}

// hashAPIKey creates a hash of the API key for storage
func hashAPIKey(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash API key: %w", err)
	}
	return string(hash), nil
}

// getKeyPrefix extracts a prefix from the key for lookup purposes
// Uses first 8 characters of the SHA256 hash of the key
func getKeyPrefix(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])[:8]
}

// CreateAPIKey creates a new API key for the given user
func (c *Core) CreateAPIKey(ctx context.Context, name, userUUID string, expiresAt *time.Time) (models.APIKeyWithRawKey, error) {
	// Parse user UUID
	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return models.APIKeyWithRawKey{}, fmt.Errorf("invalid user UUID: %w", err)
	}

	// Get user internal ID
	userID, err := c.store.GetUserIDByUUID(ctx, uid)
	if err != nil {
		return models.APIKeyWithRawKey{}, fmt.Errorf("user not found: %w", err)
	}

	// Generate the raw API key
	rawKey, err := generateAPIKey()
	if err != nil {
		return models.APIKeyWithRawKey{}, err
	}

	// Hash the key for storage
	keyHash, err := hashAPIKey(rawKey)
	if err != nil {
		return models.APIKeyWithRawKey{}, err
	}

	// Get prefix for lookup
	keyPrefix := getKeyPrefix(rawKey)

	// Prepare expiration
	var expiresAtSQL sql.NullTime
	if expiresAt != nil {
		expiresAtSQL = sql.NullTime{Time: *expiresAt, Valid: true}
	}

	// Create the API key in the database
	apiKey, err := c.store.CreateAPIKey(ctx, repo.CreateAPIKeyParams{
		Name:      name,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		UserID:    userID,
		ExpiresAt: expiresAtSQL,
	})
	if err != nil {
		return models.APIKeyWithRawKey{}, fmt.Errorf("failed to create API key: %w", err)
	}

	result := models.APIKeyWithRawKey{
		APIKey: repoAPIKeyToModel(apiKey, userUUID),
		RawKey: rawKey,
	}

	return result, nil
}

// ValidateAPIKey validates an API key and returns the associated user info
func (c *Core) ValidateAPIKey(ctx context.Context, rawKey string) (models.UserInfo, error) {
	// Get prefix for lookup
	keyPrefix := getKeyPrefix(rawKey)

	// Look up the API key by prefix
	apiKeyRow, err := c.store.GetAPIKeyByPrefix(ctx, keyPrefix)
	if err != nil {
		return models.UserInfo{}, fmt.Errorf("API key not found: %w", err)
	}

	// Verify the key hash
	if err := bcrypt.CompareHashAndPassword([]byte(apiKeyRow.KeyHash), []byte(rawKey)); err != nil {
		return models.UserInfo{}, fmt.Errorf("invalid API key: %w", err)
	}

	// Check expiration
	if apiKeyRow.ExpiresAt.Valid && time.Now().After(apiKeyRow.ExpiresAt.Time) {
		return models.UserInfo{}, fmt.Errorf("API key has expired")
	}

	// Update last used timestamp (fire and forget)
	go func() {
		_ = c.store.UpdateAPIKeyLastUsed(context.Background(), apiKeyRow.ID)
	}()

	// Get user groups
	groups, err := c.store.GetUserGroups(ctx, apiKeyRow.UserUuid)
	if err != nil {
		return models.UserInfo{}, fmt.Errorf("failed to get user groups: %w", err)
	}

	var groupIDs []string
	for _, g := range groups {
		groupIDs = append(groupIDs, g.Uuid.String())
	}

	return models.UserInfo{
		ID:       apiKeyRow.UserUuid.String(),
		Username: apiKeyRow.Username,
		Name:     apiKeyRow.UserName,
		Role:     string(apiKeyRow.Role),
		Groups:   groupIDs,
	}, nil
}

// ListAPIKeysByUser returns all API keys for a given user
func (c *Core) ListAPIKeysByUser(ctx context.Context, userUUID string) ([]models.APIKey, error) {
	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return nil, fmt.Errorf("invalid user UUID: %w", err)
	}

	apiKeys, err := c.store.GetAPIKeysByUserUUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	result := make([]models.APIKey, len(apiKeys))
	for i, k := range apiKeys {
		result[i] = repoAPIKeyToModel(k, userUUID)
	}

	return result, nil
}

// DeleteAPIKey deletes an API key by its UUID
func (c *Core) DeleteAPIKey(ctx context.Context, keyUUID string) error {
	uid, err := uuid.Parse(keyUUID)
	if err != nil {
		return fmt.Errorf("invalid API key UUID: %w", err)
	}

	if err := c.store.DeleteAPIKeyByUUID(ctx, uid); err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	return nil
}

// repoAPIKeyToModel converts a repo.ApiKey to models.APIKey
func repoAPIKeyToModel(k repo.ApiKey, userUUID string) models.APIKey {
	apiKey := models.APIKey{
		ID:        k.Uuid.String(),
		Name:      k.Name,
		KeyPrefix: k.KeyPrefix,
		UserID:    userUUID,
		CreatedAt: k.CreatedAt,
		UpdatedAt: k.UpdatedAt,
	}

	if k.ExpiresAt.Valid {
		apiKey.ExpiresAt = &k.ExpiresAt.Time
	}
	if k.LastUsedAt.Valid {
		apiKey.LastUsedAt = &k.LastUsedAt.Time
	}

	return apiKey
}
