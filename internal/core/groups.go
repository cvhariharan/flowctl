package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/google/uuid"
)

func (c *Core) GetAllGroupsWithUsers(ctx context.Context) ([]models.GroupWithUsers, error) {
	g, err := c.store.GetAllGroupsWithUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get all groups: %w", err)
	}

	var groups []models.GroupWithUsers
	for _, v := range g {
		grp, err := c.repoGroupViewToGroupWithUsers(v)
		if err != nil {
			return nil, err
		}

		groups = append(groups, grp)
	}

	return groups, nil
}

func (c *Core) GetAllGroups(ctx context.Context) ([]models.Group, error) {
	g, err := c.store.GetAllGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get all groups: %w", err)
	}

	var groups []models.Group
	for _, v := range g {
		grp := c.repoGroupToGroup(v)
		groups = append(groups, grp)
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

	return c.repoGroupToGroup(g), nil
}

func (c *Core) GetGroupByName(ctx context.Context, name string) (models.Group, error) {
	g, err := c.store.GetGroupByName(ctx, name)
	if err != nil {
		return models.Group{}, fmt.Errorf("could not get group with name %s: %w", name, err)
	}

	return c.repoGroupToGroup(g), nil
}

func (c *Core) DeleteGroupByUUID(ctx context.Context, groupUUID string) error {
	gid, err := uuid.Parse(groupUUID)
	if err != nil {
		return fmt.Errorf("group id should be a UUID: %w", err)
	}

	return c.store.DeleteGroupByUUID(ctx, gid)
}

func (c *Core) CreateGroup(ctx context.Context, name, description string) (models.GroupWithUsers, error) {
	g, err := c.store.CreateGroup(ctx, repo.CreateGroupParams{
		Name:        name,
		Description: sql.NullString{String: description, Valid: true},
	})
	if err != nil {
		return models.GroupWithUsers{}, fmt.Errorf("could not create group %s: %w", name, err)
	}

	group, err := c.store.GetGroupByUUIDWithUsers(ctx, g.Uuid)
	if err != nil {
		return models.GroupWithUsers{}, fmt.Errorf("could not get group %s with users: %w", name, err)
	}

	return c.repoGroupViewToGroupWithUsers(group)
}

func (c *Core) SearchGroup(ctx context.Context, query string, limit, offset int) ([]models.GroupWithUsers, int64, int64, error) {
	g, err := c.store.SearchGroup(ctx, repo.SearchGroupParams{
		Column1: query,
		Limit: int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, -1, -1, err
	}

	var groups []models.GroupWithUsers
	var pageCount int64
	var totalCount int64

	for i, v := range g {
		groupView := repo.GroupView{
			ID: v.ID,
			Uuid: v.Uuid,
			Name: v.Name,
			Description: v.Description,
			CreatedAt: v.CreatedAt,
			UpdatedAt: v.UpdatedAt,
			Users: v.Users,
		}

		group, err := c.repoGroupViewToGroupWithUsers(groupView)
		if err != nil {
			return nil, -1, -1, err
		}
		groups = append(groups, group)

		if i == 0 {
			pageCount = v.PageCount
			totalCount = v.TotalCount
		}
	}

	return groups, pageCount, totalCount, nil
}

func (c *Core) repoGroupToGroup(group repo.Group) models.Group {
	return models.Group{
		ID:          group.Uuid.String(),
		Name:        group.Name,
		Description: group.Description.String,
	}
}

func (c *Core) repoGroupViewToGroupWithUsers(group repo.GroupView) (models.GroupWithUsers, error) {
	var users []models.User
	if group.Users != nil {
		if err := json.Unmarshal(group.Users.([]byte), &users); err != nil {
			return models.GroupWithUsers{}, fmt.Errorf("could not get users for the group %s: %w", group.Uuid.String(), err)
		}
	}
	g := models.GroupWithUsers{
		Group: models.Group{
			ID:          group.Uuid.String(),
			Name:        group.Name,
			Description: group.Description.String,
		},
		Users: users,
	}

	return g, nil
}
