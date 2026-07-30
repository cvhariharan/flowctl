package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/google/uuid"
)

// FlowDomain returns the Casbin domain for a flow based on namespace and prefix.
// Ungrouped flows (empty prefix) use the "_" sentinel.
func FlowDomain(namespaceID, prefix string) string {
	if prefix == "" {
		return "/" + namespaceID + "/_"
	}
	return "/" + namespaceID + "/" + prefix
}

// NamespaceDomain returns the Casbin domain for namespace-level checks.
func NamespaceDomain(namespaceID string) string {
	return "/" + namespaceID + "/*"
}

// InitializeRBACPolicies sets up the base policies for each role.
// Domain-based: "/*" for all namespaces, "/:ns/_" for ungrouped flows only.
func (c *Core) InitializeRBACPolicies() error {
	// Clear all policies from memory
	c.enforcer.ClearPolicy()
	c.enforcer.SavePolicy()

	// User role policies — ungrouped flows only (/:ns/_)
	c.enforcer.AddPolicy("role:user", "/:ns/_", string(models.ResourceFlow), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:user", "/:ns/_", string(models.ResourceFlow), string(models.RBACActionExecute))
	// User role policies — non-flow resources unchanged (/* = all namespaces)
	c.enforcer.AddPolicy("role:user", "/*", string(models.ResourceMember), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:user", "/*", string(models.ResourceNamespace), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:user", "/*", string(models.ResourceExecution), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:user", "/*", string(models.ResourceExecution), string(models.RBACActionUpdate))

	// Operator role policies — same flow visibility as user (ungrouped + explicit prefix grants)
	c.enforcer.AddPolicy("role:operator", "/:ns/_", string(models.ResourceFlow), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:operator", "/:ns/_", string(models.ResourceFlow), string(models.RBACActionExecute))
	// Operator can view ALL executions (not restricted like user)
	c.enforcer.AddPolicy("role:operator", "/*", string(models.ResourceExecution), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:operator", "/*", string(models.ResourceExecution), string(models.RBACActionUpdate))
	c.enforcer.AddPolicy("role:operator", "/*", string(models.ResourceMember), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:operator", "/*", string(models.ResourceNamespace), string(models.RBACActionView))
	// Operator can view the full flow config/definition (read-only)
	c.enforcer.AddPolicy("role:operator", "/*", string(models.ResourceFlow), string(models.RBACActionViewConfig))

	// Reviewer role policies
	c.enforcer.AddPolicy("role:reviewer", "/*", string(models.ResourceFlow), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:reviewer", "/*", string(models.ResourceFlow), string(models.RBACActionExecute))
	c.enforcer.AddPolicy("role:reviewer", "/*", string(models.ResourceApproval), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:reviewer", "/*", string(models.ResourceMember), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:reviewer", "/*", string(models.ResourceApproval), string(models.RBACActionApprove))
	c.enforcer.AddPolicy("role:reviewer", "/*", string(models.ResourceExecution), string(models.RBACActionView))

	// Admin role policies
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceFlow), string(models.RBACActionCreate))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceFlow), string(models.RBACActionUpdate))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceFlow), string(models.RBACActionDelete))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceFlow), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceFlow), string(models.RBACActionExecute))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceExecution), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceExecution), string(models.RBACActionUpdate))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceNode), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceNode), string(models.RBACActionCreate))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceNode), string(models.RBACActionUpdate))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceNode), string(models.RBACActionDelete))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceApproval), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceApproval), string(models.RBACActionApprove))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceCredential), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceCredential), string(models.RBACActionCreate))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceCredential), string(models.RBACActionUpdate))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceCredential), string(models.RBACActionDelete))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceMember), string(models.RBACActionCreate))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceMember), string(models.RBACActionUpdate))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceMember), string(models.RBACActionDelete))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceMember), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceFlowSecret), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceFlowSecret), string(models.RBACActionCreate))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceFlowSecret), string(models.RBACActionUpdate))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceFlowSecret), string(models.RBACActionDelete))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceNamespaceSecret), string(models.RBACActionView))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceNamespaceSecret), string(models.RBACActionCreate))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceNamespaceSecret), string(models.RBACActionUpdate))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceNamespaceSecret), string(models.RBACActionDelete))
	// Admin can view flow config (does not inherit from operator, so must be explicit)
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceFlow), string(models.RBACActionViewConfig))
	c.enforcer.AddPolicy("role:admin", "/*", string(models.ResourceFlow), string(models.RBACActionExport))

	// Synchronize user/group role assignments from database
	if err := c.SynchronizePolicies(context.Background()); err != nil {
		return err
	}

	// Synchronize prefix access policies from database
	if err := c.SynchronizePrefixPolicies(context.Background()); err != nil {
		return err
	}

	// Role hierarchy
	c.enforcer.AddGroupingPolicy("role:operator", "role:user", "/*")
	c.enforcer.AddGroupingPolicy("role:reviewer", "role:user", "/*")
	c.enforcer.AddGroupingPolicy("role:admin", "role:reviewer", "/*")

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
	switch subjectType {
	case "user":
		userUUID, err := uuid.Parse(subjectID)
		if err != nil {
			return fmt.Errorf("invalid user UUID: %w", err)
		}
		_, err = c.store.AssignUserNamespaceRole(ctx, repo.AssignUserNamespaceRoleParams{
			Uuid:   userUUID,
			Uuid_2: namespaceUUID,
			Role:   string(role),
		})
		if err != nil {
			return err
		}
	case "group":
		groupUUID, err := uuid.Parse(subjectID)
		if err != nil {
			return fmt.Errorf("invalid group UUID: %w", err)
		}
		_, err = c.store.AssignGroupNamespaceRole(ctx, repo.AssignGroupNamespaceRoleParams{
			Uuid:   groupUUID,
			Uuid_2: namespaceUUID,
			Role:   string(role),
		})
		if err != nil {
			return err
		}
	}

	// Add Casbin policy with path-based domain
	domain := "/" + namespaceID + "/*"
	c.enforcer.AddGroupingPolicy(subject, fmt.Sprintf("role:%s", role), domain)

	return c.enforcer.SavePolicy()
}

// CheckPermission checks if a user has permission to perform an action on a resource.
// The domain parameter encodes namespace and optional prefix scope.
func (c *Core) CheckPermission(ctx context.Context, userID string, domain string, resource models.Resource, action models.RBACAction) (bool, error) {
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
	allowed, err := c.enforcer.Enforce(userSubject, domain, string(resource), string(action))
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
		allowed, err = c.enforcer.Enforce(groupSubject, domain, string(resource), string(action))
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

	user, err := c.store.GetUserByUUID(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	if user.Role == "superuser" {
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

// UpdateNamespaceMember updates the role of a user or group in a namespace
func (c *Core) UpdateNamespaceMember(ctx context.Context, membershipID, namespaceID string, role models.NamespaceRole) error {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	membershipUUID, err := uuid.Parse(membershipID)
	if err != nil {
		return fmt.Errorf("invalid membership UUID: %w", err)
	}

	oldMember, err := c.store.GetNamespaceMemberByUUID(ctx, repo.GetNamespaceMemberByUUIDParams{
		Uuid:   namespaceUUID,
		Uuid_2: membershipUUID,
	})
	if err != nil {
		return fmt.Errorf("failed to get current member: %w", err)
	}

	m, err := c.store.UpdateNamespaceMember(ctx, repo.UpdateNamespaceMemberParams{
		Uuid:   namespaceUUID,
		Uuid_2: membershipUUID,
		Role:   string(role),
	})
	if err != nil {
		return err
	}

	var subjectID string
	if m.UserID.Valid {
		user, err := c.store.GetUserByID(ctx, m.UserID.Int32)
		if err != nil {
			return err
		}
		subjectID = fmt.Sprintf("user:%s", user.Uuid.String())
	} else if m.GroupID.Valid {
		group, err := c.store.GetGroupByID(ctx, m.GroupID.Int32)
		if err != nil {
			return err
		}
		subjectID = fmt.Sprintf("group:%s", group.Uuid.String())
	}

	domain := "/" + namespaceID + "/*"
	c.enforcer.RemoveFilteredGroupingPolicy(0, subjectID, "", domain)
	c.enforcer.AddGroupingPolicy(subjectID, fmt.Sprintf("role:%s", role), domain)

	// If downgraded from admin/reviewer/operator to user, revoke all prefix access
	if (oldMember.Role == "admin" || oldMember.Role == "reviewer" || oldMember.Role == "operator") && string(role) == "user" {
		err = c.store.RevokeAllMemberPrefixAccess(ctx, repo.RevokeAllMemberPrefixAccessParams{
			Uuid:   namespaceUUID,
			Uuid_2: membershipUUID,
		})
		if err != nil {
			return fmt.Errorf("failed to revoke prefix access on downgrade: %w", err)
		}

		c.removePrefixPolicies(subjectID, namespaceID)
	}

	return c.enforcer.SavePolicy()
}

// removePrefixPolicies removes all Casbin prefix-specific p-policies for a subject in a namespace.
// Prefix policies use domains like /<nsID>/<prefix>, while role groupings use /<nsID>/*
// and ungrouped flows use /<nsID>/_. Only prefix-specific policies are removed.
func (c *Core) removePrefixPolicies(subjectID, namespaceID string) {
	nsPrefix := "/" + namespaceID + "/"
	policies, _ := c.enforcer.GetFilteredPolicy(0, subjectID)
	for _, p := range policies {
		dom := p[1]
		if strings.HasPrefix(dom, nsPrefix) {
			slug := strings.TrimPrefix(dom, nsPrefix)
			if slug != "*" && slug != "_" {
				c.enforcer.RemoveFilteredPolicy(0, subjectID, dom)
			}
		}
	}
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

	m, err := c.store.RemoveNamespaceMember(ctx, repo.RemoveNamespaceMemberParams{
		Uuid:   namespaceUUID,
		Uuid_2: membershipUUID,
	})
	if err != nil {
		return err
	}

	var subjectID string
	if m.UserID.Valid {
		user, err := c.store.GetUserByID(ctx, m.UserID.Int32)
		if err != nil {
			return err
		}
		subjectID = fmt.Sprintf("user:%s", user.Uuid.String())
	} else if m.GroupID.Valid {
		group, err := c.store.GetGroupByID(ctx, m.GroupID.Int32)
		if err != nil {
			return err
		}
		subjectID = fmt.Sprintf("group:%s", group.Uuid.String())
	}
	// Revoke all prefix access rows for this member
	err = c.store.RevokeAllMemberPrefixAccess(ctx, repo.RevokeAllMemberPrefixAccessParams{
		Uuid:   namespaceUUID,
		Uuid_2: membershipUUID,
	})
	if err != nil {
		return fmt.Errorf("failed to revoke prefix access: %w", err)
	}

	if subjectID != "" {
		domain := "/" + namespaceID + "/*"
		c.enforcer.RemoveFilteredGroupingPolicy(0, subjectID, "", domain)
		c.removePrefixPolicies(subjectID, namespaceID)
	}

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
			CreatedAt:   row.CreatedAt.Format(TimeFormat),
			UpdatedAt:   row.UpdatedAt.Format(TimeFormat),
		}

		member.SubjectID = row.SubjectUuid.String()
		member.Name = row.SubjectName

		result = append(result, member)
	}

	return result, nil
}

// GetPermissionsForUser returns the casbin policies for the user
func (c *Core) GetPermissionsForUser(userID string) (string, error) {
	policies, _ := c.enforcer.GetPolicy()
	groupingPolicies, _ := c.enforcer.GetGroupingPolicy()

	modelText := c.enforcer.GetModel().ToText()

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
		"g": [][]string{},
		"m": modelText,
	}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		return "", err
	}

	return string(responseBytes), nil
}

// SynchronizePolicies synchronizes Casbin grouping policies from the namespace_members table
func (c *Core) SynchronizePolicies(ctx context.Context) error {
	members, err := c.store.GetAllNamespaceMembers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get namespace members: %w", err)
	}

	for _, member := range members {
		subject := fmt.Sprintf("%s:%s", member.SubjectType, member.SubjectUuid.String())
		role := fmt.Sprintf("role:%s", member.Role)
		domain := "/" + member.NamespaceUuid.String() + "/*"

		if _, err := c.enforcer.AddGroupingPolicy(subject, role, domain); err != nil {
			return err
		}
	}

	return nil
}

// SynchronizePrefixPolicies reads the prefix_access table and rebuilds Casbin p policies
func (c *Core) SynchronizePrefixPolicies(ctx context.Context) error {
	accesses, err := c.store.GetAllPrefixAccesses(ctx)
	if err != nil {
		return fmt.Errorf("failed to get prefix accesses: %w", err)
	}

	for _, a := range accesses {
		subject := fmt.Sprintf("%s:%s", a.SubjectType, a.SubjectUuid.String())
		dom := "/" + a.NamespaceUuid.String() + "/" + a.Prefix
		c.enforcer.AddPolicy(subject, dom, string(models.ResourceFlow), string(models.RBACActionView))
		c.enforcer.AddPolicy(subject, dom, string(models.ResourceFlow), string(models.RBACActionExecute))
		c.enforcer.AddPolicy(subject, dom, string(models.ResourceExecution), string(models.RBACActionView))
	}

	return nil
}

// AssignPrefixAccess grants a user or group access to a specific prefix (dual write: DB + Casbin)
func (c *Core) AssignPrefixAccess(ctx context.Context, subjectID, subjectType, namespaceID, prefix string) error {
	subjectUUID, err := uuid.Parse(subjectID)
	if err != nil {
		return fmt.Errorf("invalid subject UUID: %w", err)
	}
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	switch subjectType {
	case "user":
		err = c.store.AssignUserPrefixAccess(ctx, repo.AssignUserPrefixAccessParams{
			Uuid:   subjectUUID,
			Uuid_2: namespaceUUID,
			Name:   prefix,
		})
	case "group":
		err = c.store.AssignGroupPrefixAccess(ctx, repo.AssignGroupPrefixAccessParams{
			Uuid:   subjectUUID,
			Uuid_2: namespaceUUID,
			Name:   prefix,
		})
	default:
		return fmt.Errorf("invalid subject type: %s", subjectType)
	}
	if err != nil {
		return fmt.Errorf("failed to assign prefix access: %w", err)
	}

	subject := fmt.Sprintf("%s:%s", subjectType, subjectID)
	dom := "/" + namespaceID + "/" + prefix
	c.enforcer.AddPolicy(subject, dom, string(models.ResourceFlow), string(models.RBACActionView))
	c.enforcer.AddPolicy(subject, dom, string(models.ResourceFlow), string(models.RBACActionExecute))
	c.enforcer.AddPolicy(subject, dom, string(models.ResourceExecution), string(models.RBACActionView))
	return c.enforcer.SavePolicy()
}

// RevokePrefixAccess removes a user or group's access to a specific prefix (dual write: DB + Casbin)
func (c *Core) RevokePrefixAccess(ctx context.Context, subjectID, subjectType, namespaceID, prefix string) error {
	subjectUUID, err := uuid.Parse(subjectID)
	if err != nil {
		return fmt.Errorf("invalid subject UUID: %w", err)
	}
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	switch subjectType {
	case "user":
		err = c.store.RevokeUserPrefixAccess(ctx, repo.RevokeUserPrefixAccessParams{
			Uuid:   subjectUUID,
			Uuid_2: namespaceUUID,
			Name:   prefix,
		})
	case "group":
		err = c.store.RevokeGroupPrefixAccess(ctx, repo.RevokeGroupPrefixAccessParams{
			Uuid:   subjectUUID,
			Uuid_2: namespaceUUID,
			Name:   prefix,
		})
	default:
		return fmt.Errorf("invalid subject type: %s", subjectType)
	}
	if err != nil {
		return fmt.Errorf("failed to revoke prefix access: %w", err)
	}

	subject := fmt.Sprintf("%s:%s", subjectType, subjectID)
	dom := "/" + namespaceID + "/" + prefix
	c.enforcer.RemoveFilteredPolicy(0, subject, dom)
	return c.enforcer.SavePolicy()
}

// getUserPrefixAccess returns the list of prefix names a user can access.
// If hasFullAccess is true, the user can see all flows (superuser/admin/reviewer).
func (c *Core) getUserPrefixAccess(ctx context.Context, userID, namespaceID string) (prefixes []string, hasFullAccess bool, err error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, false, fmt.Errorf("invalid user UUID: %w", err)
	}

	user, err := c.store.GetUserByUUID(ctx, userUUID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get user: %w", err)
	}

	if user.Role == "superuser" {
		return nil, true, nil
	}

	subjects := []string{"user:" + userID}
	groups, err := c.store.GetUserGroups(ctx, userUUID)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get user groups: %w", err)
	}
	for _, g := range groups {
		subjects = append(subjects, "group:"+g.Uuid.String())
	}

	nsPrefix := "/" + namespaceID + "/"
	seen := map[string]bool{}

	for _, subject := range subjects {
		roles := c.enforcer.GetRolesForUserInDomain(subject, "/"+namespaceID+"/*")
		for _, role := range roles {
			if role == "role:admin" || role == "role:reviewer" {
				return nil, true, nil
			}
		}

		policies, _ := c.enforcer.GetFilteredPolicy(0, subject)
		for _, p := range policies {
			dom := p[1]
			if strings.HasPrefix(dom, nsPrefix) && p[2] == string(models.ResourceFlow) && p[3] == string(models.RBACActionView) {
				slug := strings.TrimPrefix(dom, nsPrefix)
				if slug != "" && slug != "_" && slug != "*" && !seen[slug] {
					prefixes = append(prefixes, slug)
					seen[slug] = true
				}
			}
		}
	}
	return prefixes, false, nil
}

// GetAccessibleGroups returns the flow groups (prefixes) in a namespace that the user has access to.
// Superusers and namespace admins/reviewers see all groups; regular users see only their granted prefixes.
func (c *Core) GetAccessibleGroups(ctx context.Context, userID, namespaceID string) ([]models.FlowPrefix, error) {
	_, hasFullAccess, err := c.getUserPrefixAccess(ctx, userID, namespaceID)
	if err != nil {
		return nil, err
	}

	if hasFullAccess {
		return c.GetDistinctPrefixes(ctx, namespaceID)
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user UUID: %w", err)
	}

	names, err := c.store.GetUserAccessiblePrefixes(ctx, repo.GetUserAccessiblePrefixesParams{
		Uuid:   namespaceUUID,
		Uuid_2: userUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user accessible prefixes: %w", err)
	}

	var prefixes []models.FlowPrefix
	for _, name := range names {
		prefixes = append(prefixes, models.FlowPrefix{Name: name})
	}
	return prefixes, nil
}

// GetDistinctPrefixes returns all distinct prefixes in a namespace
func (c *Core) GetDistinctPrefixes(ctx context.Context, namespaceID string) ([]models.FlowPrefix, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	rows, err := c.store.GetDistinctPrefixes(ctx, namespaceUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get distinct prefixes: %w", err)
	}

	var prefixes []models.FlowPrefix
	for _, row := range rows {
		prefixes = append(prefixes, models.FlowPrefix{
			ID:          row.Uuid.String(),
			Name:        row.Name,
			Description: row.Description,
		})
	}
	return prefixes, nil
}

// GetFlowCountByPrefix returns the number of active flows with a given prefix in a namespace
func (c *Core) GetFlowCountByPrefix(ctx context.Context, namespaceID, prefix string) (int64, error) {
	prefixID, err := c.ResolvePrefixID(ctx, prefix, namespaceID)
	if err != nil {
		return 0, err
	}

	count, err := c.store.GetFlowCountByPrefix(ctx, prefixID)
	if err != nil {
		return 0, fmt.Errorf("failed to get flow count by prefix: %w", err)
	}
	return count, nil
}

// GrantPrefixAccessForMember resolves a namespace member and grants prefix access
func (c *Core) GrantPrefixAccessForMember(ctx context.Context, namespaceID, membershipID, prefix string) error {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}
	memberUUID, err := uuid.Parse(membershipID)
	if err != nil {
		return fmt.Errorf("invalid membership UUID: %w", err)
	}

	m, err := c.store.GetNamespaceMemberByUUID(ctx, repo.GetNamespaceMemberByUUIDParams{
		Uuid:   namespaceUUID,
		Uuid_2: memberUUID,
	})
	if err != nil {
		return fmt.Errorf("could not find namespace member: %w", err)
	}

	return c.AssignPrefixAccess(ctx, m.SubjectUuid.String(), m.SubjectType, namespaceID, prefix)
}

// RevokePrefixAccessForMember resolves a namespace member and revokes prefix access
func (c *Core) RevokePrefixAccessForMember(ctx context.Context, namespaceID, membershipID, prefix string) error {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}
	memberUUID, err := uuid.Parse(membershipID)
	if err != nil {
		return fmt.Errorf("invalid membership UUID: %w", err)
	}

	m, err := c.store.GetNamespaceMemberByUUID(ctx, repo.GetNamespaceMemberByUUIDParams{
		Uuid:   namespaceUUID,
		Uuid_2: memberUUID,
	})
	if err != nil {
		return fmt.Errorf("could not find namespace member: %w", err)
	}

	return c.RevokePrefixAccess(ctx, m.SubjectUuid.String(), m.SubjectType, namespaceID, prefix)
}

// GetMemberPrefixes returns the flow prefixes accessible to a specific namespace member.
// Admin/reviewer roles see all groups; user role sees only explicitly granted prefixes.
func (c *Core) GetMemberPrefixes(ctx context.Context, namespaceID, membershipID string) ([]models.FlowPrefix, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}
	memberUUID, err := uuid.Parse(membershipID)
	if err != nil {
		return nil, fmt.Errorf("invalid membership UUID: %w", err)
	}

	m, err := c.store.GetNamespaceMemberByUUID(ctx, repo.GetNamespaceMemberByUUIDParams{
		Uuid:   namespaceUUID,
		Uuid_2: memberUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("could not find namespace member: %w", err)
	}

	// For user subjects, delegate to GetAccessibleGroups which handles role checks
	if m.SubjectType == "user" {
		return c.GetAccessibleGroups(ctx, m.SubjectUuid.String(), namespaceID)
	}

	// Group member: admin/reviewer roles see all groups
	if m.Role == "admin" || m.Role == "reviewer" {
		return c.GetDistinctPrefixes(ctx, namespaceID)
	}

	// Group member with user role: get explicitly granted prefixes
	rows, err := c.store.GetMemberPrefixes(ctx, repo.GetMemberPrefixesParams{
		Uuid:   namespaceUUID,
		Uuid_2: memberUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get member prefixes: %w", err)
	}

	var prefixes []models.FlowPrefix
	for _, row := range rows {
		prefixes = append(prefixes, models.FlowPrefix{Name: row.Prefix})
	}
	return prefixes, nil
}
