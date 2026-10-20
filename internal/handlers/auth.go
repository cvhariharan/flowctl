package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/cvhariharan/flowctl/internal/config"
	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/labstack/echo/v4"
	"github.com/zerodha/simplesessions/v3"
	"golang.org/x/oauth2"
)

const (
	SessionTimeout     = 2 * time.Hour
	RedirectPath       = "/auth/callback"
	LoginPath          = "/login"
	RedirectAfterLogin = "/"
)

type TokenData struct {
	Provider   string
	RawIDToken string
}

// GetEncoded returns the token data in base64 encoding
func (s *TokenData) GetEncoded() (string, error) {
	j, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("error encoding token data: %w", err)
	}

	return base64.URLEncoding.EncodeToString(j), nil
}

// Decode converts and unmarshals encoded token data
func (s *TokenData) Decode(e string) error {
	j, err := base64.URLEncoding.DecodeString(e)
	if err != nil {
		return fmt.Errorf("error decoding token data: %w", err)
	}

	if err := json.Unmarshal(j, &s); err != nil {
		return fmt.Errorf("invalid state: %w", err)
	}
	return nil
}

type StateData struct {
	Provider string
	Nonce    string
}

// GetEncoded returns the state data in base64 encoding
func (s *StateData) GetEncoded() (string, error) {
	j, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("error encoding state: %w", err)
	}

	return base64.URLEncoding.EncodeToString(j), nil
}

// Decode converts and unmarshals encoded state
func (s *StateData) Decode(e string) error {
	j, err := base64.URLEncoding.DecodeString(e)
	if err != nil {
		return fmt.Errorf("error decoding state: %w", err)
	}

	if err := json.Unmarshal(j, &s); err != nil {
		return fmt.Errorf("invalid state: %w", err)
	}
	return nil
}

// isSafeRedirect determines if the redirect URL is safe
// Must start with '/' but not with '//' or '/\'.
func isSafeRedirect(u string) bool {
	return len(u) > 0 && u[0] == '/' && (len(u) == 1 || (u[1] != '/' && u[1] != '\\'))
}

func (h *Handler) initOIDC() error {
	for _, oauthConfig := range h.config.OIDC {
		provider, err := oidc.NewProvider(context.Background(), oauthConfig.Issuer)
		if err != nil {
			h.logger.Warn("skipping oidc provider initialization",
				"provider", oauthConfig.Name,
				"issuer", oauthConfig.Issuer,
				"error", err)
			continue
		}

		redirectURL, err := url.JoinPath(h.config.App.RootURL, RedirectPath)
		if err != nil {
			h.logger.Warn("skipping oidc provider due to invalid redirect url",
				"provider", oauthConfig.Name,
				"issuer", oauthConfig.Issuer,
				"error", err)
			continue
		}

		if oauthConfig.RedirectURL != "" {
			redirectURL = oauthConfig.RedirectURL
		}

		endpoint := provider.Endpoint()
		if oauthConfig.AuthURL != "" {
			endpoint.AuthURL = oauthConfig.AuthURL
		}
		if oauthConfig.TokenURL != "" {
			endpoint.TokenURL = oauthConfig.TokenURL
		}

		oauth2Config := &oauth2.Config{
			ClientID:     oauthConfig.ClientID,
			ClientSecret: oauthConfig.ClientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     endpoint,
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		}

		verifier := provider.Verifier(&oidc.Config{
			ClientID: oauthConfig.ClientID,
		})

		var discoveryClaims struct {
			EndSessionEndpoint string `json:"end_session_endpoint"`
		}
		if err := provider.Claims(&discoveryClaims); err != nil {
			h.logger.Warn("could not read end_session_endpoint from provider discovery",
				"provider", oauthConfig.Name,
				"error", err)
		}

		endSessionEndpoint := discoveryClaims.EndSessionEndpoint
		if oauthConfig.LogoutURL != "" {
			endSessionEndpoint = oauthConfig.LogoutURL
		}

		h.authconfig[oauthConfig.Name] = OIDCAuthConfig{
			provider:           provider,
			verifier:           verifier,
			oauth2Config:       oauth2Config,
			endSessionEndpoint: endSessionEndpoint,
		}

	}

	return nil
}

func (h *Handler) getOIDCAuthConfig(provider string) (OIDCAuthConfig, bool) {
	authConfig, ok := h.authconfig[provider]
	if !ok || authConfig.oauth2Config == nil || authConfig.verifier == nil {
		return OIDCAuthConfig{}, false
	}
	return authConfig, true
}

func (h *Handler) HandleLoginPage(c echo.Context) error {
	var req AuthReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "invalid request", err, nil)
	}

	sess, err := h.sessMgr.Acquire(nil, c, c)

	if err == simplesessions.ErrInvalidSession {
		sess, err = h.sessMgr.NewSession(c, c)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, err)
		}
	}

	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err)
	}

	if req.Username == "" || req.Password == "" {
		return wrapError(ErrRequiredFieldMissing, "username or password cannot be empty", fmt.Errorf("username or password cannot be empty"), nil)
	}

	user, err := h.co.GetUserByUsernameWithGroups(c.Request().Context(), req.Username)
	if err != nil {
		return wrapError(ErrAuthenticationFailed, "could not authenticate user", err, nil)
	}

	// not using password based login
	if user.LoginType != models.StandardLoginType {
		return wrapError(ErrAuthenticationFailed, "invalid authentication method", fmt.Errorf("invalid authentication method for user: %s", user.ID), nil)
	}

	if err := user.CheckPassword(req.Password); err != nil {
		return wrapError(ErrInvalidCredentials, "invalid credentials", err, nil)
	}

	sess.Set("method", "password")

	var groups []string
	for _, v := range user.Groups {
		groups = append(groups, v.ID)
	}

	sess.Set("user", user.ToUserInfo())

	redirectAfterLogin := RedirectAfterLogin
	if redirectURL := c.QueryParam("redirect_url"); redirectURL != "" && isSafeRedirect(redirectURL) {
		redirectAfterLogin = redirectURL
	}

	c.Response().Header().Set("x-redirect", redirectAfterLogin)
	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleOIDCLogin(c echo.Context) error {
	provider := c.Param("provider")
	if provider == "" {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("oidc provider cannot be empty"))
	}
	authConfig, ok := h.getOIDCAuthConfig(provider)
	if !ok {
		return wrapError(ErrResourceNotFound, "oidc provider is not available", fmt.Errorf("oidc provider is not available: %s", provider), nil)
	}

	sess, err := h.sessMgr.Acquire(nil, c, c)

	if err == simplesessions.ErrInvalidSession {
		sess, err = h.sessMgr.NewSession(c, c)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, err)
		}
	}

	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err)
	}

	nonce, err := generateRandomState()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not generate a login state")
	}

	state := StateData{
		Provider: provider,
		Nonce:    nonce,
	}

	encodedState, err := state.GetEncoded()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	sess.Set("state", encodedState)

	if redirectURL := c.QueryParam("redirect_url"); redirectURL != "" && isSafeRedirect(redirectURL) {
		sess.Set("redirect_url", redirectURL)
	}

	authURL := authConfig.oauth2Config.AuthCodeURL(encodedState)
	return c.Redirect(http.StatusTemporaryRedirect, authURL)
}

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random state: %w", err)
	}

	state := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b)
	return state, nil
}

func (h *Handler) HandleAuthCallback(c echo.Context) error {
	sess, err := h.sessMgr.Acquire(nil, c, c)
	if err != nil {
		return wrapError(ErrInvalidInput, "session does not exist", err, nil)
	}

	rawSessionState, err := sess.Get("state")
	if err != nil {
		return wrapError(ErrInvalidInput, "state not found", err, nil)
	}
	var sessionState StateData
	if err := sessionState.Decode(rawSessionState.(string)); err != nil {
		return wrapError(ErrInternalError, "invalid session state", err, nil)
	}

	var callbackState StateData
	if err := callbackState.Decode(c.QueryParam("state")); err != nil {
		return wrapError(ErrInternalError, "invalid callback state", err, nil)
	}

	if sessionState.Nonce != callbackState.Nonce || sessionState.Provider != callbackState.Provider {
		return wrapError(ErrInvalidInput, "invalid state parameter", nil, nil)
	}
	authConfig, ok := h.getOIDCAuthConfig(sessionState.Provider)
	if !ok {
		return wrapError(ErrResourceNotFound, "oidc provider is not available", fmt.Errorf("oidc provider is not available: %s", sessionState.Provider), nil)
	}

	token, err := authConfig.oauth2Config.Exchange(context.Background(), c.QueryParam("code"))
	if err != nil {
		return wrapError(ErrOperationFailed, "failed to exchange token", err, nil)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return wrapError(ErrOperationFailed, "no id_token in token response", nil, nil)
	}

	idToken, err := authConfig.verifier.Verify(context.Background(), rawIDToken)
	if err != nil {
		return wrapError(ErrOperationFailed, "failed to verify ID token", err, nil)
	}

	var claims struct {
		Email  string   `json:"email"`
		Name   string   `json:"name"`
		Groups []string `json:"groups"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return wrapError(ErrOperationFailed, "failed to parse claims", err, nil)
	}

	user, err := h.co.GetUserByUsernameWithGroups(c.Request().Context(), claims.Email)
	if err != nil {
		user, err = h.autoCreateOIDCUser(c.Request().Context(), sessionState.Provider, claims.Email, claims.Name)
		if err != nil {
			return wrapError(ErrForbidden, err.Error(), err, nil)
		}
	}

	sess.Set("method", "oidc")

	td := TokenData{
		Provider:   sessionState.Provider,
		RawIDToken: rawIDToken,
	}
	tokenData, err := td.GetEncoded()
	if err != nil {
		return wrapError(ErrInternalError, err.Error(), err, nil)
	}

	sess.Set("id_token", tokenData)

	sess.Set("user", user.ToUserInfo())

	redirectAfterLogin := RedirectAfterLogin
	if redirectURL, err := sess.Get("redirect_url"); err == nil && redirectURL != nil {
		if url, ok := redirectURL.(string); ok && isSafeRedirect(url) {
			redirectAfterLogin = url
		}
	}

	return c.Redirect(http.StatusTemporaryRedirect, redirectAfterLogin)
}

func (h *Handler) autoCreateOIDCUser(ctx context.Context, provider, email, claimsName string) (models.UserWithGroups, error) {
	var autoCreate config.OIDCAutoCreateConfig
	for _, oidcCfg := range h.config.OIDC {
		if oidcCfg.Name == provider {
			autoCreate = oidcCfg.AutoCreateUsers
			break
		}
	}

	if !autoCreate.Enabled {
		return models.UserWithGroups{}, fmt.Errorf("auto create users is not enabled for provider: %s", provider)
	}

	if len(autoCreate.AllowedDomains) > 0 {
		emailParts := strings.SplitN(email, "@", 2)
		if len(emailParts) != 2 {
			return models.UserWithGroups{}, fmt.Errorf("invalid email address: %s", email)
		}
		domain := emailParts[1]
		allowed := false
		for _, d := range autoCreate.AllowedDomains {
			if strings.EqualFold(domain, d) {
				allowed = true
				break
			}
		}
		if !allowed {
			return models.UserWithGroups{}, fmt.Errorf("email domain %q is not allowed for provider: %s", domain, provider)
		}
	}

	name := claimsName
	if name == "" {
		name = strings.Split(email, "@")[0]
	}

	var groupIDs []string
	for _, groupName := range autoCreate.Groups {
		g, err := h.co.GetGroupByName(ctx, groupName)
		if err == nil {
			groupIDs = append(groupIDs, g.ID)
		}
	}

	user, err := h.co.CreateUser(ctx, name, email, models.OIDCLoginType, models.StandardUserRole, groupIDs)
	if err != nil {
		return models.UserWithGroups{}, fmt.Errorf("could not create user: %w", err)
	}

	if autoCreate.Namespace != "" || autoCreate.Role != "" {
		namespace := autoCreate.Namespace
		if namespace == "" {
			namespace = "default"
		}
		role := models.NamespaceRole(autoCreate.Role)
		if role == "" {
			role = models.NamespaceRoleUser
		}
		if ns, err := h.co.GetNamespaceByName(ctx, namespace); err == nil {
			h.co.AssignNamespaceRole(ctx, user.ID, "user", ns.ID, role)
		}
	}

	return user, nil
}

func (h *Handler) HandleLogout(c echo.Context) error {
	sess, err := h.sessMgr.Acquire(nil, c, c)
	if err != nil {
		return c.NoContent(http.StatusOK)
	}

	var endSessionURL string

	method, _ := sess.Get("method")
	if method == "oidc" {
		td, err := sess.Get("id_token")
		if err == nil && td != nil {
			var tokenData TokenData
			if err := tokenData.Decode(td.(string)); err == nil {
				authConfig, ok := h.authconfig[tokenData.Provider]
				if ok && authConfig.endSessionEndpoint != "" {
					postLogoutURI, _ := url.JoinPath(h.config.App.RootURL, LoginPath)
					u, err := url.Parse(authConfig.endSessionEndpoint)
					if err == nil {
						params := u.Query()
						params.Set("id_token_hint", tokenData.RawIDToken)
						params.Set("post_logout_redirect_uri", postLogoutURI)
						params.Set("client_id", authConfig.oauth2Config.ClientID)
						u.RawQuery = params.Encode()
						endSessionURL = u.String()
					}
				}
			}
		}
	}

	if err := sess.Destroy(); err != nil {
		return wrapError(ErrInternalError, "could not destroy session", err, nil)
	}

	if endSessionURL != "" {
		c.Response().Header().Set("x-redirect", endSessionURL)
	}

	return c.NoContent(http.StatusOK)
}

// HandleGetCasbinPermissions returns policies for casbin.js
// It doesn't use the subject query param set by the frontend but instead uses the session user
func (h *Handler) HandleGetCasbinPermissions(c echo.Context) error {
	user, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
	}

	p, err := h.co.GetPermissionsForUser(user.ID)
	if err != nil {
		return wrapError(ErrResourceNotFound, "could not get permissions for user", err, nil)
	}

	return c.JSON(http.StatusOK, struct {
		Data string `json:"data"`
	}{
		Data: p,
	})
}

// HandleCheckPermissions evaluates permission checks server-side and returns results.
func (h *Handler) HandleCheckPermissions(c echo.Context) error {
	user, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
	}

	var req struct {
		Permissions []struct {
			Resource string `json:"resource"`
			Action   string `json:"action"`
			Domain   string `json:"domain"`
		} `json:"permissions"`
	}
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrValidationFailed, "invalid request body", err, nil)
	}

	results := make(map[string]bool, len(req.Permissions))
	for _, p := range req.Permissions {
		resource := models.Resource(p.Resource)
		action := models.RBACAction(p.Action)

		if !models.ValidResource(resource) || !models.ValidRBACAction(action) {
			key := p.Domain + ":" + p.Resource + ":" + p.Action
			results[key] = false
			continue
		}

		allowed, err := h.co.CheckPermission(
			c.Request().Context(),
			user.ID,
			p.Domain,
			resource,
			action,
		)
		if err != nil {
			allowed = false
		}
		key := p.Domain + ":" + p.Resource + ":" + p.Action
		results[key] = allowed
	}

	return c.JSON(http.StatusOK, map[string]any{"results": results})
}

func (h *Handler) HandleGetSSOProviders(c echo.Context) error {
	var providers []SSOProvider

	for _, v := range h.config.OIDC {
		if _, ok := h.getOIDCAuthConfig(v.Name); !ok {
			continue
		}

		label := v.Label
		if label == "" {
			label = fmt.Sprintf("Sign in with %s", v.Name)
		}

		providers = append(providers, SSOProvider{
			ID:    v.Name,
			Label: label,
		})
	}

	return c.JSON(http.StatusOK, providers)
}
