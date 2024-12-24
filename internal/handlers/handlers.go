package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/cvhariharan/autopilot/internal/core"
	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/cvhariharan/autopilot/internal/ui"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	co       *core.Core
	validate *validator.Validate
}

func NewHandler(co *core.Core) *Handler {
	validate := validator.New()
	validate.RegisterValidation("alphanum_underscore", models.AlphanumericUnderscore)
	validate.RegisterValidation("alphanum_whitespace", models.AlphanumericSpace)

	return &Handler{co: co, validate: validate}
}

func (h *Handler) HandlePing(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}

func render(c echo.Context, component templ.Component, status int) error {
	c.Response().Writer.WriteHeader(status)
	return component.Render(c.Request().Context(), c.Response().Writer)
}

func showErrorPage(c echo.Context, code int, message string) error {
	return ui.ErrorPage(code, message).Render(c.Request().Context(), c.Response().Writer)
}

func ErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	code := http.StatusInternalServerError
	errMsg := "error processing the request"
	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		errMsg = he.Message.(string)
	}

	c.Logger().Error(err)

	if err := showErrorPage(c, code, errMsg); err != nil {
		c.Logger().Error(err)
	}
}

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
