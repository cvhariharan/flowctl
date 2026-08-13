package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coder/websocket"
	"github.com/cvhariharan/flowctl/internal/core"
	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/labstack/echo/v4"
)

type frameType string

const (
	frameOpen  frameType = "open"
	frameBatch frameType = "batch"
	framePing  frameType = "ping"
	frameEnd   frameType = "end"
)

const (
	protocolVersion             = 1
	batchFlushInterval          = 50 * time.Millisecond
	batchSizeCap                = 256
	websocketPingInterval       = 20 * time.Second
	websocketWriteTimeout       = 10 * time.Second
	websocketReadLimit    int64 = 4 << 10
)

type openFrame struct {
	Type     frameType `json:"type"`
	Protocol int       `json:"protocol"`
	ExecID   string    `json:"exec_id"`
	Replay   bool      `json:"replay"`
}

type batchFrame struct {
	Type     frameType     `json:"type"`
	Messages []FlowLogResp `json:"messages"`
}

type pingFrame struct {
	Type frameType `json:"type"`
}

type endReason string

const (
	endComplete endReason = "complete"
	endTimeout  endReason = "timeout"
)

type endFrame struct {
	Type   frameType `json:"type"`
	Reason endReason `json:"reason"`
}

func (h *Handler) authorizeExecutionAccess(c echo.Context) (models.ExecutionSummary, error) {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return models.ExecutionSummary{}, wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	req := LogStreamingReq{LogID: c.Param("logID")}
	if err := h.validate.Struct(req); err != nil {
		return models.ExecutionSummary{}, wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	execSummary, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), req.LogID, namespace)
	if err != nil {
		return models.ExecutionSummary{}, wrapError(ErrResourceNotFound, "execution not found", err, nil)
	}

	user, err := h.getUserInfo(c)
	if err != nil {
		return models.ExecutionSummary{}, wrapError(ErrForbidden, "could not get user info", err, nil)
	}

	restricted, err := h.isUserOnly(c.Request().Context(), user.ID, namespace)
	if err != nil {
		return models.ExecutionSummary{}, wrapError(ErrOperationFailed, "could not determine user role", err, nil)
	}
	if restricted && execSummary.TriggeredByID != user.ID {
		return models.ExecutionSummary{}, wrapError(ErrForbidden, "insufficient permissions", nil, nil)
	}

	return execSummary, nil
}

func (h *Handler) HandleLogStream(c echo.Context) error {
	execSummary, err := h.authorizeExecutionAccess(c)
	if err != nil {
		return err
	}
	namespace := c.Get("namespace").(string)
	return h.serveLogStream(c, execSummary.ExecID, func(ctx context.Context) (<-chan models.StreamMessage, <-chan core.LogStreamEndReason, error) {
		return h.co.StreamLogs(ctx, execSummary.ExecID, namespace)
	})
}

type logStreamSource func(context.Context) (<-chan models.StreamMessage, <-chan core.LogStreamEndReason, error)

func (h *Handler) serveLogStream(c echo.Context, execID string, source logStreamSource) error {
	conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{
		CompressionMode:    websocket.CompressionDisabled,
		InsecureSkipVerify: false,
	})
	if err != nil {
		return err
	}
	defer conn.CloseNow()
	conn.SetReadLimit(websocketReadLimit)

	ctx, cancel := context.WithCancel(c.Request().Context())
	defer cancel()
	go discardClientFrames(ctx, conn, cancel)

	writeFrame := func(frame any) error {
		payload, err := json.Marshal(frame)
		if err != nil {
			return err
		}
		writeCtx, writeCancel := context.WithTimeout(ctx, websocketWriteTimeout)
		defer writeCancel()
		return conn.Write(writeCtx, websocket.MessageText, payload)
	}
	fail := func(err error) error {
		h.logger.Error("WebSocket log stream failed", "execID", execID, "error", err)
		_ = conn.Close(websocket.StatusInternalError, "log stream failed")
		return nil
	}

	if err := writeFrame(openFrame{Type: frameOpen, Protocol: protocolVersion, ExecID: execID, Replay: true}); err != nil {
		return fail(err)
	}

	msgCh, terminalCh, err := source(ctx)
	if err != nil {
		return fail(err)
	}

	batchTicker := time.NewTicker(batchFlushInterval)
	defer batchTicker.Stop()
	pingTicker := time.NewTicker(websocketPingInterval)
	defer pingTicker.Stop()
	pending := make([]FlowLogResp, 0, batchSizeCap)

	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		messages := append([]FlowLogResp(nil), pending...)
		pending = pending[:0]
		return writeFrame(batchFrame{Type: frameBatch, Messages: messages})
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-batchTicker.C:
			if err := flush(); err != nil {
				return fail(err)
			}
		case <-pingTicker.C:
			pingCtx, pingCancel := context.WithTimeout(ctx, websocketWriteTimeout)
			err := conn.Ping(pingCtx)
			pingCancel()
			if err != nil {
				return nil
			}
			if err := writeFrame(pingFrame{Type: framePing}); err != nil {
				return fail(err)
			}
		case msg, ok := <-msgCh:
			if !ok {
				if err := flush(); err != nil {
					return fail(err)
				}
				reason := endComplete
				terminal, ok := <-terminalCh
				if ok && terminal == core.LogStreamEndError {
					return fail(fmt.Errorf("log reader failed"))
				}
				if ok && terminal == core.LogStreamEndTimeout {
					reason = endTimeout
				}
				if err := writeFrame(endFrame{Type: frameEnd, Reason: reason}); err != nil {
					return fail(err)
				}
				_ = conn.Close(websocket.StatusNormalClosure, string(reason))
				return nil
			}

			response, err := h.flowLogResponse(msg)
			if err != nil {
				return fail(err)
			}
			pending = append(pending, response)
			if len(pending) >= batchSizeCap || msg.MType != models.LogMessageType {
				if err := flush(); err != nil {
					return fail(err)
				}
			}
		}
	}
}

func discardClientFrames(ctx context.Context, conn *websocket.Conn, cancel context.CancelFunc) {
	defer cancel()
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			return
		}
	}
}

func (h *Handler) flowLogResponse(msg models.StreamMessage) (FlowLogResp, error) {
	response := FlowLogResp{
		ActionID:  msg.ActionID,
		MType:     string(msg.MType),
		NodeID:    msg.NodeID,
		Value:     msg.Val,
		Timestamp: msg.Timestamp,
	}
	if msg.MType == models.ResultMessageType {
		if err := json.Unmarshal([]byte(msg.Val), &response.Results); err != nil {
			return FlowLogResp{}, fmt.Errorf("could not decode results: %w", err)
		}
		response.Value = ""
	}
	return response, nil
}
