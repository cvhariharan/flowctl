package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/labstack/echo/v4"
	"github.com/zerodha/simplesessions/v3"
	"golang.org/x/oauth2"
)

const (
	SessionTimeout     = 2 * time.Hour
	RedirectPath       = "/auth/callback"
	LoginPath          = "/login"
	RedirectAfterLogin = "/view/default/flows"
)

func (h *Handler) initOIDC(authconfig OIDCAuthConfig) error {
	provider, err := oidc.NewProvider(context.Background(), authconfig.Issuer)
	if err != nil {
		return fmt.Errorf("could not initialize new OIDC provider client: %w", err)
	}

	if len(authconfig.Scopes) == 0 {
		authconfig.Scopes = []string{oidc.ScopeOpenID, "profile", "email", "groups"}
	}

	redirectURL, err := url.JoinPath(h.config.App.RootURL, RedirectPath)
	if err != nil {
		return fmt.Errorf("failed to create redirect URL: %w", err)
	}

	oauth2Config := &oauth2.Config{
		ClientID:     authconfig.ClientID,
		ClientSecret: authconfig.ClientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       authconfig.Scopes,
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: authconfig.ClientID,
	})

	authconfig.provider = provider
	authconfig.verifier = verifier
	authconfig.oauth2Config = oauth2Config

	h.authconfig = authconfig

	return nil
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
	if redirect, _ := sess.String(sess.Get("redirect_after_login")); redirect != "" {
		redirectAfterLogin = redirect
	}

	c.Response().Header().Set("x-redirect", redirectAfterLogin)
	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleOIDCLogin(c echo.Context) error {
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

	state, err := generateRandomState()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not generate a login state")
	}

	sess.Set("state", state)

	authURL := h.authconfig.oauth2Config.AuthCodeURL(state)
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

	state, err := sess.Get("state")
	if err != nil {
		return wrapError(ErrInvalidInput, "state not found", err, nil)
	}

	if state.(string) != c.QueryParam("state") {
		return wrapError(ErrInvalidInput, "invalid state parameter", nil, nil)
	}

	token, err := h.authconfig.oauth2Config.Exchange(context.Background(), c.QueryParam("code"))
	if err != nil {
		return wrapError(ErrOperationFailed, "failed to exchange token", err, nil)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return wrapError(ErrOperationFailed, "no id_token in token response", nil, nil)
	}

	idToken, err := h.authconfig.verifier.Verify(context.Background(), rawIDToken)
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
		return wrapError(ErrForbidden, "user does not exist in flowctl", err, nil)
	}

	sess.Set("method", "oidc")
	sess.Set("id_token", rawIDToken)

	sess.Set("user", user.ToUserInfo())

	redirectURL, err := sess.Get("redirect_after_login")
	if err != nil || redirectURL == nil {
		redirectURL = RedirectAfterLogin
	}

	return c.Redirect(http.StatusTemporaryRedirect, redirectURL.(string))
}

// func (h *Handler) handleUnauthenticated(c echo.Context) error {
// 	sess, err := h.sessMgr.Acquire(nil, c, c)

// 	if err == simplesessions.ErrInvalidSession {
// 		sess, err = h.sessMgr.NewSession(c, c)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	sess.Set("redirect_after_login", c.Request().URL.String())

// 	// For API requests, return 401
// 	if strings.HasPrefix(c.Request().URL.Path, "/api/") {
// 		return wrapError(ErrAuthenticationFailed, "authentication required", nil, nil)
// 	}

// 	// For web requests, redirect to login page
// 	return c.Redirect(http.StatusTemporaryRedirect, LoginPath)
// }

func (h *Handler) HandleLogout(c echo.Context) error {
	sess, err := h.sessMgr.Acquire(nil, c, c)
	if err != nil {
		return c.NoContent(http.StatusOK)
	}

	err = sess.Destroy()
	if err != nil {
		return wrapError(ErrInternalError, "could not destroy session", err, nil)
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
