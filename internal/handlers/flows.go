package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

var (
	upgrader = websocket.Upgrader{}
)

func (h *Handler) HandleFlowTrigger(c echo.Context) error {
	user, ok := c.Get("user").(models.UserInfo)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get user details", nil, nil)
	}

	var req map[string]interface{}
	// This is done to only bind request body and ignore path / query params
	if err := (&echo.DefaultBinder{}).BindBody(c, &req); err != nil {
		return wrapError(http.StatusBadRequest, "could not parse request", err, nil)
	}

	f, err := h.co.GetFlowByID(c.Param("flow"))
	if err != nil {
		return wrapError(http.StatusBadRequest, "could not get flow", err, nil)
	}

	if len(f.Actions) == 0 {
		return wrapError(http.StatusBadRequest, "no actions in flow", nil, nil)
	}

	if err := f.ValidateInput(req); err != nil {
		return wrapError(http.StatusBadRequest, "", err, FlowInputValidationError{
			FieldName:  err.FieldName,
			ErrMessage: err.Msg,
		})
	}

	// Add to queue
	execID, err := h.co.QueueFlowExecution(c.Request().Context(), f, req, user.ID)
	if err != nil {
		return wrapError(http.StatusBadRequest, "could not trigger flow", err, nil)
	}

	c.Response().Header().Set("x-redirect", fmt.Sprintf("%s/view/results/%s/%s", h.appRoot, f.Meta.ID, execID))
	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleLogStreaming(c echo.Context) error {
	// Upgrade to WebSocket connection
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		h.logger.Error("websocket", "error", err)
		return err
	}
	h.logger.Debug("websocket connection created")

	logID := c.Param("logID")
	if logID == "" {
		return ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "execution id cannot be empty"))
	}

	msgCh, err := h.co.StreamLogs(c.Request().Context(), logID)
	if err != nil {
		h.logger.Error("log msg ch", "error", err)
		return ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "error subscribing to logs"))
	}

    for msg := range msgCh {
        if err := h.handleLogStreaming(c, msg, ws); err != nil {
            h.logger.Error("websocket error", "error", err)
        	ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, err.Error()))
         return nil
        }
    }
    c.Logger().Info("msg ch closed")
    return ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "connection closed"))
}

func (h *Handler) handleLogStreaming(c echo.Context, msg models.StreamMessage, ws *websocket.Conn) error {
	var buf bytes.Buffer
	switch msg.MType {
	case models.ResultMessageType:
		var res map[string]string
		if err := json.Unmarshal(msg.Val, &res); err != nil {
			return fmt.Errorf("could not decode results: %w", err)
		}

		if err := json.NewEncoder(&buf).Encode(FlowLogResp{
			MType:   string(msg.MType),
			Results: res,
		}); err != nil {
			return err
		}
	default:
		h.logger.Debug("Default message", "value", string(msg.Val))
		if err := json.NewEncoder(&buf).Encode(FlowLogResp{
			MType: string(msg.MType),
			Value: string(msg.Val),
		}); err != nil {
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
