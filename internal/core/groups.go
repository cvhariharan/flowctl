package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/google/uuid"
)

func (c *Core) GetAllGroupsWithUsers(ctx context.Context) ([]models.Group, error) {
	g, err := c.store.GetAllGroupsWithUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get all groups: %w", err)
	}

	var groups []models.Group
	for _, v := range g {
		var users []models.User
		if v.Users != nil {
			if err := json.Unmarshal(v.Users.([]byte), &users); err != nil {
				return nil, fmt.Errorf("could not get users for the group %s: %w", v.Uuid.String(), err)
			}
		}
		groups = append(groups, models.Group{
			ID:          v.Uuid.String(),
			Name:        v.Name,
			Description: v.Description.String,
			Users:       users,
		})
	}

	return groups, nil
}

func (c *Core) GetGroupByUUID(ctx context.Context, groupUUID string) (models.Group, error) {
	gid, err := uuid.Parse(groupUUID)
	if err != nil {
		return models.Group{}, fmt.Errorf("group id should be a UUID: %w", err)
	}

	g, err := c.store.GetGroupByUUID(ctx, gid)
	if err != nil {
		return models.Group{}, fmt.Errorf("could not get group %s: %w", gid, err)
	}

	return models.Group{
		ID:          g.Uuid.String(),
		Name:        g.Name,
		Description: g.Description.String,
	}, nil
}

func (c *Core) DeleteGroupByUUID(ctx context.Context, groupUUID string) error {
	gid, err := uuid.Parse(groupUUID)
	if err != nil {
		return fmt.Errorf("group id should be a UUID: %w", err)
	}

	return c.store.DeleteGroupByUUID(ctx, gid)
}

func (c *Core) CreateGroup(ctx context.Context, name, description string) (models.Group, error) {
	g, err := c.store.CreateGroup(ctx, repo.CreateGroupParams{
		Name:        name,
		Description: sql.NullString{String: description, Valid: true},
	})
	if err != nil {
		return models.Group{}, fmt.Errorf("could not create group %s: %w", name, err)
	}

	return models.Group{
		ID:          g.Uuid.String(),
		Name:        g.Name,
		Description: g.Description.String,
	}, nil
}

func (c *Core) SearchGroup(ctx context.Context, query string) ([]models.Group, error) {
	g, err := c.store.SearchGroup(ctx, query)
	if err != nil {
		return nil, err
	}

	var groups []models.Group
	for _, v := range g {
		var users []models.User
		if v.Users != nil {
			if err := json.Unmarshal(v.Users.([]byte), &users); err != nil {
				return nil, fmt.Errorf("could not get users for the group %s: %w", v.Uuid.String(), err)
			}
		}
		groups = append(groups, models.Group{
			ID:          v.Uuid.String(),
			Name:        v.Name,
			Description: v.Description.String,
			Users:       users,
		})
	}

	return groups, nil
}
