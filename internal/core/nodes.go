package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/google/uuid"
)

func (c *Core) CreateNode(ctx context.Context, node *models.Node, namespaceID string) (*models.Node, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	if node.Name == "" {
		return nil, errors.New("node name is required")
	}
	if node.Hostname == "" {
		return nil, errors.New("hostname is required")
	}

	credID, err := uuid.Parse(node.Auth.CredentialID)
	if err != nil {
		return nil, errors.New("invalid credential ID format")
	}

	credential, err := c.store.GetCredentialByUUID(ctx, repo.GetCredentialByUUIDParams{
		Uuid:   credID,
		Uuid_2: namespaceUUID,
	})
	if err != nil {
		return nil, errors.New("credential not found")
	}

	created, err := c.store.CreateNode(ctx, repo.CreateNodeParams{
		Name:         node.Name,
		Hostname:     node.Hostname,
		Port:         int32(node.Port),
		Username:     node.Username,
		OsFamily:     node.OSFamily,
		Tags:         node.Tags,
		AuthMethod:   repo.AuthenticationMethod(node.Auth.Method),
		CredentialID: sql.NullInt32{Int32: credential.ID, Valid: true},
		Uuid:         namespaceUUID,
	})
	if err != nil {
		return nil, err
	}

	key := credential.KeyData

	return &models.Node{
		ID:       created.Uuid.String(),
		Name:     created.Name,
		Hostname: created.Hostname,
		Port:     int(created.Port),
		Username: created.Username,
		OSFamily: created.OsFamily,
		Tags:     created.Tags,
		Auth: models.NodeAuth{
			Method:       node.Auth.Method,
			CredentialID: credential.Uuid.String(),
			Key:          key,
		},
	}, nil
}

func (c *Core) GetNodeByID(ctx context.Context, id string, namespaceID string) (*models.Node, error) {
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, err
	}

	node, err := c.store.GetNodeByUUID(ctx, repo.GetNodeByUUIDParams{
		Uuid:   uuidID,
		Uuid_2: namespaceUUID,
	})
	if err != nil {
		return nil, err
	}

	credential, err := c.store.GetCredentialByID(ctx, repo.GetCredentialByIDParams{
		ID:   node.CredentialID.Int32,
		Uuid: namespaceUUID,
	})
	if err != nil {
		return nil, errors.New("credential not found")
	}

	key := credential.KeyData

	return &models.Node{
		ID:       node.Uuid.String(),
		Name:     node.Name,
		Hostname: node.Hostname,
		Port:     int(node.Port),
		Username: node.Username,
		OSFamily: node.OsFamily,
		Tags:     node.Tags,
		Auth: models.NodeAuth{
			Method:       models.AuthMethod(node.AuthMethod),
			CredentialID: credential.Uuid.String(),
			Key:          key,
		},
	}, nil
}

func (c *Core) ListNodes(ctx context.Context, limit, offset int, namespaceID string) ([]*models.Node, int64, int64, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, -1, -1, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	nodes, err := c.store.ListNodes(ctx, repo.ListNodesParams{
		Uuid:   namespaceUUID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, -1, -1, err
	}

	results := make([]*models.Node, 0)
	for _, n := range nodes {
		res, err := c.GetNodeByID(ctx, n.Uuid.String(), namespaceID)
		if err != nil {
			return nil, -1, -1, err
		}
		results = append(results, res)
	}

	// Get pagination metadata
	if len(nodes) > 0 {
		return results, nodes[0].PageCount, nodes[0].TotalCount, nil
	}
	return results, 0, 0, nil
}

func (c *Core) UpdateNode(ctx context.Context, id string, node *models.Node, namespaceID string) (*models.Node, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	if node.Name == "" {
		return nil, errors.New("node name is required")
	}
	if node.Hostname == "" {
		return nil, errors.New("hostname is required")
	}

	uuidID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	credID, _ := uuid.Parse(node.Auth.CredentialID)
	credential, err := c.store.GetCredentialByUUID(ctx, repo.GetCredentialByUUIDParams{
		Uuid:   credID,
		Uuid_2: namespaceUUID,
	})
	if err != nil {
		return nil, errors.New("credential not found")
	}

	updated, err := c.store.UpdateNode(ctx, repo.UpdateNodeParams{
		Uuid:         uuidID,
		Name:         node.Name,
		Hostname:     node.Hostname,
		Port:         int32(node.Port),
		Username:     node.Username,
		OsFamily:     node.OSFamily,
		Tags:         node.Tags,
		AuthMethod:   repo.AuthenticationMethod(node.Auth.Method),
		CredentialID: sql.NullInt32{Int32: credential.ID, Valid: true},
		Uuid_2:       namespaceUUID,
	})
	if err != nil {
		return nil, err
	}

	key := credential.KeyData

	return &models.Node{
		ID:       updated.Uuid.String(),
		Name:     updated.Name,
		Hostname: updated.Hostname,
		Port:     int(updated.Port),
		Username: updated.Username,
		OSFamily: updated.OsFamily,
		Tags:     updated.Tags,
		Auth: models.NodeAuth{
			Method:       models.AuthMethod(updated.AuthMethod),
			CredentialID: credential.Uuid.String(),
			Key:          key,
		},
	}, nil
}

func (c *Core) DeleteNode(ctx context.Context, id string, namespaceID string) error {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	uuidID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid node UUID: %w", err)
	}
	return c.store.DeleteNode(ctx, repo.DeleteNodeParams{
		Uuid:   uuidID,
		Uuid_2: namespaceUUID,
	})
}

