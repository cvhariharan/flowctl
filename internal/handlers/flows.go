package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/labstack/echo/v4"
)

const (
	maxFileSize = 100 * 1024 * 1024 // 100MB
	tempDirName = "/tmp"
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
	user, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
	}

	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	f, err := h.co.GetFlowByID(c.Param("flow"), namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "could not get flow", err, nil)
	}

	req, err := h.processFlowInputs(c, f, namespace)
	if err != nil {
		return wrapError(ErrValidationFailed, err.Error(), err, nil)
	}

	if len(f.Actions) == 0 {
		return wrapError(ErrValidationFailed, "no actions in flow", nil, nil)
	}

	// Convert string inputs to appropriate types
	if err := convertRequestInputs(req, f); err != nil {
		return wrapError(ErrInvalidInput, "input conversion error", err, nil)
	}

	if err := f.ValidateInput(req); err != nil {
		return wrapError(ErrValidationFailed, "", err, FlowInputValidationError{
			FieldName:  err.FieldName,
			ErrMessage: err.Msg,
		})
	}

	// Add to queue
	execID, err := h.co.QueueFlowExecution(c.Request().Context(), f, req, user.ID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, fmt.Sprintf("could not trigger flow: %v", err), err, nil)
	}
	return c.JSON(http.StatusOK, FlowTriggerResp{
		ExecID: execID,
	})
}

func (h *Handler) HandleLogStreaming(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	logID := c.Param("logID")
	if logID == "" {
		return wrapError(ErrRequiredFieldMissing, "execution id cannot be empty", nil, nil)
	}

	// Set SSE headers
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().WriteHeader(http.StatusOK)

	h.logger.Debug("SSE connection created", "logID", logID)

	msgCh, err := h.co.StreamLogs(c.Request().Context(), logID, namespace)
	if err != nil {
		h.logger.Error("log msg ch", "error", err)
		return err
	}

	for msg := range msgCh {
		if err := h.handleLogStreaming(msg, c.Response()); err != nil {
			h.logger.Error("SSE streaming error", "error", err)
			return err
		}
	}

	h.logger.Debug("SSE streaming completed", "logID", logID)
	return nil
}

func (h *Handler) handleLogStreaming(msg models.StreamMessage, w http.ResponseWriter) error {
	var response FlowLogResp

	switch msg.MType {
	case models.ResultMessageType:
		var res map[string]string
		if err := json.Unmarshal(msg.Val, &res); err != nil {
			return fmt.Errorf("could not decode results: %w", err)
		}

		response = FlowLogResp{
			ActionID:  msg.ActionID,
			MType:     string(msg.MType),
			Results:   res,
			NodeID:    msg.NodeID,
			Timestamp: msg.Timestamp,
		}
	default:
		h.logger.Debug("Default message", "type", msg.MType, "value", string(msg.Val))
		response = FlowLogResp{
			ActionID:  msg.ActionID,
			MType:     string(msg.MType),
			NodeID:    msg.NodeID,
			Value:     string(msg.Val),
			Timestamp: msg.Timestamp,
		}
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("could not marshal response: %w", err)
	}

	// Write in SSE format: data: <json>\n\n
	if _, err := fmt.Fprintf(w, "data: %s\n\n", jsonData); err != nil {
		return err
	}

	// Flush to send immediately
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	return nil
}

func (h *Handler) HandleFlowsPagination(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req PaginateRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "invalid request", err, nil)
	}

	if req.Page < 0 || req.Count < 0 {
		return wrapError(ErrInvalidPagination, "invalid request, page or count per page cannot be less than 0", fmt.Errorf("page and count per page less than zero"), nil)
	}

	if req.Page > 0 {
		req.Page -= 1
	}

	if req.Count == 0 {
		req.Count = CountPerPage
	}

	flows, pageCount, totalCount, err := h.co.SearchFlows(c.Request().Context(), namespace, req.Filter, req.Count, req.Count*req.Page)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not search flows", err, nil)
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
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req FlowGetReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	flow, err := h.co.GetFlowByID(req.FlowID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "flow not found", err, nil)
	}

	return c.JSON(http.StatusOK, flow)
}

func (h *Handler) HandleGetExecutionSummary(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req ExecutionGetReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	execSummary, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), req.ExecID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "execution not found", err, nil)
	}

	response := coreExecutionSummaryToExecutionSummary(execSummary)
	return c.JSON(http.StatusOK, response)
}

func (h *Handler) HandleExecutionsPagination(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	flowID := c.Param("flowID")
	if flowID == "" {
		return wrapError(ErrRequiredFieldMissing, "flow ID cannot be empty", nil, nil)
	}

	var req PaginateRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "invalid request", err, nil)
	}

	if req.Page < 0 || req.Count < 0 {
		return wrapError(ErrInvalidPagination, "invalid request, page or count per page cannot be less than 0", fmt.Errorf("page and count per page less than zero"), nil)
	}

	if req.Page > 0 {
		req.Page -= 1
	}

	if req.Count == 0 {
		req.Count = CountPerPage
	}

	flow, err := h.co.GetFlowByID(flowID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "flow not found", err, nil)
	}

	executions, pageCount, totalCount, err := h.co.GetExecutionSummaryPaginated(c.Request().Context(), flow, namespace, req.Count, req.Count*req.Page)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not get paginated executions", err, nil)
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
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req PaginateRequest
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "invalid request", err, nil)
	}

	if req.Page < 0 || req.Count < 0 {
		return wrapError(ErrInvalidPagination, "invalid request, page or count per page cannot be less than 0", fmt.Errorf("page and count per page less than zero"), nil)
	}

	if req.Page > 0 {
		req.Page -= 1
	}

	if req.Count == 0 {
		req.Count = CountPerPage
	}

	executions, pageCount, totalCount, err := h.co.GetAllExecutionSummaryPaginated(c.Request().Context(), namespace, req.Count, req.Count*req.Page)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not get all paginated executions", err, nil)
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
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req FlowGetReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	flow, err := h.co.GetFlowByID(req.FlowID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "flow not found", err, nil)
	}

	inputs := coreFlowInputsToInputs(flow.Inputs)
	return c.JSON(http.StatusOK, FlowInputsResp{
		Inputs: inputs,
	})
}

func (h *Handler) HandleGetFlowMeta(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req FlowGetReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "could not decode request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	flow, err := h.co.GetFlowByID(req.FlowID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "flow not found", err, nil)
	}

	meta := coreFlowMetatoFlowMeta(flow.Meta)
	actions := coreFlowActionstoFlowActions(flow.Actions)
	return c.JSON(http.StatusOK, FlowMetaResp{
		Metadata: meta,
		Actions:  actions,
	})
}

func (h *Handler) HandleCreateFlow(c echo.Context) error {
	namespace := c.Param("namespace")
	namespaceID, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	var req FlowCreateReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "invalid request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	flow := models.Flow{
		Meta: models.Metadata{
			ID:          GenerateSlug(req.Meta.Name),
			Name:        req.Meta.Name,
			Description: req.Meta.Description,
			Schedules:   req.Meta.Schedules,
			Namespace:   namespace,
		},
		Inputs:  convertFlowInputsReqToInputs(req.Inputs),
		Actions: convertFlowActionsReqToActions(req.Actions),
	}

	if err := flow.Validate(); err != nil {
		return wrapError(ErrValidationFailed, "flow validation error", err, nil)
	}

	if err := h.co.CreateFlow(c.Request().Context(), flow, namespaceID); err != nil {
		return wrapError(ErrOperationFailed, err.Error(), err, nil)
	}

	return c.JSON(http.StatusCreated, FlowCreateResp{
		ID: flow.Meta.ID,
	})
}

func (h *Handler) HandleUpdateFlow(c echo.Context) error {
	namespaceID, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}
	flowID := c.Param("flowID")

	var req FlowUpdateReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrInvalidInput, "invalid request", err, nil)
	}

	f, err := h.co.GetFlowByID(flowID, namespaceID)
	if err != nil {
		return wrapError(ErrResourceNotFound, "could not get flow", err, nil)
	}

	// Update the metadata schedules from request
	updatedMeta := f.Meta
	updatedMeta.Schedules = req.Schedules

	flow := models.Flow{
		Meta:    updatedMeta,
		Inputs:  convertFlowInputsReqToInputs(req.Inputs),
		Actions: convertFlowActionsReqToActions(req.Actions),
	}

	if err := flow.Validate(); err != nil {
		return wrapError(ErrValidationFailed, "flow validation error", err, nil)
	}

	if err := h.co.UpdateFlow(c.Request().Context(), flow, namespaceID); err != nil {
		return wrapError(ErrOperationFailed, err.Error(), err, nil)
	}

	return c.JSON(http.StatusOK, FlowCreateResp{
		ID: flow.Meta.ID,
	})
}

func (h *Handler) HandleDeleteFlow(c echo.Context) error {
	namespaceID, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}
	flowID := c.Param("flowID")

	if err := h.co.DeleteFlow(c.Request().Context(), flowID, namespaceID); err != nil {
		return wrapError(ErrOperationFailed, err.Error(), err, nil)
	}

	return c.NoContent(http.StatusOK)
}

func (h *Handler) HandleGetFlowConfig(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	f, err := h.co.GetFlowByID(c.Param("flowID"), namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "could not get flow", err, nil)
	}

	return c.JSON(http.StatusOK, FlowCreateReq{
		Meta: FlowMetaReq{
			Name:        f.Meta.Name,
			Description: f.Meta.Description,
			Schedules:   f.Meta.Schedules,
		},
		Inputs:  convertFlowInputsToInputsReq(f.Inputs),
		Actions: convertFlowActionsToActionsReq(f.Actions),
	})
}

func (h *Handler) HandleCancelExecution(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	execID := c.Param("execID")
	if execID == "" {
		return wrapError(ErrRequiredFieldMissing, "execution ID is required", nil, nil)
	}

	// Verify the execution exists and user has permission to cancel it
	_, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), execID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "execution not found", err, nil)
	}

	err = h.co.CancelFlowExecution(c.Request().Context(), execID)
	if err != nil {
		return wrapError(ErrOperationFailed, "failed to cancel execution", err, nil)
	}

	return c.JSON(http.StatusOK, FlowCancellationResp{
		Message: "Cancellation signal sent",
		ExecID:  execID,
	})
}
