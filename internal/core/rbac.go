package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/google/uuid"
)

// InitializeRBACPolicies sets up the base policies for each role
// These policies apply to all namespaces using wildcard "*"
func (c *Core) InitializeRBACPolicies() error {
	// Clear all policies from both memory and Redis
	c.enforcer.ClearPolicy()
	c.enforcer.SavePolicy()

	// User role policies - for all namespaces
	c.enforcer.AddPolicy("role:user", "*", string(models.ResourceFlow), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:user", "*", string(models.ResourceFlow), string(models.RBACActionExecute))
	c.enforcer.AddPolicy("role:user", "*", string(models.ResourceMember), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:user", "*", string(models.ResourceNamespace), string(models.RBACActionView))

	// Reviewer role policies (inherits from user) - for all namespaces
	c.enforcer.AddPolicy("role:reviewer", "*", string(models.ResourceApproval), "*")
	c.enforcer.AddPolicy("role:reviewer", "*", string(models.ResourceExecution), string(models.RBACActionView))

	// Admin role policies - for all namespaces
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceFlow), string(models.RBACActionCreate))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceFlow), string(models.RBACActionUpdate))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceFlow), string(models.RBACActionDelete))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceNode), string(models.RBACActionCreate))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceNode), string(models.RBACActionUpdate))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceNode), string(models.RBACActionDelete))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceCredential), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceCredential), string(models.RBACActionCreate))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceCredential), string(models.RBACActionUpdate))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceCredential), string(models.RBACActionDelete))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceMember), string(models.RBACActionCreate))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceMember), string(models.RBACActionUpdate))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceMember), string(models.RBACActionDelete))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceFlowSecret), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceFlowSecret), string(models.RBACActionCreate))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceFlowSecret), string(models.RBACActionUpdate))
	c.enforcer.AddPolicy("role:admin", "*", string(models.ResourceFlowSecret), string(models.RBACActionDelete))

	// Synchronize user/group role assignments from database
	if err := c.SynchronizePolicies(context.Background()); err != nil {
		return err
	}

	c.enforcer.AddGroupingPolicy("role:reviewer", "role:user", "*")
	c.enforcer.AddGroupingPolicy("role:admin", "role:reviewer", "*")

	return c.enforcer.SavePolicy()
}

// AssignNamespaceRole assigns a role to a user or group in a namespace
func (c *Core) AssignNamespaceRole(ctx context.Context, subjectID string, subjectType string, namespaceID string, role models.NamespaceRole) error {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	subject := fmt.Sprintf("%s:%s", subjectType, subjectID)

	// Add role assignment in database
	if subjectType == "user" {
		userUUID, err := uuid.Parse(subjectID)
		if err != nil {
			return fmt.Errorf("invalid user UUID: %w", err)
		}

		user, err := c.store.GetUserByUUID(ctx, userUUID)
		if err != nil {
			return err
		}

		_, err = c.store.AssignUserNamespaceRole(ctx, repo.AssignUserNamespaceRoleParams{
			SubjectUuid: user.Uuid,
			Uuid:        namespaceUUID,
			Role:        string(role),
		})
		if err != nil {
			return err
		}
	} else if subjectType == "group" {
		groupUUID, err := uuid.Parse(subjectID)
		if err != nil {
			return fmt.Errorf("invalid group UUID: %w", err)
		}

		group, err := c.store.GetGroupByUUID(ctx, groupUUID)
		if err != nil {
			return err
		}

		_, err = c.store.AssignGroupNamespaceRole(ctx, repo.AssignGroupNamespaceRoleParams{
			SubjectUuid: group.Uuid,
			Uuid:        namespaceUUID,
			Role:        string(role),
		})
		if err != nil {
			return err
		}
	}

	// Add Casbin policy
	c.enforcer.AddGroupingPolicy(subject, fmt.Sprintf("role:%s", role), namespaceID)

	return c.enforcer.SavePolicy()
}

// CheckPermission checks if a user has permission to perform an action on a resource in a namespace
func (c *Core) CheckPermission(ctx context.Context, userID string, namespaceID string, resource models.Resource, action models.RBACAction) (bool, error) {
	// Check if user is global admin
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return false, fmt.Errorf("invalid user UUID: %w", err)
	}

	user, err := c.store.GetUserByUUID(ctx, userUUID)
	if err != nil {
		return false, err
	}

	if user.Role == "superuser" {
		return true, nil
	}

	// Check direct user permission
	userSubject := fmt.Sprintf("user:%s", userID)
	allowed, err := c.enforcer.Enforce(userSubject, namespaceID, string(resource), string(action))
	if err != nil {
		return false, err
	}
	if allowed {
		return true, nil
	}

	// Check group permissions
	groups, err := c.store.GetUserGroups(ctx, userUUID)
	if err != nil {
		return false, err
	}

	for _, group := range groups {
		groupSubject := fmt.Sprintf("group:%s", group.Uuid.String())
		allowed, err = c.enforcer.Enforce(groupSubject, namespaceID, string(resource), string(action))
		if err != nil {
			return false, err
		}
		if allowed {
			return true, nil
		}
	}

	return false, nil
}

// GetUserNamespaces returns all namespaces a user has access to with their roles
func (c *Core) GetUserNamespaces(ctx context.Context, userID string) ([]models.NamespaceWithRole, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user UUID: %w", err)
	}

	// Global admin check remains the same
	user, err := c.store.GetUserByUUID(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	if user.Role == "superuser" {
		// Global superusers have admin access to all namespaces
		namespaces, err := c.store.GetAllNamespaces(ctx)
		if err != nil {
			return nil, err
		}

		var result []models.NamespaceWithRole
		for _, ns := range namespaces {
			result = append(result, models.NamespaceWithRole{
				Namespace: models.Namespace{
					ID:   ns.Uuid.String(),
					Name: ns.Name,
				},
				Role: models.NamespaceRoleAdmin,
			})
		}
		return result, nil
	}

	// Get namespaces from new namespace_members table
	rows, err := c.store.GetUserNamespacesWithRoles(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	var result []models.NamespaceWithRole
	for _, row := range rows {
		result = append(result, models.NamespaceWithRole{
			Namespace: models.Namespace{
				ID:   row.Uuid.String(),
				Name: row.Name,
			},
			Role: models.NamespaceRole(row.Role),
		})
	}
	return result, nil
}

// RemoveNamespaceMember removes a user or group from a namespace
func (c *Core) RemoveNamespaceMember(ctx context.Context, membershipID, namespaceID string) error {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	membershipUUID, err := uuid.Parse(membershipID)
	if err != nil {
		return fmt.Errorf("invalid membership UUID: %w", err)
	}

	// Remove from database
	m, err := c.store.RemoveNamespaceMember(ctx, repo.RemoveNamespaceMemberParams{
		Uuid:   namespaceUUID,
		Uuid_2: membershipUUID,
	})
	if err != nil {
		return err
	}

	// Remove from Casbin policies
	subject := fmt.Sprintf("%s:%s", m.SubjectType, m.SubjectUuid.String())
	c.enforcer.RemoveFilteredGroupingPolicy(0, subject, "", namespaceID)

	return c.enforcer.SavePolicy()
}

// GetNamespaceMembers returns all members of a namespace
func (c *Core) GetNamespaceMembers(ctx context.Context, namespaceID string) ([]models.NamespaceMember, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	rows, err := c.store.GetNamespaceMembers(ctx, namespaceUUID)
	if err != nil {
		return nil, err
	}

	var result []models.NamespaceMember
	for _, row := range rows {
		member := models.NamespaceMember{
			ID:          row.Uuid.String(),
			SubjectType: row.SubjectType,
			Role:        models.NamespaceRole(row.Role),
			CreatedAt:   row.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:   row.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}

		// Convert subject_uuid from interface{} to string
		if row.SubjectUuid != nil {
			if uuidVal, ok := row.SubjectUuid.(string); ok {
				member.SubjectID = uuidVal
			}
		}

		if row.SubjectName != nil {
			if nameVal, ok := row.SubjectName.(string); ok {
				if row.SubjectType == "user" {
					member.UserName = &nameVal
				} else {
					member.GroupName = &nameVal
				}
			}
		}

		result = append(result, member)
	}

	return result, nil
}

// GetPermissions returns the casbin policies for the user
func (c *Core) GetPermissionsForUser(userID string) (string, error) {
	policies, _ := c.enforcer.GetPolicy()
	groupingPolicies, _ := c.enforcer.GetGroupingPolicy()

	modelText := c.enforcer.GetModel().ToText()

	// Combine all policies into a single array with type prefixes
	var allPolicies [][]string

	for _, policy := range policies {
		policyWithType := append([]string{"p"}, policy...)
		allPolicies = append(allPolicies, policyWithType)
	}

	for _, grouping := range groupingPolicies {
		groupingWithType := append([]string{"g"}, grouping...)
		allPolicies = append(allPolicies, groupingWithType)
	}

	response := map[string]interface{}{
		"p": allPolicies,
		"g": [][]string{}, // Empty as all policies are in 'p' array, this is a workaround for a bug in casbin.js
		"m": modelText,
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		return "", err
	}

	return string(responseBytes), nil
}

// SynchronizePolicies synchronizes Casbin policies from the namespace_members table
// This ensures that the RBAC policies in Casbin match the role assignments stored in the database
func (c *Core) SynchronizePolicies(ctx context.Context) error {
	members, err := c.store.GetAllNamespaceMembers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get namespace members: %w", err)
	}

	for _, member := range members {
		subject := fmt.Sprintf("%s:%s", member.SubjectType, member.SubjectUuid.String())
		role := fmt.Sprintf("role:%s", member.Role)
		namespaceID := member.NamespaceUuid.String()

		if _, err := c.enforcer.AddGroupingPolicy(subject, role, namespaceID); err != nil {
			return err
		}
	}

	return nil
}
