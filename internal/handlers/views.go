package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/cvhariharan/autopilot/internal/flow"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/cvhariharan/autopilot/internal/ui"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	flows map[string]flow.Flow
	store repo.Store
}

func NewHandler(f map[string]flow.Flow, r repo.Store) *Handler {
	return &Handler{flows: f, store: r}
}

func (h *Handler) HandleTrigger(c echo.Context) error {
	var req map[string]interface{}
	// This is done to only bind request body and ignore path / query params
	if err := (&echo.DefaultBinder{}).BindBody(c, &req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "error validating request bind")
	}

	f, ok := h.flows[c.Param("flow")]
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "requested flow not found")
	}

	if err := f.ValidateInput(req); err != nil {
		var ferr *flow.FlowValidationError
		if errors.As(err, &ferr) {
			return ui.Form(f, map[string]string{ferr.FieldName: ferr.Msg}).Render(c.Request().Context(), c.Response().Writer)
		}
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("error validating input: %v", err))
	}

	// Add to queue
	inputBytes, err := json.Marshal(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "error marshaling input to json")
	}
	_, err = h.store.AddToQueue(c.Request().Context(), repo.AddToQueueParams{
		FlowID: f.Meta.DBID,
		Input:  inputBytes,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("could not queue workflow: %v", err))
	}

	return ui.Result(f).Render(c.Request().Context(), c.Response().Writer)
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

	return ui.Form(flow, make(map[string]string)).Render(c.Request().Context(), c.Response().Writer)
}
