package handlers

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
	"github.com/cvhariharan/autopilot/internal/core"
	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/cvhariharan/autopilot/internal/ui"
	"github.com/cvhariharan/autopilot/internal/ui/partials"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	flows map[string]models.Flow
	co    *core.Core
}

func NewHandler(f map[string]models.Flow, co *core.Core) *Handler {
	return &Handler{flows: f, co: co}
}

func render(c echo.Context, component templ.Component) error {
	return component.Render(c.Request().Context(), c.Response().Writer)
}

func showErrorPage(c echo.Context, code int, message string) error {
	return ui.ErrorPage(code, message).Render(c.Request().Context(), c.Response().Writer)
}

func (h *Handler) HandleFlowTrigger(c echo.Context) error {
	var req map[string]interface{}
	// This is done to only bind request body and ignore path / query params
	if err := (&echo.DefaultBinder{}).BindBody(c, &req); err != nil {
		return showErrorPage(c, http.StatusNotFound, "could not parse request")
	}

	f, ok := h.flows[c.Param("flow")]
	if !ok {
		return render(c, ui.FlowInputFormPage(f, nil, "request flow not found"))
	}

	if err := f.ValidateInput(req); err != nil {
		return render(c, ui.FlowInputFormPage(f, map[string]string{err.FieldName: err.Msg}, ""))
	}

	// Add to queue
	logID := uuid.NewString()
	_, err := h.co.QueueFlowExecution(f, req, logID)
	if err != nil {
		return render(c, ui.FlowInputFormPage(f, nil, err.Error()))
	}

	return partials.LogTerminal(fmt.Sprintf("/api/logs/%s", logID)).Render(c.Request().Context(), c.Response().Writer)
}

func (h *Handler) HandleFlowForm(c echo.Context) error {
	flow, ok := h.flows[c.Param("flow")]
	if !ok {
		return showErrorPage(c, http.StatusNotFound, "requested flow not found")
	}

	return ui.FlowInputFormPage(flow, nil, "").Render(c.Request().Context(), c.Response().Writer)
}
