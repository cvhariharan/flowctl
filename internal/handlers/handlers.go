package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/cvhariharan/flowctl/internal/config"
	"github.com/cvhariharan/flowctl/internal/core"
	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/zerodha/simplesessions/stores/postgres/v3"
	"github.com/zerodha/simplesessions/v3"
	"golang.org/x/oauth2"
)

const (
	// Pagination count per page
	CountPerPage = 10
)

type OIDCAuthConfig struct {
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
}

type Handler struct {
	co                 *core.Core
	validate           *validator.Validate
	sessMgr            *simplesessions.Manager
	authconfig         map[string]OIDCAuthConfig
	logger             *slog.Logger
	config             config.Config
	executorSigningKey []byte
	version            string
	commit             string
	buildDate          string
	hasThemeCSS        bool
	brandingRoot       *os.Root
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

func NewHandler(logger *slog.Logger, db *sql.DB, co *core.Core, cfg config.Config, executorSigningKey []byte, version, commit, buildDate string) (*Handler, error) {
	validate := validator.New()
	validate.RegisterValidation("alphanum_underscore", models.AlphanumericUnderscore)
	validate.RegisterValidation("alphanum_whitespace", models.AlphanumericSpace)
	validate.RegisterValidation("namespace_name", models.NamespaceName)
	validate.RegisterValidation("no_html", models.NoHTML)

	sessMgr := simplesessions.New(simplesessions.Options{
		EnableAutoCreate: false,
		Cookie: simplesessions.CookieOptions{
			IsHTTPOnly: true,
			SameSite: http.SameSiteLaxMode,
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
		for {
			if err := sessionStore.Prune(); err != nil {
				log.Printf("error pruning login sessions: %v", err)
			}
			time.Sleep(SessionTimeout / 2)
		}
	}()

	brandingRoot, err := openBrandingRoot(cfg.App.Branding.BrandingDir)
	if err != nil {
		return nil, fmt.Errorf("could not open branding directory: %w", err)
	}

	hasThemeCSS := false
	if brandingRoot != nil {
		if _, err := brandingRoot.Stat("theme.css"); err == nil {
			hasThemeCSS = true
		}
	}

	h := &Handler{
		co:                 co,
		validate:           validate,
		sessMgr:            sessMgr,
		authconfig:         make(map[string]OIDCAuthConfig),
		logger:             logger,
		config:             cfg,
		executorSigningKey: executorSigningKey,
		version:            version,
		commit:             commit,
		buildDate:          buildDate,
		hasThemeCSS:        hasThemeCSS,
		brandingRoot:       brandingRoot,
	}
	if err := h.initOIDC(); err != nil {
		return nil, fmt.Errorf("error initializing oidc config: %w", err)
	}
	return h, nil
}

func (h *Handler) HandlePing(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleGetInfo(c echo.Context) error {
	var branding *BrandingInfo
	bc := h.config.App.Branding
	if bc.AppName != "" || bc.LogoURL != "" || bc.LogoLightURL != "" || bc.FaviconURL != "" || bc.BrandingDir != "" {
		branding = &BrandingInfo{
			AppName:      bc.AppName,
			LogoURL:      bc.LogoURL,
			LogoLightURL: bc.LogoLightURL,
			IconURL:      bc.IconURL,
			FaviconURL:   bc.FaviconURL,
			HasThemeCSS:  h.hasThemeCSS,
		}
	}
	return c.JSON(http.StatusOK, AppInfoResponse{
		Version:         h.version,
		Commit:          h.commit,
		BuildDate:       h.buildDate,
		DefaultTimezone: h.config.Scheduler.DefaultTimezone,
		Branding:        branding,
	})
}

func formatValidationErrors(err error) string {
	if err == nil {
		return ""
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return err.Error()
	}

	var errMsgs []string
	for _, e := range validationErrors {
		m := fmt.Sprintf("%s: %s", e.Field(), e.Tag())
		if e.Param() != "" {
			m = fmt.Sprintf("%s=%s", m, e.Param())
		}
		errMsgs = append(errMsgs, m)
	}

	return strings.Join(errMsgs, "; ")
}
