package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cvhariharan/autopilot/internal/models"
)

func (c *Core) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	user, err := c.store.GetUserByUsername(ctx, username)
	if err != nil {
		return models.User{}, fmt.Errorf("could not get user %s: %w", username, err)
	}

	var p string
	if user.Password.Valid {
		p = user.Password.String
	}

	return models.User{
		UUID:     user.Uuid.String(),
		Username: user.Username,
		Password: p,
	}, nil
}

func (c *Core) GetAllUsersWithGroups(ctx context.Context) ([]models.User, error) {
	u, err := c.store.GetAllUsersWithGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get users with groups: %w", err)
	}

	var users []models.User
	for _, v := range u {
		var groups []models.Group
		if v.Groups != nil {
			if err := json.Unmarshal(v.Groups.([]byte), &groups); err != nil {
				return nil, fmt.Errorf("could not get groups for the user %s: %w", v.Uuid.String(), err)
			}
		}

		users = append(users, models.User{
			UUID:     v.Uuid.String(),
			Name:     v.Name,
			Username: v.Username,
			Groups:   groups,
		})
	}

	return users, nil
}

func (c *Core) SearchUser(ctx context.Context, query string) ([]models.User, error) {
	g, err := c.store.SearchUser(ctx, query)
	if err != nil {
		return nil, err
	}

	var users []models.User
	for _, v := range g {
		var groups []models.Group
		if v.Groups != nil {
			if err := json.Unmarshal(v.Groups.([]byte), &groups); err != nil {
				return nil, fmt.Errorf("could not get groups for the user %s: %w", v.Uuid.String(), err)
			}
		}
		users = append(users, models.User{
			UUID:     v.Uuid.String(),
			Name:     v.Name,
			Username: v.Username,
			Groups:   groups,
		})
	}

	return users, nil
}
