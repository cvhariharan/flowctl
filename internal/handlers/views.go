package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/cvhariharan/autopilot/internal/ui"
	"github.com/cvhariharan/autopilot/internal/ui/partials"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var (
	upgrader = websocket.Upgrader{}
)

func (h *Handler) HandleFlowTrigger(c echo.Context) error {
	user, ok := c.Get("user").(models.UserInfo)
	if !ok {
		return echo.NewHTTPError(http.StatusForbidden, "could not get user details")
	}

	var req map[string]interface{}
	// This is done to only bind request body and ignore path / query params
	if err := (&echo.DefaultBinder{}).BindBody(c, &req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "could not parse request")
	}

	f, err := h.co.GetFlowByID(c.Param("flow"))
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	if len(f.Actions) == 0 {
		return echo.NewHTTPError(http.StatusInternalServerError, "no actions in flow")
	}

	if err := f.ValidateInput(req); err != nil {
		return render(c, ui.FlowInputFormPage(f, "", map[string]string{err.FieldName: err.Msg}, ""), http.StatusOK)
	}

	// Add to queue
	execID, err := h.co.QueueFlowExecution(c.Request().Context(), f, req, user.ID)
	if err != nil {
		c.Logger().Error(err)
		return render(c, partials.InlineError("could not queue flow for execution"), http.StatusInternalServerError)
	}

	c.Response().Header().Set("HX-Redirect", fmt.Sprintf("/view/results/%s/%s", f.Meta.ID, execID))
	return c.NoContent(http.StatusCreated)
}

func (h *Handler) HandleExecTrigger(c echo.Context) error {
	execID := c.Param("execID")
	if execID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "exec ID cannot be empty")
	}

	actionID := c.Param("actionID")
	if actionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "exec ID cannot be empty")
	}

	user, ok := c.Get("user").(models.UserInfo)
	if !ok {
		return echo.NewHTTPError(http.StatusForbidden, "could not get user details")
	}

	exec, err := h.co.GetExecutionByExecID(c.Request().Context(), execID)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "could not get exec details")
	}

	if exec.TriggeredBy != user.ID {
		return echo.NewHTTPError(http.StatusForbidden, "only the person who triggered the exec can resume it")
	}

	if err := h.co.ResumeFlowExecution(c.Request().Context(), execID, actionID, user.ID); err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "could not queue flow for execution")
	}

	return c.NoContent(http.StatusCreated)
}

func (h *Handler) HandleFlowForm(c echo.Context) error {
	flow, err := h.co.GetFlowByID(c.Param("flow"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	return render(c, ui.FlowInputFormPage(flow, "", nil, ""), http.StatusOK)
}

func (h *Handler) HandleFlowsList(c echo.Context) error {
	flows, err := h.co.GetAllFlows()
	if err != nil {
		return showErrorPage(c, http.StatusInternalServerError, err.Error())
	}

	return render(c, ui.FlowsListPage(flows), http.StatusOK)
}

func (h *Handler) HandleFlowExecutionResults(c echo.Context) error {
	user, ok := c.Get("user").(models.UserInfo)
	if !ok {
		return echo.NewHTTPError(http.StatusForbidden, "could not get user details")
	}

	flowID := c.Param("flowID")
	if flowID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "flow id cannot be empty")
	}

	logID := c.Param("logID")
	if logID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "execution id cannot be empty")
	}

	f, err := h.co.GetFlowByID(flowID)
	if err != nil {
		return render(c, partials.InlineError("flow could not be found"), http.StatusNotFound)
	}

	exec, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), logID)
	if err != nil {
		c.Logger().Error(err)
		return render(c, partials.InlineError("could not get execution summary for the given flow"), http.StatusNotFound)
	}

	if exec.TriggeredBy != user.ID {
		return echo.NewHTTPError(http.StatusForbidden, "you are not allowed to view this execution summary")
	}

	return render(c, ui.ResultsPage(f, fmt.Sprintf("/view/logs/%s", logID)), http.StatusOK)
}

func (h *Handler) HandleExecutionSummary(c echo.Context) error {
	user, ok := c.Get("user").(models.UserInfo)
	if !ok {
		return echo.NewHTTPError(http.StatusForbidden, "could not get user details")
	}

	flowID := c.Param("flowID")
	if flowID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "flow id cannot be empty")
	}

	f, err := h.co.GetFlowByID(flowID)
	if err != nil {
		return render(c, partials.InlineError("flow could not be found"), http.StatusNotFound)
	}

	summary, err := h.co.GetAllExecutionSummary(c.Request().Context(), f, user.ID)
	if err != nil {
		c.Logger().Error(err)
		return render(c, partials.InlineError(err.Error()), http.StatusInternalServerError)
	}

	return render(c, ui.ExecutionSummaryPage(f, summary), http.StatusOK)
}

func (h *Handler) HandleLogStreaming(c echo.Context) error {
	// Upgrade to WebSocket connection
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer func() {
		ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(int(http.StateClosed), "Connection closed"))
	}()

	logID := c.Param("logID")
	if logID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "execution id cannot be empty")
	}

	msgCh, err := h.co.StreamLogs(c.Request().Context(), logID)
	if err != nil {
		c.Logger().Error(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "error subscribing to logs")
	}

	for {
		msg, ok := <-msgCh
		if !ok {
			c.Logger().Info("msg ch closed")
			return nil
		}
		if err := h.handleLogStreaming(c, msg, ws); err != nil {
			return err
		}
	}
}

func (h *Handler) handleLogStreaming(c echo.Context, msg models.StreamMessage, ws *websocket.Conn) error {
	var buf bytes.Buffer
	switch msg.MType {
	case models.LogMessageType:
		if err := partials.LogMessage(string(msg.Val)).Render(c.Request().Context(), &buf); err != nil {
			return err
		}
	case models.ResultMessageType:
		var res map[string]string
		if err := json.Unmarshal(msg.Val, &res); err != nil {
			return fmt.Errorf("could not decode results: %w", err)
		}
		if err := partials.ExecutionOutput(res).Render(c.Request().Context(), &buf); err != nil {
			return err
		}
	case models.ErrMessageType:
		if err := partials.InlineError(string(msg.Val)).Render(c.Request().Context(), &buf); err != nil {
			return err
		}
	case models.StateMessageType:
		if err := partials.ApprovalMessage(string(msg.Val)).Render(c.Request().Context(), &buf); err != nil {
			return err
		}
	}

	if buf.Len() > 0 {
		if err := ws.WriteMessage(websocket.TextMessage, buf.Bytes()); err != nil {
			return err
		}
	}

	return nil
}
