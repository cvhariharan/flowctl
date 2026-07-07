package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/cvhariharan/flowctl/internal/config"
	"github.com/cvhariharan/flowctl/internal/core"
	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/storage"
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
	storage            storage.Storage
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

	var storeInstance storage.Storage
	if cfg.Artifacts.Retain {
		var err error
		storeInstance, err = storage.NewLocalStorage(cfg.Artifacts.Directory)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize artifacts storage: %w", err)
		}
	}

	h := &Handler{
		co:                 co,
		validate:           validate,
		logger:             logger,
		sessMgr:            sessMgr,
		config:             cfg,
		authconfig:         make(map[string]OIDCAuthConfig),
		executorSigningKey: executorSigningKey,
		version:            version,
		commit:             commit,
		buildDate:          buildDate,
		storage:            storeInstance,
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
	return c.JSON(http.StatusOK, AppInfoResponse{
		Version:         h.version,
		Commit:          h.commit,
		BuildDate:       h.buildDate,
		DefaultTimezone: h.config.Scheduler.DefaultTimezone,
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
