package core

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/google/uuid"
)

func (c *Core) CreateCredential(ctx context.Context, cred *models.Credential, namespaceID string) (*models.Credential, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	if cred.Name == "" {
		return nil, errors.New("credential name is required")
	}

	if cred.KeyData == "" {
		return nil, errors.New("key data is required")
	}

	if cred.KeyType == "" {
		return nil, errors.New("key type is required")
	}

	enc, err := c.keeper.Encrypt(ctx, []byte(cred.KeyData))
	if err != nil {
		return nil, err
	}
	encryptedKeyData := hex.EncodeToString(enc)

	created, err := c.store.CreateCredential(ctx, repo.CreateCredentialParams{
		Name:    cred.Name,
		KeyType: cred.KeyType,
		KeyData: encryptedKeyData,
		Uuid:    namespaceUUID,
	})
	if err != nil {
		return nil, err
	}

	return models.RepoCredentialToCredential(created), nil
}

func (c *Core) GetCredentialByID(ctx context.Context, id string, namespaceID string) (*models.Credential, error) {
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	cred, err := c.store.GetCredentialByUUID(ctx, repo.GetCredentialByUUIDParams{
		Uuid:   uuidID,
		Uuid_2: namespaceUUID,
	})
	if err != nil {
		return nil, err
	}

	return models.RepoCredentialByUUIDToCredential(cred), nil
}

func (c *Core) ListCredentials(ctx context.Context, limit, offset int, namespaceID string) ([]*models.Credential, int64, int64, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, -1, -1, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	creds, err := c.store.ListCredentials(ctx, repo.ListCredentialsParams{
		Uuid:   namespaceUUID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, -1, -1, err
	}

	results := models.RepoCredentialListToCredential(creds)

	if len(creds) > 0 {
		return results, creds[0].PageCount, creds[0].TotalCount, nil
	}

	return results, 0, 0, nil
}

func (c *Core) UpdateCredential(ctx context.Context, id string, cred *models.Credential, namespaceID string) (*models.Credential, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	if cred.Name == "" {
		return nil, errors.New("credential name is required")
	}
	if cred.KeyData == "" {
		return nil, errors.New("key data is required")
	}

	if cred.KeyType == "" {
		return nil, errors.New("key type is required")
	}

	uuidID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	enc, err := c.keeper.Encrypt(ctx, []byte(cred.KeyData))
	if err != nil {
		return nil, err
	}
	encryptedKeyData := hex.EncodeToString(enc)

	updated, err := c.store.UpdateCredential(ctx, repo.UpdateCredentialParams{
		Uuid:    uuidID,
		Name:    cred.Name,
		KeyType: cred.KeyType,
		KeyData: encryptedKeyData,
		Uuid_2:  namespaceUUID,
	})
	if err != nil {
		return nil, err
	}

	return models.RepoCredentialToCredential(updated), nil
}

func (c *Core) DeleteCredential(ctx context.Context, id string, namespaceID string) error {
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid credential UUID: %w", err)
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	return c.store.DeleteCredential(ctx, repo.DeleteCredentialParams{
		Uuid:   uuidID,
		Uuid_2: namespaceUUID,
	})
}
