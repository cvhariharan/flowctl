package handlers

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/cvhariharan/flowctl/internal/core"
	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/websocket"
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
	Issuer       string
	ClientID     string
	ClientSecret string
	Scopes       []string
	provider     *oidc.Provider
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
}

type Handler struct {
	co       *core.Core
	validate *validator.Validate
	appRoot  string
	sessMgr    *simplesessions.Manager
	authconfig OIDCAuthConfig
	logger     *slog.Logger
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

func NewHandler(logger *slog.Logger, db *sql.DB, co *core.Core, authconfig OIDCAuthConfig, appRoot string) (*Handler, error) {
	validate := validator.New()
	validate.RegisterValidation("alphanum_underscore", models.AlphanumericUnderscore)
	validate.RegisterValidation("alphanum_whitespace", models.AlphanumericSpace)

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

	h := &Handler{co: co, validate: validate, logger: logger, sessMgr: sessMgr, authconfig: authconfig, appRoot: appRoot}
	if err := h.initOIDC(authconfig); err != nil {
		return nil, fmt.Errorf("error initializing oidc config: %w", err)
	}
	return h, nil
}

func (h *Handler) HandlePing(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}

func render(c echo.Context, component templ.Component, status int) error {
	c.Response().Writer.WriteHeader(status)
	return component.Render(c.Request().Context(), c.Response().Writer)
}

// func showErrorPage(c echo.Context, code int, message string) error {
// 	return ui.ErrorPage(code, message).Render(c.Request().Context(), c.Response().Writer)
// }

// func ErrorHandler(err error, c echo.Context) {
// 	if c.Response().Committed {
// 		return
// 	}

// 	code := http.StatusInternalServerError
// 	errMsg := "error processing the request"
// 	if he, ok := err.(*echo.HTTPError); ok {
// 		code = he.Code
// 		errMsg = he.Message.(string)
// 	}

// 	c.Logger().Error(err)

// 	if err := showErrorPage(c, code, errMsg); err != nil {
// 		c.Logger().Error(err)
// 	}
// }

func renderToWebsocket(c echo.Context, component templ.Component, ws *websocket.Conn) error {
	var buf bytes.Buffer
	if err := component.Render(c.Request().Context(), &buf); err != nil {
		return fmt.Errorf("could not render component: %w", err)
	}

	if err := ws.WriteMessage(websocket.TextMessage, buf.Bytes()); err != nil {
		return fmt.Errorf("could not send to websocket: %w", err)
	}

	return nil
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
		errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", e.Field(), e.Tag()))
	}

	return strings.Join(errMsgs, "; ")
}
