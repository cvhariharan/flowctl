package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/google/uuid"
)

func (c *Core) CreateNamespace(ctx context.Context, namespace *models.Namespace) (models.Namespace, error) {
	if namespace.Name == "" {
		return models.Namespace{}, errors.New("namespace name is required")
	}

	created, err := c.store.CreateNamespace(ctx, namespace.Name)
	if err != nil {
		return models.Namespace{}, err
	}

	return models.Namespace{
		ID:   created.Uuid.String(),
		Name: created.Name,
	}, nil
}

func (c *Core) GetNamespaceByID(ctx context.Context, id string) (models.Namespace, error) {
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return models.Namespace{}, err
	}

	namespace, err := c.store.GetNamespaceByUUID(ctx, uuidID)
	if err != nil {
		return models.Namespace{}, err
	}

	return models.Namespace{
		ID:   namespace.Uuid.String(),
		Name: namespace.Name,
	}, nil
}

func (c *Core) GetNamespaceByName(ctx context.Context, name string) (models.Namespace, error) {
	ns, err := c.store.GetNamespaceByName(ctx, name)
	if err != nil {
		return models.Namespace{}, fmt.Errorf("could not get namespace %s: %w", name, err)
	}

	return models.Namespace{
		ID:   ns.Uuid.String(),
		Name: ns.Name,
	}, nil
}

func (c *Core) ListNamespaces(ctx context.Context, limit, offset int) ([]models.Namespace, int64, int64, error) {
	namespaces, err := c.store.ListNamespaces(ctx, repo.ListNamespacesParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, -1, -1, err
	}

	results := make([]models.Namespace, len(namespaces))
	for i, n := range namespaces {
		results[i] = models.Namespace{
			ID:   n.Uuid.String(),
			Name: n.Name,
		}
	}

	if len(namespaces) > 0 {
		return results, namespaces[0].PageCount, namespaces[0].TotalCount, nil
	}
	return results, 0, 0, nil
}

func (c *Core) UpdateNamespace(ctx context.Context, id string, namespace models.Namespace) (models.Namespace, error) {
	if namespace.Name == "" {
		return models.Namespace{}, errors.New("namespace name is required")
	}

	uuidID, err := uuid.Parse(id)
	if err != nil {
		return models.Namespace{}, err
	}

	updated, err := c.store.UpdateNamespace(ctx, repo.UpdateNamespaceParams{
		Uuid: uuidID,
		Name: namespace.Name,
	})
	if err != nil {
		return models.Namespace{}, err
	}

	return models.Namespace{
		ID:   updated.Uuid.String(),
		Name: updated.Name,
	}, nil
}

func (c *Core) DeleteNamespace(ctx context.Context, id string) error {
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return err
	}
	return c.store.DeleteNamespace(ctx, uuidID)
}

// CanAccessNamespace checks if a user can access a given namespace.
// Returns true if:
// 1. User is admin (access to all namespaces)
// 2. User belongs to a group with access to the namespace
func (c *Core) CanAccessNamespace(ctx context.Context, userID string, namespaceID string) (bool, error) {
	// Get user info (assumes you have a method to fetch user by ID)
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return false, fmt.Errorf("invalid user UUID: %w", err)
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return false, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	user, err := c.store.GetUserByUUID(ctx, userUUID)
	if err != nil {
		return false, err
	}

	// Admins have access to all namespaces
	if user.Role == "admin" {
		return true, nil
	}

	// Check if user belongs to any group with access to the namespace
	hasAccess, err := c.store.CheckUserNamespaceAccess(ctx, repo.CheckUserNamespaceAccessParams{
		UserID: user.ID,
		Uuid: namespaceUUID,
	})
	if err != nil {
		return false, err
	}

	return hasAccess, nil
}


func (c *Core) GetGroupsWithNamespaceAccess(ctx context.Context, namespaceID string) ([]models.Group, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	g, err := c.store.GetGroupsWithNamespaceAccess(ctx, namespaceUUID)
	if err != nil {
		return nil, fmt.Errorf("could not get groups with access to namespace %s: %w", namespaceID, err)
	}

	var groups []models.Group
	for _, gr := range g {
		groups = append(groups, models.Group{
			ID: gr.Uuid.String(),
			Name: gr.Name,
			Description: gr.Description.String,
		})
	}

	return groups, nil
}

func (c *Core) GrantGroupNamespaceAccess(ctx context.Context, groupID, namespaceID string) error {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	groupUUID, err := uuid.Parse(groupID)
	if err != nil {
		return fmt.Errorf("invalid group UUID: %w", err)
	}

	_, err = c.store.GrantGroupNamespaceAccess(ctx, repo.GrantGroupNamespaceAccessParams{
		Uuid: groupUUID,
		Uuid_2: namespaceUUID,
	})
	return err
}

func (c *Core) RevokeGroupNamespaceAccess(ctx context.Context, groupID, namespaceID string) error {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	groupUUID, err := uuid.Parse(groupID)
	if err != nil {
		return fmt.Errorf("invalid group UUID: %w", err)
	}

	return c.store.RevokeGroupNamespaceAccess(ctx, repo.RevokeGroupNamespaceAccessParams{
		Uuid: groupUUID,
		Uuid_2: namespaceUUID,
	})
}
