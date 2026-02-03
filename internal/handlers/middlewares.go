package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/labstack/echo/v4"
)

func (h *Handler) Authenticate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Try API key authentication first
		if userInfo, ok := h.tryAPIKeyAuth(c); ok {
			c.Set("user", userInfo)
			return next(c)
		}

		// Fall back to session authentication
		sess, err := h.sessMgr.Acquire(nil, c, c)
		if err != nil {
			return wrapError(ErrAuthenticationFailed, "could not get user session", err, nil)
		}

		user, err := sess.Get("user")
		if err != nil {
			return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
		}

		method, err := sess.String(sess.Get("method"))
		if err != nil {
			return wrapError(ErrAuthenticationFailed, "could not get login method", err, nil)
		}

		// if using oidc, validate the token to check if they have not expired
		if method == "oidc" {
			td, err := sess.Get("id_token")
			if err != nil {
				return wrapError(ErrAuthenticationFailed, "could not get id token", err, nil)
			}
			var tokenData TokenData
			if err := tokenData.Decode(td.(string)); err != nil {
				return wrapError(ErrAuthenticationFailed, "invalid token data", err, nil)
			}

			_, err = h.authconfig[tokenData.Provider].verifier.Verify(context.Background(), tokenData.RawIDToken)
			if err != nil {
				sess.Delete("method")
				sess.Delete("id_token")
				sess.Delete("user")
				return wrapError(ErrAuthenticationFailed, "could not verify id token", err, nil)
			}
		}

		var userInfo models.UserInfo
		userBytes, err := json.Marshal(user)
		if err != nil {
			return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
		}

		if err := json.NewDecoder(bytes.NewBuffer(userBytes)).Decode(&userInfo); err != nil {
			c.Logger().Error(err)
			return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
		}
		c.Set("user", userInfo)

		return next(c)
	}
}

// tryAPIKeyAuth attempts to authenticate using an API key from the Authorization header
// Returns the user info and true if successful, otherwise returns empty and false
func (h *Handler) tryAPIKeyAuth(c echo.Context) (models.UserInfo, bool) {
	// Check Authorization header: Bearer <api_key>
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			apiKey := strings.TrimPrefix(authHeader, "Bearer ")
			if strings.HasPrefix(apiKey, "flowctl_") {
				userInfo, err := h.co.ValidateAPIKey(c.Request().Context(), apiKey)
				if err == nil {
					return userInfo, true
				}
				h.logger.Debug("API key validation failed", "error", err)
			}
		}
	}

	// Check X-API-Key header
	apiKey := c.Request().Header.Get("X-API-Key")
	if apiKey != "" && strings.HasPrefix(apiKey, "flowctl_") {
		userInfo, err := h.co.ValidateAPIKey(c.Request().Context(), apiKey)
		if err == nil {
			return userInfo, true
		}
		h.logger.Debug("API key validation failed", "error", err)
	}

	return models.UserInfo{}, false
}

func (h *Handler) AuthorizeForRole(expectedRole string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userInfo, err := h.getUserInfo(c)
			if err != nil {
				return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
			}

			if userInfo.Role == expectedRole {
				return next(c)
			}

			return wrapError(ErrUnauthorized, "unauthorized", nil, nil)
		}
	}
}

// AuthorizeNamespaceAction checks if a user is allowed to perform an action on the given resource given the namespace
func (h *Handler) AuthorizeNamespaceAction(resource models.Resource, action models.RBACAction) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, err := h.getUserInfo(c)
			if err != nil {
				return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
			}

			namespaceID, ok := c.Get("namespace").(string)
			if !ok {
				return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
			}

			allowed, err := h.co.CheckPermission(c.Request().Context(), user.ID, namespaceID, resource, action)
			if err != nil {
				return wrapError(ErrOperationFailed, "could not check permissions", err, nil)
			}

			if !allowed {
				return wrapError(ErrForbidden, "insufficient permissions", nil, nil)
			}

			return next(c)
		}
	}
}

// AuthorizeNamespaceAdmins checks if a user is an admin in at least one namespace
// This is used for global resources that namespace admins should be able to access
func (h *Handler) AuthorizeNamespaceAdmins() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, err := h.getUserInfo(c)
			if err != nil {
				return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
			}

			// Get all namespaces the user has access to
			namespaces, err := h.co.GetUserNamespaces(c.Request().Context(), user.ID)
			if err != nil {
				return wrapError(ErrOperationFailed, "could not get user namespaces", err, nil)
			}

			// Check if user is admin in any namespace
			for _, ns := range namespaces {
				if ns.Role == models.NamespaceRoleAdmin {
					return next(c)
				}
			}

			return wrapError(ErrForbidden, "insufficient permissions", nil, nil)
		}
	}
}

// AuthorizeAction checks if a user is allowed to perform an action on the resource irrespective of the namespace
func (h *Handler) AuthorizeAction(resource models.Resource, action models.RBACAction) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, err := h.getUserInfo(c)
			if err != nil {
				return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
			}

			allowed, err := h.co.CheckPermission(c.Request().Context(), user.ID, "*", resource, action)
			if err != nil {
				return wrapError(ErrOperationFailed, "could not check permissions", err, nil)
			}

			if !allowed {
				return wrapError(ErrForbidden, "insufficient permissions", nil, nil)
			}

			return next(c)
		}
	}
}

// Updated NamespaceMiddleware for simpler access check
func (h *Handler) NamespaceMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		namespace := c.Param("namespace")
		if namespace == "" {
			return wrapError(ErrRequiredFieldMissing, "namespace cannot be empty", nil, nil)
		}

		ns, err := h.co.GetNamespaceByName(c.Request().Context(), namespace)
		if err != nil {
			return wrapError(ErrResourceNotFound, "could not find namespace", err, nil)
		}

		user, err := h.getUserInfo(c)
		if err != nil {
			return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
		}

		// Basic access check - user must have at least view permission
		hasAccess, err := h.co.CheckPermission(c.Request().Context(), user.ID, ns.ID, models.ResourceFlow, models.RBACActionView)
		if err != nil {
			return wrapError(ErrOperationFailed, "could not check namespace access", err, nil)
		}

		if !hasAccess {
			return wrapError(ErrForbidden, "user does not have access to this namespace", nil, nil)
		}

		c.Set("namespace", ns.ID)
		return next(c)
	}
}

func (h *Handler) getUserInfo(c echo.Context) (models.UserInfo, error) {
	sess, err := h.sessMgr.Acquire(nil, c, c)
	if err != nil {
		return models.UserInfo{}, err
	}

	user, err := sess.Get("user")
	if err != nil {
		return models.UserInfo{}, err
	}

	if user == nil {
		err := fmt.Errorf("user session is empty")
		return models.UserInfo{}, err
	}

	var userInfo models.UserInfo
	userBytes, err := json.Marshal(user)
	if err != nil {
		return models.UserInfo{}, err
	}

	if err := json.NewDecoder(bytes.NewBuffer(userBytes)).Decode(&userInfo); err != nil {
		return models.UserInfo{}, err
	}

	return userInfo, nil
}
