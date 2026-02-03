package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/labstack/echo/v4"
)

// APIKeyCreateReq represents a request to create an API key
type APIKeyCreateReq struct {
	Name      string  `json:"name" validate:"required,min=1,max=255"`
	ExpiresIn *string `json:"expires_in,omitempty"` // Optional: "30d", "90d", "1y", "never"
}

// APIKeyResp represents an API key response (without the raw key)
type APIKeyResp struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	KeyPrefix  string  `json:"key_prefix"`
	ExpiresAt  *string `json:"expires_at,omitempty"`
	LastUsedAt *string `json:"last_used_at,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

// APIKeyCreateResp includes the raw key (only returned on creation)
type APIKeyCreateResp struct {
	APIKeyResp
	Key string `json:"key"`
}

// APIKeysListResp is the response for listing API keys
type APIKeysListResp struct {
	APIKeys []APIKeyResp `json:"api_keys"`
}

// HandleCreateAPIKey creates a new API key for the authenticated user
func (h *Handler) HandleCreateAPIKey(c echo.Context) error {
	user, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
	}

	var req APIKeyCreateReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	// Parse expiration
	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn != "never" {
		exp, err := parseExpiresIn(*req.ExpiresIn)
		if err != nil {
			return wrapError(ErrInvalidInput, "invalid expires_in value", err, nil)
		}
		expiresAt = &exp
	}

	apiKey, err := h.co.CreateAPIKey(c.Request().Context(), req.Name, user.ID, expiresAt)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not create API key", err, nil)
	}

	resp := APIKeyCreateResp{
		APIKeyResp: apiKeyToResp(apiKey.APIKey),
		Key:        apiKey.RawKey,
	}

	return c.JSON(http.StatusCreated, resp)
}

// HandleListAPIKeys lists all API keys for the authenticated user
func (h *Handler) HandleListAPIKeys(c echo.Context) error {
	user, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
	}

	apiKeys, err := h.co.ListAPIKeysByUser(c.Request().Context(), user.ID)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not list API keys", err, nil)
	}

	resp := APIKeysListResp{
		APIKeys: make([]APIKeyResp, len(apiKeys)),
	}
	for i, k := range apiKeys {
		resp.APIKeys[i] = apiKeyToResp(k)
	}

	return c.JSON(http.StatusOK, resp)
}

// HandleDeleteAPIKey deletes an API key
func (h *Handler) HandleDeleteAPIKey(c echo.Context) error {
	keyID := c.Param("keyID")
	if keyID == "" {
		return wrapError(ErrRequiredFieldMissing, "key ID cannot be empty", nil, nil)
	}

	if err := h.co.DeleteAPIKey(c.Request().Context(), keyID); err != nil {
		return wrapError(ErrOperationFailed, "could not delete API key", err, nil)
	}

	return c.NoContent(http.StatusOK)
}

// parseExpiresIn parses a duration string like "30d", "90d", "1y"
func parseExpiresIn(s string) (time.Time, error) {
	now := time.Now()

	switch s {
	case "30d":
		return now.AddDate(0, 0, 30), nil
	case "60d":
		return now.AddDate(0, 0, 60), nil
	case "90d":
		return now.AddDate(0, 0, 90), nil
	case "180d":
		return now.AddDate(0, 0, 180), nil
	case "1y":
		return now.AddDate(1, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("invalid duration: %s", s)
	}
}

// apiKeyToResp converts a models.APIKey to APIKeyResp
func apiKeyToResp(k models.APIKey) APIKeyResp {
	resp := APIKeyResp{
		ID:        k.ID,
		Name:      k.Name,
		KeyPrefix: k.KeyPrefix,
		CreatedAt: k.CreatedAt.Format(TimeFormat),
	}

	if k.ExpiresAt != nil {
		exp := k.ExpiresAt.Format(TimeFormat)
		resp.ExpiresAt = &exp
	}

	if k.LastUsedAt != nil {
		lastUsed := k.LastUsedAt.Format(TimeFormat)
		resp.LastUsedAt = &lastUsed
	}

	return resp
}
