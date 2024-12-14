package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/cvhariharan/autopilot/internal/flow"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/cvhariharan/autopilot/internal/tasks"
	"github.com/cvhariharan/autopilot/internal/ui"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

type Handler struct {
	flows map[string]flow.Flow
	store repo.Store
	q     *asynq.Client
}

func NewHandler(f map[string]flow.Flow, r repo.Store, q *asynq.Client) *Handler {
	return &Handler{flows: f, store: r, q: q}
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
	task, err := tasks.NewFlowExecution(f, req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error creating task: %v", err))
	}
	info, err := h.q.Enqueue(task)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("error enqueuing task: %v", err))
	}
	log.Println(info.ID)

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
