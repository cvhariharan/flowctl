package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

const (
	maxFileSize = 100 * 1024 * 1024 // 100MB
	tempDirName = "/tmp"
)

var (
	upgrader = websocket.Upgrader{}
)

// convertRequestInputs converts request values from strings to their appropriate types based on flow input definitions
func convertRequestInputs(req map[string]interface{}, flow models.Flow) error {
	for _, input := range flow.Inputs {
		value, exists := req[input.Name]
		if !exists {
			continue
		}

		if strVal, ok := value.(string); ok {
			switch input.Type {
			case models.INPUT_TYPE_NUMBER:
				if strVal == "" {
					// Let validation handle empty required fields
					continue
				}
				// Try to parse as int first, then float
				if intVal, err := strconv.Atoi(strVal); err == nil {
					req[input.Name] = intVal
				} else if floatVal, err := strconv.ParseFloat(strVal, 64); err == nil {
					req[input.Name] = floatVal
				} else {
					return fmt.Errorf("field %s must be a valid number", input.Name)
				}
			case models.INPUT_TYPE_CHECKBOX:
				// Convert string to boolean
				req[input.Name] = strVal == "true"
			case models.INPUT_TYPE_STRING, models.INPUT_TYPE_PASSWORD, models.INPUT_TYPE_FILE, models.INPUT_TYPE_DATETIME, models.INPUT_TYPE_SELECT:
				// Keep as string
				continue
			}
		}
	}
	return nil
}

// processFileUpload handles a single file upload and returns the temporary file path
func (h *Handler) processFileUpload(c echo.Context, input models.Input, namespace, flowID string) (string, error) {
	file, err := c.FormFile(input.Name)
	if err != nil {
		if input.Required {
			return "", fmt.Errorf("file %s is required", input.Name)
		}
		return "", nil
	}

	// Validate file size
	if file.Size > maxFileSize {
		return "", fmt.Errorf("file %s is too large (max %dMB)", input.Name, maxFileSize/(1024*1024))
	}

	// Create secure temporary directory
	tmpDir, err := os.MkdirTemp(tempDirName, fmt.Sprintf("flow_%s_%s_*", flowID, namespace))
	if err != nil {
		return "", fmt.Errorf("could not create temp directory for storing the uploaded file: %w", err)
	}

	// Sanitize filename
	filename := filepath.Base(filepath.Clean(file.Filename))
	if filename == "" || filename == "." || filename == ".." {
		filename = "uploaded_file"
	}
	tmpFilePath := filepath.Join(tmpDir, filename)

	// Save uploaded file
	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(tmpFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to save uploaded file: %w", err)
	}

	return tmpFilePath, nil
}

// processFlowInputs processes all flow inputs from the request and returns a map of input values
func (h *Handler) processFlowInputs(c echo.Context, flow models.Flow, namespace string) (map[string]interface{}, error) {
	req := make(map[string]interface{})

	for _, input := range flow.Inputs {
		switch input.Type {
		case models.INPUT_TYPE_FILE:
			filePath, err := h.processFileUpload(c, input, namespace, flow.Meta.ID)
			if err != nil {
				return nil, err
			}
			if filePath != "" {
				req[input.Name] = filePath
			}
			log.Println(filePath)
		default:
			if value := c.FormValue(input.Name); value != "" {
				req[input.Name] = value
			}
		}
	}

	return req, nil
}

func (h *Handler) HandleFlowTrigger(c echo.Context) error {
	user, ok := c.Get("user").(models.UserInfo)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get user details", nil, nil)
	}

	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	f, err := h.co.GetFlowByID(c.Param("flow"), namespace)
	if err != nil {
		return wrapError(http.StatusBadRequest, "could not get flow", err, nil)
	}

	req, err := h.processFlowInputs(c, f, namespace)
	if err != nil {
		return wrapError(http.StatusBadRequest, "failed to process inputs", err, nil)
	}

	if len(f.Actions) == 0 {
		return wrapError(http.StatusBadRequest, "no actions in flow", nil, nil)
	}

	// Convert string inputs to appropriate types
	if err := convertRequestInputs(req, f); err != nil {
		return wrapError(http.StatusBadRequest, "input conversion error", err, nil)
	}

	if err := f.ValidateInput(req); err != nil {
		return wrapError(http.StatusBadRequest, "", err, FlowInputValidationError{
			FieldName:  err.FieldName,
			ErrMessage: err.Msg,
		})
	}

	// Add to queue
	execID, err := h.co.QueueFlowExecution(c.Request().Context(), f, req, user.ID, namespace)
	if err != nil {
		return wrapError(http.StatusBadRequest, fmt.Sprintf("could not trigger flow: %v", err), err, nil)
	}
	return c.JSON(http.StatusOK, FlowTriggerResp{
		ExecID: execID,
	})
}

func (h *Handler) HandleLogStreaming(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

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

	msgCh, err := h.co.StreamLogs(c.Request().Context(), logID, namespace)
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
			ActionID: msg.ActionID,
			MType:    string(msg.MType),
			Results:  res,
		}); err != nil {
			return err
		}
	default:
		h.logger.Debug("Default message", "value", string(msg.Val))
		if err := json.NewEncoder(&buf).Encode(FlowLogResp{
			ActionID: msg.ActionID,
			MType:    string(msg.MType),
			Value:    string(msg.Val),
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

func (h *Handler) HandleListFlows(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	flows, err := h.co.GetAllFlows(c.Request().Context(), namespace)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not list flows", err, nil)
	}

	response := coreFlowsToFlows(flows)
	return c.JSON(http.StatusOK, response)
}

func (h *Handler) HandleFlowsPagination(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	var req PaginateRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}

	if req.Page < 0 || req.Count < 0 {
		return wrapError(http.StatusInternalServerError, "invalid request, page or count per page cannot be less than 0", fmt.Errorf("page and count per page less than zero"), nil)
	}

	if req.Page > 0 {
		req.Page -= 1
	}

	if req.Count == 0 {
		req.Count = CountPerPage
	}

	flows, pageCount, totalCount, err := h.co.SearchFlows(c.Request().Context(), namespace, req.Filter, req.Count, req.Count*req.Page)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not search flows", err, nil)
	}

	flowItems := make([]FlowListItem, len(flows))
	for i, flow := range flows {
		flowItems[i] = coreFlowToFlow(flow)
	}

	return c.JSON(http.StatusOK, FlowsPaginateResponse{
		Flows:      flowItems,
		PageCount:  pageCount,
		TotalCount: totalCount,
	})
}

func (h *Handler) HandleGetFlow(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	flowID := c.Param("flowID")
	if flowID == "" {
		return wrapError(http.StatusBadRequest, "flow ID cannot be empty", nil, nil)
	}

	flow, err := h.co.GetFlowByID(flowID, namespace)
	if err != nil {
		return wrapError(http.StatusNotFound, "flow not found", err, nil)
	}

	return c.JSON(http.StatusOK, flow)
}

func (h *Handler) HandleGetExecutionSummary(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	execID := c.Param("execID")
	if execID == "" {
		return wrapError(http.StatusBadRequest, "execution ID cannot be empty", nil, nil)
	}

	execSummary, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), execID, namespace)
	if err != nil {
		return wrapError(http.StatusNotFound, "execution not found", err, nil)
	}

	response := coreExecutionSummaryToExecutionSummary(execSummary)
	return c.JSON(http.StatusOK, response)
}

func (h *Handler) HandleExecutionsPagination(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	flowID := c.Param("flowID")
	if flowID == "" {
		return wrapError(http.StatusBadRequest, "flow ID cannot be empty", nil, nil)
	}

	var req PaginateRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}

	if req.Page < 0 || req.Count < 0 {
		return wrapError(http.StatusInternalServerError, "invalid request, page or count per page cannot be less than 0", fmt.Errorf("page and count per page less than zero"), nil)
	}

	if req.Page > 0 {
		req.Page -= 1
	}

	if req.Count == 0 {
		req.Count = CountPerPage
	}

	flow, err := h.co.GetFlowByID(flowID, namespace)
	if err != nil {
		return wrapError(http.StatusNotFound, "flow not found", err, nil)
	}

	executions, pageCount, totalCount, err := h.co.GetExecutionSummaryPaginated(c.Request().Context(), flow, namespace, req.Count, req.Count*req.Page)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get paginated executions", err, nil)
	}

	executionItems := make([]ExecutionSummary, len(executions))
	for i, exec := range executions {
		executionItems[i] = coreExecutionSummaryToExecutionSummary(exec)
	}

	return c.JSON(http.StatusOK, ExecutionsPaginateResponse{
		Executions: executionItems,
		PageCount:  pageCount,
		TotalCount: totalCount,
	})
}

func (h *Handler) HandleAllExecutionsPagination(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	var req PaginateRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(http.StatusInternalServerError, "invalid request", err, nil)
	}

	if req.Page < 0 || req.Count < 0 {
		return wrapError(http.StatusInternalServerError, "invalid request, page or count per page cannot be less than 0", fmt.Errorf("page and count per page less than zero"), nil)
	}

	if req.Page > 0 {
		req.Page -= 1
	}

	if req.Count == 0 {
		req.Count = CountPerPage
	}

	executions, pageCount, totalCount, err := h.co.GetAllExecutionSummaryPaginated(c.Request().Context(), namespace, req.Count, req.Count*req.Page)
	if err != nil {
		return wrapError(http.StatusInternalServerError, "could not get all paginated executions", err, nil)
	}

	executionItems := make([]ExecutionSummary, len(executions))
	for i, exec := range executions {
		executionItems[i] = coreExecutionSummaryToExecutionSummary(exec)
	}

	return c.JSON(http.StatusOK, ExecutionsPaginateResponse{
		Executions: executionItems,
		PageCount:  pageCount,
		TotalCount: totalCount,
	})
}

func (h *Handler) HandleGetFlowInputs(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	flowID := c.Param("flowID")
	if flowID == "" {
		return wrapError(http.StatusBadRequest, "flow ID cannot be empty", nil, nil)
	}

	flow, err := h.co.GetFlowByID(flowID, namespace)
	if err != nil {
		return wrapError(http.StatusNotFound, "flow not found", err, nil)
	}

	inputs := coreFlowInputsToInputs(flow.Inputs)
	return c.JSON(http.StatusOK, FlowInputsResp{
		Inputs: inputs,
	})
}

func (h *Handler) HandleGetFlowMeta(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(http.StatusBadRequest, "could not get namespace", nil, nil)
	}

	flowID := c.Param("flowID")
	if flowID == "" {
		return wrapError(http.StatusBadRequest, "flow ID cannot be empty", nil, nil)
	}

	flow, err := h.co.GetFlowByID(flowID, namespace)
	if err != nil {
		return wrapError(http.StatusNotFound, "flow not found", err, nil)
	}

	meta := coreFlowMetatoFlowMeta(flow.Meta)
	actions := coreFlowActionstoFlowActions(flow.Actions)
	return c.JSON(http.StatusOK, FlowMetaResp{
		Metadata: meta,
		Actions:  actions,
	})
}
