package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/labstack/echo/v4"
	"github.com/zerodha/simplesessions/v3"
	"golang.org/x/oauth2"
)

type OIDCAuthConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
	LoginPath    string
}

type UserInfo struct {
	Subject string   `json:"sub"`
	Email   string   `json:"email"`
	Name    string   `json:"name"`
	Groups  []string `json:"groups"`
}

func getCookie(name string, r interface{}) (*http.Cookie, error) {
	rd := r.(echo.Context)
	return rd.Cookie(name)
}

func setCookie(cookie *http.Cookie, w interface{}) error {
	wr := w.(echo.Context)
	wr.SetCookie(cookie)
	return nil
}

func (h *Handler) HandleLogin(c echo.Context) error {
	sess, err := h.sessMgr.Acquire(nil, c, c)

	if err == simplesessions.ErrInvalidSession {
		sess, err = h.sessMgr.NewSession(c, c)
		log.Println(sess.ID())
		if err != nil {
			return err
		}
	}

	state, err := generateRandomState()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not generate a login state")
	}

	log.Println(sess.Set("state", state))

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
		return echo.NewHTTPError(http.StatusBadRequest, "session does not exist")
	}

	state, err := sess.Get("state")
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, "state not found")
	}

	if state.(string) != c.QueryParam("state") {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid state parameter")
	}

	token, err := h.authconfig.oauth2Config.Exchange(context.Background(), c.QueryParam("code"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to exchange token")
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "no id_token in token response")
	}

	idToken, err := h.authconfig.verifier.Verify(context.Background(), rawIDToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to verify ID token")
	}

	var claims struct {
		Email  string   `json:"email"`
		Name   string   `json:"name"`
		Groups []string `json:"groups"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to parse claims")
	}

	sess.Set("id_token", rawIDToken)
	sess.Set("user", UserInfo{
		Subject: idToken.Subject,
		Email:   claims.Email,
		Name:    claims.Name,
		Groups:  claims.Groups,
	})

	redirectURL, err := sess.Get("redirect_after_login")
	if err != nil || redirectURL == nil {
		redirectURL = "/"
	}

	return c.Redirect(http.StatusTemporaryRedirect, redirectURL.(string))
}

func (h *Handler) handleUnauthenticated(c echo.Context) error {
	// Store the current URL for redirect after login
	sess, err := h.sessMgr.Acquire(nil, c, c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "session does not exist redirect")
	}
	sess.Set("redirect_after_login", c.Request().URL.String())

	// For API requests, return 401
	if strings.HasPrefix(c.Request().URL.Path, "/api/") {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	// For web requests, redirect to login page
	return c.Redirect(http.StatusTemporaryRedirect, h.authconfig.LoginPath)
}
