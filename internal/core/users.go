package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/google/uuid"
)

type UserLoginType string
type UserRoleType string

const (
	OIDCLoginType UserLoginType = "oidc"
	// Password based login
	StandardLoginType UserLoginType = "standard"

	AdminUserRole    UserRoleType = "admin"
	StandardUserRole UserRoleType = "user"
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

func (c *Core) GetUserByUUID(ctx context.Context, userUUID string) (models.User, error) {
	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return models.User{}, fmt.Errorf("user ID should be a UUID: %w", err)
	}

	u, err := c.store.GetUserByUUID(ctx, uid)
	if err != nil {
		return models.User{}, fmt.Errorf("could not get user %s: %w", userUUID, err)
	}

	return models.User{
		UUID:     u.Uuid.String(),
		Name:     u.Name,
		Username: u.Username,
		Password: u.Password.String,
	}, nil
}

func (c *Core) DeleteUserByUUID(ctx context.Context, userUUID string) error {
	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return fmt.Errorf("user ID should be a UUID: %w", err)
	}

	if err := c.store.DeleteUserByUUID(ctx, uid); err != nil {
		return fmt.Errorf("could not delete user %s: %w", userUUID, err)
	}

	return nil
}

func (c *Core) CreateUser(ctx context.Context, name, username string, loginType UserLoginType, userRole UserRoleType) (models.User, error) {
	var ltype repo.UserLoginType
	switch loginType {
	case OIDCLoginType:
		ltype = repo.UserLoginTypeOidc
	case StandardLoginType:
		ltype = repo.UserLoginTypeStandard
	default:
		return models.User{}, fmt.Errorf("unknown login type")
	}

	var urole repo.UserRoleType
	switch userRole {
	case AdminUserRole:
		urole = repo.UserRoleTypeAdmin
	case StandardUserRole:
		urole = repo.UserRoleTypeUser
	default:
		return models.User{}, fmt.Errorf("unknown role type")
	}

	u, err := c.store.CreateUser(ctx, repo.CreateUserParams{
		Name:      name,
		Username:  username,
		LoginType: ltype,
		Role:      urole,
	})
	if err != nil {
		return models.User{}, fmt.Errorf("could not create user %s: %w", username, err)
	}

	return models.User{
		UUID:     u.Uuid.String(),
		Name:     name,
		Username: username,
	}, nil
}
