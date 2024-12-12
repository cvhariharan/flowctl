package handlers

import (
	"fmt"
	"net/http"

	"github.com/cvhariharan/autopilot/internal/flow"
	"github.com/cvhariharan/autopilot/internal/ui"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	flows map[string]flow.Flow
}

func NewHandler(f map[string]flow.Flow) *Handler {
	return &Handler{flows: f}
}

// HandleTrigger responds to API calls with an input.
// Input is of the form name=>value
func (h *Handler) HandleTrigger(c echo.Context) error {
	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error validating request bind")
	}

	flowName := c.Param("flow")
	flow, ok := h.flows[flowName]
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "requested flow not found")
	}

	if err := flow.ValidateInput(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("error validating input: %v", err))
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleForm(c echo.Context) error {
	var req map[string]interface{}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error validating request bind")
	}
	flow, ok := h.flows[c.Param("flow")]
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "requested flow not found")
	}

	return ui.Form(flow.Inputs).Render(c.Request().Context(), c.Response().Writer)
}
