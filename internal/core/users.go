package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/google/uuid"
)

func (c *Core) GetUserByUsername(ctx context.Context, username string) (models.User, error) {
	user, err := c.store.GetUserByUsername(ctx, username)
	if err != nil {
		return models.User{}, fmt.Errorf("could not get user %s: %w", username, err)
	}

	return c.repoUserToUser(user), nil
}

func (c *Core) GetUserByUsernameWithGroups(ctx context.Context, username string) (models.UserWithGroups, error) {
	user, err := c.store.GetUserByUsernameWithGroups(ctx, username)
	if err != nil {
		return models.UserWithGroups{}, fmt.Errorf("could not get user %s: %w", username, err)
	}

	return c.repoUserViewToUserWithGroups(user)
}

func (c *Core) GetAllUsersWithGroups(ctx context.Context) ([]models.UserWithGroups, error) {
	u, err := c.store.GetAllUsersWithGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get users with groups: %w", err)
	}

	var users []models.UserWithGroups
	for _, v := range u {
		user, err := c.repoUserViewToUserWithGroups(v)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

func (c *Core) GetUserWithUUIDWithGroups(ctx context.Context, userUUID string) (models.UserWithGroups, error) {
	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return models.UserWithGroups{}, fmt.Errorf("user ID should be a UUID: %w", err)
	}

	u, err := c.store.GetUserByUUIDWithGroups(ctx, uid)
	if err != nil {
		return models.UserWithGroups{}, fmt.Errorf("could not get users with groups: %w", err)
	}

	return c.repoUserViewToUserWithGroups(u)
}

func (c *Core) SearchUser(ctx context.Context, query string) ([]models.UserWithGroups, error) {
	u, err := c.store.SearchUsersWithGroups(ctx, query)
	if err != nil {
		return nil, err
	}

	var users []models.UserWithGroups
	for _, v := range u {
		user, err := c.repoUserViewToUserWithGroups(v)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
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

	return c.repoUserToUser(u), nil
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

func (c *Core) CreateUser(ctx context.Context, name, username string, loginType models.UserLoginType, userRole models.UserRoleType) (models.User, error) {
	var ltype repo.UserLoginType
	switch loginType {
	case models.OIDCLoginType:
		ltype = repo.UserLoginTypeOidc
	case models.StandardLoginType:
		ltype = repo.UserLoginTypeStandard
	default:
		return models.User{}, fmt.Errorf("unknown login type")
	}

	var urole repo.UserRoleType
	switch userRole {
	case models.AdminUserRole:
		urole = repo.UserRoleTypeAdmin
	case models.StandardUserRole:
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

	return c.repoUserToUser(u), nil
}

func (c *Core) UpdateUser(ctx context.Context, userUUID string, name string, username string, groups []string) (models.UserWithGroups, error) {
	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return models.UserWithGroups{}, fmt.Errorf("user ID should be a UUID: %w", err)
	}

	u, err := c.store.UpdateUserByUUID(ctx, repo.UpdateUserByUUIDParams{
		Uuid:     uid,
		Name:     name,
		Username: username,
	})
	if err != nil {
		return models.UserWithGroups{}, fmt.Errorf("could not update name and username for user %s: %w", userUUID, err)
	}

	for _, group := range groups {
		gid, err := uuid.Parse(group)
		if err != nil {
			return models.UserWithGroups{}, fmt.Errorf("group ID should be a UUID: %w", err)
		}

		if err := c.store.AddGroupToUserByUUID(ctx, repo.AddGroupToUserByUUIDParams{
			UserUuid:  uid,
			GroupUuid: gid,
		}); err != nil {
			return models.UserWithGroups{}, fmt.Errorf("could not add group %s to user %s: %w", group, userUUID, err)
		}
	}

	user, err := c.store.GetUserByUsernameWithGroups(ctx, u.Username)
	if err != nil {
		return models.UserWithGroups{}, fmt.Errorf("could not get updated user %s: %w", userUUID, err)
	}
	return c.repoUserViewToUserWithGroups(user)
}

func (c *Core) repoUserViewToUserWithGroups(user repo.UserView) (models.UserWithGroups, error) {
	var groups []models.Group
	if user.Groups != nil {
		if err := json.Unmarshal(user.Groups.([]byte), &groups); err != nil {
			return models.UserWithGroups{}, fmt.Errorf("could not get groups for the user %s: %w", user.Uuid.String(), err)
		}
	}

	u := models.UserWithGroups{
		User: models.User{
			ID:        user.Uuid.String(),
			Name:      user.Name,
			Username:  user.Username,
			LoginType: models.UserLoginType(user.LoginType),
			Role:      models.UserRoleType(user.Role),
		},
		Groups: groups,
	}

	u.User = u.User.WithPassword(user.Password.String)
	return u, nil
}

func (c *Core) repoUserToUser(user repo.User) models.User {
	u := models.User{
		ID:        user.Uuid.String(),
		Name:      user.Name,
		Username:  user.Username,
		LoginType: models.UserLoginType(user.LoginType),
		Role:      models.UserRoleType(user.Role),
	}

	u = u.WithPassword(user.Password.String)
	return u
}
