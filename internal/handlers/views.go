package handlers

import (
	"bytes"
	"fmt"
	"log"
	"net/http"

	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/cvhariharan/autopilot/internal/ui"
	"github.com/cvhariharan/autopilot/internal/ui/partials"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

var (
	upgrader = websocket.Upgrader{}
)

type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) HandleLoginPage(c echo.Context) error {
	if c.Request().Method == echo.POST {
		var req LoginReq
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "could not parse request")
		}

		if req.Username == "" || req.Password == "" {
			return render(c, ui.LoginPage("username or password cannot be empty"))
		}

		user, err := h.co.GetUserByUsername(c.Request().Context(), req.Username)
		if err != nil {
			return render(c, ui.LoginPage("could not authenticate user"))
		}

		// not using password based login
		if user.Password == "" {
			return render(c, ui.LoginPage("invalid authentication method"))
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			return render(c, ui.LoginPage("invalid credentials"))
		}
	}
	return render(c, ui.LoginPage(""))
}

func (h *Handler) HandleFlowTrigger(c echo.Context) error {
	var req map[string]interface{}
	// This is done to only bind request body and ignore path / query params
	if err := (&echo.DefaultBinder{}).BindBody(c, &req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "could not parse request")
	}

	f, err := h.co.GetFlowByID(c.Param("flow"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	if err := f.ValidateInput(req); err != nil {
		return render(c, ui.FlowInputFormPage(f, "", map[string]string{err.FieldName: err.Msg}, ""))
	}

	user, ok := c.Get("user").(models.UserInfo)
	if !ok {
		return echo.NewHTTPError(http.StatusForbidden, "could not get user details")
	}

	// Add to queue
	execID := uuid.NewString()
	_, err = h.co.QueueFlowExecution(c.Request().Context(), f, req, execID, user.ID)
	if err != nil {
		return render(c, ui.FlowInputFormPage(f, "", nil, err.Error()))
	}

	c.Response().Header().Set("HX-Redirect", fmt.Sprintf("/view/results/%s/%s", f.Meta.ID, execID))
	return c.NoContent(http.StatusCreated)
}

func (h *Handler) HandleFlowForm(c echo.Context) error {
	flow, err := h.co.GetFlowByID(c.Param("flow"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	return ui.FlowInputFormPage(flow, "", nil, "").Render(c.Request().Context(), c.Response().Writer)
}

func (h *Handler) HandleFlowsList(c echo.Context) error {
	flows, err := h.co.GetAllFlows()
	if err != nil {
		return showErrorPage(c, http.StatusInternalServerError, err.Error())
	}

	return render(c, ui.FlowsListPage(flows))
}

func (h *Handler) HandleFlowExecutionResults(c echo.Context) error {
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
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	user, ok := c.Get("user").(models.UserInfo)
	if !ok {
		return echo.NewHTTPError(http.StatusForbidden, "could not get user details")
	}

	exec, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), logID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	if exec.TriggeredBy != user.UUID {
		return echo.NewHTTPError(http.StatusForbidden, "you are not allowed to view this execution summary")
	}

	return render(c, ui.ResultsPage(f, fmt.Sprintf("/view/logs/%s", logID)))
}

func (h *Handler) HandleExecutionSummary(c echo.Context) error {
	flowID := c.Param("flowID")
	if flowID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "flow id cannot be empty")
	}

	f, err := h.co.GetFlowByID(flowID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	user, ok := c.Get("user").(models.UserInfo)
	if !ok {
		return echo.NewHTTPError(http.StatusForbidden, "could not get user details")
	}

	summary, err := h.co.GetAllExecutionSummary(c.Request().Context(), f, user.ID)
	if err != nil {
		return render(c, partials.InlineError(err.Error()))
	}

	return render(c, ui.ExecutionSummaryPage(f, summary))
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

	msgCh := h.co.StreamLogs(c.Request().Context(), logID)
	flow, err := h.co.GetFlowFromLogID(logID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "flow id cannot be empty")
	}

	for msg := range msgCh {
		if msg.Err != "" {
			return renderToWebsocket(c, partials.InlineError(msg.Err), ws)
		}

		var buf bytes.Buffer
		if err := partials.LogMessage(msg.Message).Render(c.Request().Context(), &buf); err != nil {
			return err
		}

		if msg.Checkpoint {
			log.Println("checkpoint received")
			buf = bytes.Buffer{}

			var currentActionIdx int
			var actions []string
			for i, v := range flow.Actions {
				actions = append(actions, v.Name)
				if v.ID == msg.ID {
					currentActionIdx = i
				}
			}
			log.Println(currentActionIdx)

			if err := partials.DottedProgress(actions, currentActionIdx).Render(c.Request().Context(), &buf); err != nil {
				return err
			}

			if msg.Results != nil {
				if err := partials.ExecutionOutput(msg.Results).Render(c.Request().Context(), &buf); err != nil {
					return err
				}
			}
		}
		if err := ws.WriteMessage(websocket.TextMessage, buf.Bytes()); err != nil {
			return err
		}
	}

	return nil
}
