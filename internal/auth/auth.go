package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/cvhariharan/autopilot/internal/core"
	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/cvhariharan/autopilot/internal/ui"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
	"github.com/zerodha/simplesessions/stores/postgres/v3"
	"github.com/zerodha/simplesessions/v3"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

const (
	SessionTimeout     = 2 * time.Hour
	RedirectPath       = "/auth/callback"
	LoginPath          = "/login"
	RedirectAfterLogin = "/view/"
)

type OIDCAuthConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	Scopes       []string
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
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

type AuthHandler struct {
	sessMgr    *simplesessions.Manager
	authconfig OIDCAuthConfig
	co         *core.Core
}

func NewAuthHandler(db *sql.DB, co *core.Core, authconfig OIDCAuthConfig) (*AuthHandler, error) {
	sessMgr := simplesessions.New(simplesessions.Options{
		EnableAutoCreate: false,
		Cookie: simplesessions.CookieOptions{
			IsHTTPOnly: true,
			MaxAge:     SessionTimeout,
		},
	})

	sessMgr.SetCookieHooks(getCookie, setCookie)

	sessionStore, err := postgres.New(postgres.Opt{
		TTL: SessionTimeout,
	}, db)
	if err != nil {
		return nil, fmt.Errorf("could not initialize postgres session store: %w", err)
	}

	sessMgr.UseStore(sessionStore)

	go func() {
		if err := sessionStore.Prune(); err != nil {
			log.Printf("error pruning login sessions: %v", err)
		}
		time.Sleep(SessionTimeout / 2)
	}()

	ah := &AuthHandler{sessMgr: sessMgr, co: co}
	if err := ah.initOIDC(authconfig); err != nil {
		return nil, fmt.Errorf("could not initialize OIDC config: %w", err)
	}

	return ah, nil
}

func (h *AuthHandler) initOIDC(authconfig OIDCAuthConfig) error {
	provider, err := oidc.NewProvider(context.Background(), authconfig.Issuer)
	if err != nil {
		return fmt.Errorf("could not initialize new OIDC provider client: %w", err)
	}

	if len(authconfig.Scopes) == 0 {
		authconfig.Scopes = []string{oidc.ScopeOpenID, "profile", "email", "groups"}
	}

	redirectURL := viper.GetString("app.root_url") + RedirectPath

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

func (h *AuthHandler) HandleLoginPage(c echo.Context) error {
	if c.Request().Method == echo.POST {
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

		username := c.FormValue("username")
		password := c.FormValue("password")

		if username == "" || password == "" {
			return render(c, ui.LoginPage("username or password cannot be empty"))
		}

		user, err := h.co.GetUserByUsername(c.Request().Context(), username)
		if err != nil {
			return render(c, ui.LoginPage("could not authenticate user"))
		}

		// not using password based login
		if user.Password == "" {
			return render(c, ui.LoginPage("invalid authentication method"))
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
			return render(c, ui.LoginPage("invalid credentials"))
		}

		sess.Set("method", "password")
		sess.Set("user", models.UserInfo{
			UUID:     user.UUID,
			Username: user.Username,
			Groups:   user.Groups,
		})

		c.Logger().Info("login successful")
		c.Response().Header().Set("HX-Redirect", RedirectAfterLogin)
		return c.NoContent(http.StatusCreated)
	}
	return render(c, ui.LoginPage(""))
}

func (h *AuthHandler) HandleOIDCLogin(c echo.Context) error {
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

func (h *AuthHandler) HandleAuthCallback(c echo.Context) error {
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

	user, err := h.co.GetUserByUsername(c.Request().Context(), claims.Email)
	if err != nil {
		return echo.NewHTTPError(http.StatusForbidden, "user does not exist in autopilot")
	}

	sess.Set("method", "oidc")
	sess.Set("id_token", rawIDToken)
	sess.Set("user", models.UserInfo{
		Username: claims.Email,
		UUID:     user.UUID,
		Groups:   user.Groups,
	})

	redirectURL, err := sess.Get("redirect_after_login")
	if err != nil || redirectURL == nil {
		redirectURL = RedirectAfterLogin
	}

	return c.Redirect(http.StatusTemporaryRedirect, redirectURL.(string))
}

func (h *AuthHandler) handleUnauthenticated(c echo.Context) error {
	sess, err := h.sessMgr.Acquire(nil, c, c)

	if err == simplesessions.ErrInvalidSession {
		sess, err = h.sessMgr.NewSession(c, c)
		if err != nil {
			return err
		}
	}

	sess.Set("redirect_after_login", c.Request().URL.String())

	// For API requests, return 401
	if strings.HasPrefix(c.Request().URL.Path, "/api/") {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	// For web requests, redirect to login page
	return c.Redirect(http.StatusTemporaryRedirect, LoginPath)
}

func render(c echo.Context, component templ.Component) error {
	return component.Render(c.Request().Context(), c.Response().Writer)
}
