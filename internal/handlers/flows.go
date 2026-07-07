package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const (
	optionsRequestIDHeader = "X-Options-Request-ID"
)

// populateRemoteOptions calls core.PopulateRemoteOptions and handles errors.
// strict=false logs errors and falls back to static options; strict=true returns an HTTP error.
func (h *Handler) populateRemoteOptions(c echo.Context, flow *models.Flow, namespace string, inputVals map[string]interface{}, strict bool) error {
	errs := h.co.PopulateRemoteOptions(c.Request().Context(), flow, namespace, inputVals)
	for inputName, err := range errs {
		if strict {
			return wrapError(ErrOperationFailed, fmt.Sprintf("could not fetch remote options for input %s", inputName), err, nil)
		}
		h.logger.Warn("failed to fetch remote options, using static fallback", "input", inputName, "error", err)
	}
	return nil
}

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

// processFileUpload handles a single file upload and saves it to the artifacts directory
func (h *Handler) processFileUpload(c echo.Context, input models.Input, execID string, globalMaxSize int64) (string, error) {
	file, err := c.FormFile(input.Name)
	if err != nil {
		if input.Required {
			return "", fmt.Errorf("file %s is required", input.Name)
		}
		return "", nil
	}

	maxSize := globalMaxSize
	if input.MaxFileSize > 0 {
		maxSize = input.MaxFileSize
	}

	if file.Size > maxSize {
		return "", fmt.Errorf("file %s exceeds maximum size of %dMB", input.Name, maxSize/(1024*1024))
	}

	artifactDir := filepath.Join(os.TempDir(), fmt.Sprintf("artifacts-store-%s", execID), "uploads")
	if err := os.MkdirAll(artifactDir, 0700); err != nil {
		return "", fmt.Errorf("could not create artifacts directory: %w", err)
	}

	filename := filepath.Base(filepath.Clean(file.Filename))
	if filename == "" || filename == "." || filename == ".." {
		filename = fmt.Sprintf("uploaded_%s", input.Name)
	}

	filename = fmt.Sprintf("%s_%s", input.Name, filename)
	filePath := filepath.Join(artifactDir, filename)

	src, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to save uploaded file: %w", err)
	}

	return filePath, nil
}

// processFlowInputs processes all flow inputs from the request and returns a map of input values
func (h *Handler) processFlowInputs(c echo.Context, flow models.Flow, execID string, globalMaxSize int64) (map[string]interface{}, error) {
	req := make(map[string]interface{})

	for _, input := range flow.Inputs {
		switch input.Type {
		case models.INPUT_TYPE_FILE:
			filePath, err := h.processFileUpload(c, input, execID, globalMaxSize)
			if err != nil {
				return nil, err
			}
			if filePath != "" {
				req[input.Name] = filePath
			}
		case models.INPUT_TYPE_CHECKBOX:
			if c.FormValue(input.Name) != "" {
				req[input.Name] = "true"
			} else {
				req[input.Name] = "false"
			}
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

	// Parse scheduled_at query param if provided
	var scheduledAt *time.Time
	if scheduledAtStr := c.QueryParam("scheduled_at"); scheduledAtStr != "" {
		t, err := time.Parse(time.RFC3339, scheduledAtStr)
		if err != nil {
			return wrapError(ErrValidationFailed, "invalid scheduled_at format, expected RFC3339", err, nil)
		}
		if t.Before(time.Now()) {
			return wrapError(ErrValidationFailed, "scheduled_at must be in the future", nil, nil)
		}
		scheduledAt = &t
	}

	f, err := h.co.GetFlowByID(c.Param("flow"), namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "could not get flow", err, nil)
	}

	if len(f.Actions) == 0 {
		return wrapError(ErrValidationFailed, "no actions in flow", nil, nil)
	}

	execID := uuid.NewString()
	globalMaxSize := h.config.App.MaxFileUploadSize

	req, err := h.processFlowInputs(c, f, execID, globalMaxSize)
	if err != nil {
		return wrapError(ErrValidationFailed, err.Error(), err, nil)
	}

	if err := convertRequestInputs(req, f); err != nil {
		return wrapError(ErrInvalidInput, "input conversion error", err, nil)
	}

	optionsRequestID := c.Request().Header.Get(optionsRequestIDHeader)
	if err := h.co.PrepareAndValidateInputs(c.Request().Context(), &f, namespace, req, optionsRequestID); err != nil {
		return wrapError(ErrValidationFailed, "", err, FlowInputValidationError{
			FieldName:  err.FieldName,
			ErrMessage: err.Msg,
		})
	}

	// Add to queue
	execID, err = h.co.QueueFlowExecutionWithExecID(c.Request().Context(), f, req, user.ID, namespace, execID, scheduledAt)
	if err != nil {
		return wrapError(ErrOperationFailed, fmt.Sprintf("could not trigger flow: %v", err), err, nil)
	}

	resp := FlowTriggerResp{
		ExecID: execID,
	}
	if scheduledAt != nil {
		scheduledAtStr := scheduledAt.Format(TimeFormat)
		resp.ScheduledAt = &scheduledAtStr
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) HandleLogStreaming(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	req := LogStreamingReq{LogID: c.Param("logID")}
	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	logID := req.LogID

	execSummary, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), logID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "execution not found", err, nil)
	}

	streamUser, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrForbidden, "could not get user info", err, nil)
	}

	restricted, err := h.isUserOnly(c.Request().Context(), streamUser.ID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not determine user role", err, nil)
	}
	if restricted && execSummary.TriggeredByID != streamUser.ID {
		return wrapError(ErrForbidden, "insufficient permissions", nil, nil)
	}

	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("X-Accel-Buffering", "no")
	c.Response().WriteHeader(http.StatusOK)

	h.logger.Debug("SSE connection created", "logID", logID)

	msgCh, err := h.co.StreamLogs(c.Request().Context(), logID, namespace)
	if err != nil {
		h.logger.Error("log msg ch", "error", err)
		return err
	}

	heartbeatTicker := time.NewTicker(5 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-c.Request().Context().Done():
			h.logger.Debug("SSE client disconnected", "logID", logID)
			return nil
		case <-heartbeatTicker.C:
			if _, err := fmt.Fprintf(c.Response(), ": heartbeat\n\n"); err != nil {
				h.logger.Error("SSE heartbeat error", "error", err, "logID", logID)
				return nil
			}
			if flusher, ok := c.Response().Unwrap().(http.Flusher); ok {
				flusher.Flush()
			}
		case msg, ok := <-msgCh:
			if !ok {
				h.logger.Debug("SSE message channel closed", "logID", logID)
				if _, err := fmt.Fprintf(c.Response(), "event: end\ndata: {}\n\n"); err != nil {
					h.logger.Error("SSE end event error", "error", err)
					return err
				}
				if flusher, ok := c.Response().Unwrap().(http.Flusher); ok {
					flusher.Flush()
				}
				h.logger.Debug("SSE streaming completed", "logID", logID)
				return nil
			}
			if err := h.handleLogStreaming(msg, c.Response()); err != nil {
				h.logger.Error("SSE streaming error", "error", err, "logID", logID)
				return nil
			}
		}
	}
}

func (h *Handler) HandleLogDownload(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	req := LogStreamingReq{LogID: c.Param("logID")}
	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	logID := req.LogID

	execSummary, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), logID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "execution not found", err, nil)
	}

	downloadUser, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrForbidden, "could not get user info", err, nil)
	}

	restricted, err := h.isUserOnly(c.Request().Context(), downloadUser.ID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not determine user role", err, nil)
	}
	if restricted && execSummary.TriggeredByID != downloadUser.ID {
		return wrapError(ErrForbidden, "insufficient permissions", nil, nil)
	}

	c.Response().Header().Set("Content-Type", "application/octet-stream")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.log"`, logID))
	c.Response().WriteHeader(http.StatusOK)

	if err := h.co.DownloadLogs(c.Request().Context(), logID, namespace, c.Response()); err != nil {
		h.logger.Error("log download error", "logID", logID, "error", err)
		return err
	}

	return nil
}

func (h *Handler) handleLogStreaming(msg models.StreamMessage, w http.ResponseWriter) error {
	var response FlowLogResp

	switch msg.MType {
	case models.ResultMessageType:
		var res map[string]string
		if err := json.Unmarshal([]byte(msg.Val), &res); err != nil {
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
		h.logger.Debug("Default message", "type", msg.MType, "value", msg.Val)
		response = FlowLogResp{
			ActionID:  msg.ActionID,
			MType:     string(msg.MType),
			NodeID:    msg.NodeID,
			Value:     msg.Val,
			Timestamp: msg.Timestamp,
		}
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("could not marshal response: %w", err)
	}

	if _, err := fmt.Fprintf(w, "data: %s\n\n", jsonData); err != nil {
		return err
	}

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

	user, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
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

	flows, pageCount, totalCount, err := h.co.SearchFlows(c.Request().Context(), namespace, user.ID, req.Filter, req.Count, req.Count*req.Page)
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

	req := FlowGetReq{FlowID: c.Param("flowID")}
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

	req := ExecutionGetReq{ExecID: c.Param("execID")}
	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	execSummary, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), req.ExecID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "execution not found", err, nil)
	}

	userInfo, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrForbidden, "could not get user info", err, nil)
	}

	restricted, err := h.isUserOnly(c.Request().Context(), userInfo.ID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not determine user role", err, nil)
	}
	if restricted && execSummary.TriggeredByID != userInfo.ID {
		return wrapError(ErrForbidden, "insufficient permissions", nil, nil)
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

	userInfo, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrForbidden, "could not get user info", err, nil)
	}

	flow, err := h.co.GetFlowByID(flowID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "flow not found", err, nil)
	}

	executions, pageCount, totalCount, err := h.co.GetExecutionSummaryPaginated(c.Request().Context(), flow, namespace, userInfo.ID, req.Count, req.Count*req.Page)
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

	userInfo, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrForbidden, "could not get user info", err, nil)
	}

	executions, pageCount, totalCount, err := h.co.GetAllExecutionSummaryPaginated(c.Request().Context(), namespace, userInfo.ID, req.Filter, req.Count, req.Count*req.Page)
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

	req := FlowGetReq{FlowID: c.Param("flowID")}
	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	flow, err := h.co.GetFlowByID(req.FlowID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "flow not found", err, nil)
	}

	h.logger.Debug("flow input", "input", fmt.Sprintf("%+v", flow.Inputs))

	if err := h.populateRemoteOptions(c, &flow, namespace, nil, false); err != nil {
		return err
	}

	// Cache resolved options so HandleFlowTrigger can validate without re-fetching.
	var optionsRequestID string
	resolved := make(map[string][]string)
	for _, input := range flow.Inputs {
		if input.Type == models.INPUT_TYPE_SELECT && input.RemoteOptions != nil {
			resolved[input.Name] = input.Options
		}
	}
	if len(resolved) > 0 {
		optionsRequestID = h.co.StoreRemoteOptionsCache(resolved)
	}

	inputs := coreFlowInputsToInputs(flow.Inputs)
	return c.JSON(http.StatusOK, FlowInputsResp{
		Inputs:           inputs,
		OptionsRequestID: optionsRequestID,
	})
}

func (h *Handler) HandleGetFlowMeta(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	req := FlowGetReq{FlowID: c.Param("flowID")}
	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	flow, err := h.co.GetFlowByID(req.FlowID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "flow not found", err, nil)
	}

	// Get scheduled executions for this flow
	scheduledExecs, err := h.co.GetScheduledExecutionsByFlow(c.Request().Context(), flow.Meta.DBID, namespace)
	if err != nil {
		h.logger.Error("failed to get scheduled executions", "error", err)
		scheduledExecs = []models.ScheduledExecution{}
	}

	meta := coreFlowMetatoFlowMeta(flow.Meta, flow.Schedules)
	actions := coreFlowActionstoFlowActions(flow.Actions)

	// Transform scheduled executions
	scheduledExecutionItems := make([]ScheduledExecution, len(scheduledExecs))
	for i, exec := range scheduledExecs {
		scheduledExecutionItems[i] = ScheduledExecution{
			ExecID:      exec.ExecID,
			ScheduledAt: exec.ScheduledAt.Format(TimeFormat),
		}
	}

	return c.JSON(http.StatusOK, FlowMetaResp{
		Metadata:            meta,
		Actions:             actions,
		ScheduledExecutions: scheduledExecutionItems,
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

	// Convert scheduling requests to model schedules
	var schedules []models.Schedule
	for _, sched := range req.Meta.Schedules {
		schedules = append(schedules, models.Schedule{
			Cron:     sched.Cron,
			Timezone: sched.Timezone,
		})
	}

	flow := models.Flow{
		Meta: models.Metadata{
			ID:           GenerateSlug(req.Meta.Name),
			Name:         req.Meta.Name,
			Description:  req.Meta.Description,
			Prefix:       req.Meta.Prefix,
			Namespace:    namespace,
			AllowOverlap: req.Meta.AllowOverlap,
			MaxRetries:   req.Meta.MaxRetries,
		},
		Inputs:    convertFlowInputsReqToInputs(req.Inputs),
		Actions:   convertFlowActionsReqToActions(req.Actions),
		Notify:    convertNotifyReqToNotify(req.Notifications),
		Schedules: schedules,
	}

	if err := flow.Validate(); err != nil {
		return wrapError(ErrValidationFailed, err.Error(), err, nil)
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

	var schedules []models.Schedule
	for _, sched := range req.Schedules {
		schedules = append(schedules, models.Schedule{
			Cron:     sched.Cron,
			Timezone: sched.Timezone,
		})
	}

	updatedMeta := f.Meta
	updatedMeta.Prefix = req.Prefix
	updatedMeta.AllowOverlap = req.AllowOverlap
	updatedMeta.UserSchedulable = req.UserSchedulable
	updatedMeta.MaxRetries = req.MaxRetries
	updatedMeta.Description = req.Description

	flow := models.Flow{
		Meta:      updatedMeta,
		Inputs:    convertFlowInputsReqToInputs(req.Inputs),
		Actions:   convertFlowActionsReqToActions(req.Actions),
		Notify:    convertNotifyReqToNotify(req.Notify),
		Schedules: schedules,
	}

	if err := flow.Validate(); err != nil {
		return wrapError(ErrValidationFailed, err.Error(), err, nil)
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

	schedules := make([]Schedule, 0)
	for _, s := range f.Schedules {
		schedules = append(schedules, Schedule{
			Cron:     s.Cron,
			Timezone: s.Timezone,
		})
	}

	return c.JSON(http.StatusOK, FlowCreateReq{
		Meta: FlowMeta{
			ID:              f.Meta.ID,
			Name:            f.Meta.Name,
			Description:     f.Meta.Description,
			Prefix:          f.Meta.Prefix,
			Schedules:       schedules,
			AllowOverlap:    f.Meta.AllowOverlap,
			UserSchedulable: f.Meta.UserSchedulable,
			MaxRetries:      f.Meta.MaxRetries,
		},
		Inputs:        convertFlowInputsToInputsReq(f.Inputs),
		Actions:       convertFlowActionsToActionsReq(f.Actions),
		Notifications: convertNotifyToNotifyReq(f.Notify),
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

	execSummary, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), execID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "execution not found", err, nil)
	}

	cancelUser, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrForbidden, "could not get user info", err, nil)
	}

	restricted, err := h.isUserOnly(c.Request().Context(), cancelUser.ID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not determine user role", err, nil)
	}
	if restricted && execSummary.TriggeredByID != cancelUser.ID {
		return wrapError(ErrForbidden, "insufficient permissions", nil, nil)
	}

	err = h.co.CancelFlowExecution(c.Request().Context(), execID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "failed to cancel execution", err, nil)
	}

	return c.JSON(http.StatusOK, FlowCancellationResp{
		Message: "Cancellation signal sent",
		ExecID:  execID,
	})
}

func (h *Handler) HandleRetryExecution(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	execID := c.Param("execID")
	if execID == "" {
		return wrapError(ErrRequiredFieldMissing, "execution ID is required", nil, nil)
	}

	user, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrAuthenticationFailed, "could not get user details", err, nil)
	}

	execSummary, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), execID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "execution not found", err, nil)
	}

	restricted, err := h.isUserOnly(c.Request().Context(), user.ID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not determine user role", err, nil)
	}
	if restricted && execSummary.TriggeredByID != user.ID {
		return wrapError(ErrForbidden, "insufficient permissions", nil, nil)
	}

	err = h.co.RetryFlowExecution(c.Request().Context(), execID, user.ID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, err.Error(), err, nil)
	}

	return c.NoContent(http.StatusCreated)
}

var namespaceRoleWeight = map[models.NamespaceRole]int{
	models.NamespaceRoleUser:     1,
	models.NamespaceRoleOperator: 2,
	models.NamespaceRoleReviewer: 3,
	models.NamespaceRoleAdmin:    4,
}

// isUserOnly returns true if the caller's effective namespace role is user.
// It takes the highest role across all memberships (direct and group) to handle cases
// where a user has multiple roles in the same namespace.
func (h *Handler) isUserOnly(ctx context.Context, userID, namespaceID string) (bool, error) {
	namespaces, err := h.co.GetUserNamespaces(ctx, userID)
	if err != nil {
		return false, err
	}
	best := 0
	for _, ns := range namespaces {
		if ns.Namespace.ID == namespaceID {
			if w := namespaceRoleWeight[ns.Role]; w > best {
				best = w
			}
		}
	}
	if best == 0 {
		return true, nil // no role found — treat as restricted
	}
	return best == namespaceRoleWeight[models.NamespaceRoleUser], nil
}

type ArtifactMetadata struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
}

func (h *Handler) HandleListArtifacts(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	execID := c.Param("execID")
	if execID == "" {
		return wrapError(ErrRequiredFieldMissing, "execID cannot be empty", nil, nil)
	}

	execSummary, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), execID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "execution not found", err, nil)
	}

	userInfo, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrForbidden, "could not get user info", err, nil)
	}

	restricted, err := h.isUserOnly(c.Request().Context(), userInfo.ID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not determine user role", err, nil)
	}
	if restricted && execSummary.TriggeredByID != userInfo.ID {
		return wrapError(ErrForbidden, "insufficient permissions", nil, nil)
	}

	artifacts := []ArtifactMetadata{}
	if h.storage == nil {
		return c.JSON(http.StatusOK, artifacts)
	}

	prefix := fmt.Sprintf("%s/%s/", execSummary.FlowID, execID)
	list, err := h.storage.List(c.Request().Context(), prefix)
	if err != nil {
		h.logger.Error("failed to list artifacts", "execID", execID, "error", err)
		return wrapError(ErrOperationFailed, "failed to list artifacts", err, nil)
	}

	for _, obj := range list {
		relPath := strings.TrimPrefix(obj.Path, prefix)
		artifacts = append(artifacts, ArtifactMetadata{
			Path:       relPath,
			Name:       obj.Name,
			Size:       obj.Size,
			ModifiedAt: obj.ModifiedAt,
		})
	}

	return c.JSON(http.StatusOK, artifacts)
}

func (h *Handler) HandleDownloadArtifact(c echo.Context) error {
	namespace, ok := c.Get("namespace").(string)
	if !ok {
		return wrapError(ErrRequiredFieldMissing, "could not get namespace", nil, nil)
	}

	execID := c.Param("execID")
	if execID == "" {
		return wrapError(ErrRequiredFieldMissing, "execID cannot be empty", nil, nil)
	}

	pathParam := c.QueryParam("path")
	if pathParam == "" {
		return wrapError(ErrRequiredFieldMissing, "path query parameter is required", nil, nil)
	}

	execSummary, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), execID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "execution not found", err, nil)
	}

	userInfo, err := h.getUserInfo(c)
	if err != nil {
		return wrapError(ErrForbidden, "could not get user info", err, nil)
	}

	restricted, err := h.isUserOnly(c.Request().Context(), userInfo.ID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, "could not determine user role", err, nil)
	}
	if restricted && execSummary.TriggeredByID != userInfo.ID {
		return wrapError(ErrForbidden, "insufficient permissions", nil, nil)
	}

	if h.storage == nil {
		return wrapError(ErrResourceNotFound, "artifact retention is disabled", nil, nil)
	}

	prefix := fmt.Sprintf("%s/%s/", execSummary.FlowID, execID)
	storageKey := filepath.Clean(prefix + pathParam)
	if !strings.HasPrefix(storageKey, prefix) {
		return wrapError(ErrForbidden, "access denied", nil, nil)
	}

	reader, err := h.storage.Get(c.Request().Context(), storageKey)
	if err != nil {
		return wrapError(ErrResourceNotFound, "artifact not found", err, nil)
	}

	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", filepath.Base(storageKey)))
	return c.Stream(http.StatusOK, "application/octet-stream", reader)
}

