package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/repo"
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

func (c *Core) SearchUser(ctx context.Context, query string, limit, offset int) ([]models.UserWithGroups, int64, int64, error) {
	u, err := c.store.SearchUsersWithGroups(ctx, repo.SearchUsersWithGroupsParams{
		Column1: query,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, -1, -1, err
	}

	var users []models.UserWithGroups
	var pageCount int64
	var totalCount int64

	for i, v := range u {
		userView := repo.UserView{
			ID:        v.ID,
			Uuid:      v.Uuid,
			Name:      v.Name,
			Username:  v.Username,
			Password:  v.Password,
			LoginType: v.LoginType,
			Role:      v.Role,
			CreatedAt: v.CreatedAt,
			UpdatedAt: v.UpdatedAt,
			Groups:    v.Groups,
		}

		user, err := c.repoUserViewToUserWithGroups(userView)
		if err != nil {
			return nil, -1, -1, err
		}
		users = append(users, user)

		// Set pagination counts from the first result
		if i == 0 {
			pageCount = v.PageCount
			totalCount = v.TotalCount
		}
	}

	return users, pageCount, totalCount, nil
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

func (c *Core) CreateUser(ctx context.Context, name, username string, loginType models.UserLoginType, userRole models.UserRoleType, groups []string) (models.UserWithGroups, error) {
	var ltype repo.UserLoginType
	switch loginType {
	case models.OIDCLoginType:
		ltype = repo.UserLoginTypeOidc
	case models.StandardLoginType:
		ltype = repo.UserLoginTypeStandard
	default:
		return models.UserWithGroups{}, fmt.Errorf("unknown login type")
	}

	var urole repo.UserRoleType
	switch userRole {
	case models.SuperuserUserRole:
		urole = repo.UserRoleTypeSuperuser
	case models.StandardUserRole:
		urole = repo.UserRoleTypeUser
	default:
		return models.UserWithGroups{}, fmt.Errorf("unknown role type")
	}

	u, err := c.store.CreateUser(ctx, repo.CreateUserParams{
		Name:      name,
		Username:  username,
		LoginType: ltype,
		Role:      urole,
	})
	if err != nil {
		return models.UserWithGroups{}, fmt.Errorf("could not create user %s: %w", username, err)
	}

	// Add user to specified groups
	if len(groups) > 0 {
		if err := c.store.OverwriteGroupsForUserTx(ctx, u.Uuid, groups); err != nil {
			return models.UserWithGroups{}, fmt.Errorf("could not assign groups to user %s: %w", username, err)
		}
	}

	// Automatically assign new user to default namespace with user role
	// Skip for superusers as they have access to all namespaces by default
	if userRole != models.SuperuserUserRole {
		defaultNamespace, err := c.GetNamespaceByName(ctx, "default")
		if err != nil {
			return models.UserWithGroups{}, fmt.Errorf("could not get default namespace when creating user %s: %w", username, err)
		}

		err = c.AssignNamespaceRole(ctx, u.Uuid.String(), "user", defaultNamespace.ID, models.NamespaceRoleUser)
		if err != nil {
			return models.UserWithGroups{}, fmt.Errorf("could not assign user %s to default namespace: %w", username, err)
		}
	}

	// Get user with groups to return
	userWithGroups, err := c.store.GetUserByUUIDWithGroups(ctx, u.Uuid)
	if err != nil {
		return models.UserWithGroups{}, fmt.Errorf("could not get created user with groups %s: %w", username, err)
	}

	return c.repoUserViewToUserWithGroups(userWithGroups)
}

func (c *Core) UpdateUser(ctx context.Context, userUUID string, name string, username string, groups []string) (models.UserWithGroups, error) {
	uid, err := uuid.Parse(userUUID)
	if err != nil {
		return models.UserWithGroups{}, fmt.Errorf("user ID should be a UUID: %w", err)
	}

	u, err := c.store.GetUserByUUIDWithGroups(ctx, uid)
	if err != nil {
		return models.UserWithGroups{}, fmt.Errorf("could not get user %s: %w", userUUID, err)
	}

	_, err = c.store.UpdateUserByUUID(ctx, repo.UpdateUserByUUIDParams{
		Uuid:     uid,
		Name:     name,
		Username: username,
	})
	if err != nil {
		return models.UserWithGroups{}, fmt.Errorf("could not update name and username for user %s: %w", userUUID, err)
	}

	if err := c.store.OverwriteGroupsForUserTx(ctx, uid, groups); err != nil {
		return models.UserWithGroups{}, fmt.Errorf("could not overwrite groups for user %s: %w", userUUID, err)
	}

	u, err = c.store.GetUserByUUIDWithGroups(ctx, uid)
	if err != nil {
		return models.UserWithGroups{}, fmt.Errorf("could not get updated user %s: %w", userUUID, err)
	}
	return c.repoUserViewToUserWithGroups(u)
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

// GrantSuperusersAdminAccessToAllNamespaces queries for all users with superuser role
// and adds a grouping policy to them to have admin access to all namespaces
func (c *Core) GrantSuperusersAdminAccessToAllNamespaces(ctx context.Context) error {
	// Get all superusers
	superusers, err := c.store.GetUsersByRole(ctx, repo.UserRoleTypeSuperuser)
	if err != nil {
		return fmt.Errorf("could not get superusers: %w", err)
	}

	// Grant admin access to each superuser for all namespaces using wildcard
	for _, user := range superusers {
		userSubject := fmt.Sprintf("user:%s", user.Uuid.String())
		c.enforcer.AddGroupingPolicy(userSubject, "role:admin", "*")
	}

	return c.enforcer.SavePolicy()
}
