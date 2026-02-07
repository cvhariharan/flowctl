package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/utils"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const (
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
		default:
			if value := c.FormValue(input.Name); value != "" {
				req[input.Name] = value
			}
		}
	}

	return req, nil
}

// mergeRemoteOptions fetches and merges remote options into the flow's inputs
// This is called before validation to ensure select inputs have options available
func (h *Handler) mergeRemoteOptions(flow *models.Flow, variables map[string]interface{}) error {
	for i, input := range flow.Inputs {
		// Only process select inputs with OptionsURL
		if input.Type == models.INPUT_TYPE_SELECT && input.OptionsURL != "" {
			// Fetch remote options with variable interpolation
			remoteOptions, err := utils.FetchRemoteOptionsWithVariables(input.OptionsURL, variables)
			if err != nil {
				h.logger.Warn("failed to fetch remote options", "input", input.Name, "url", input.OptionsURL, "error", err)
				// Continue without remote options - will fall back to static options if available
				continue
			}

			// Merge remote options with static options
			if len(remoteOptions) > 0 {
				// If no static options, use only remote options
				if len(input.Options) == 0 {
					flow.Inputs[i].Options = remoteOptions
				} else {
					// Merge: prioritize remote options but keep unique static options
					mergedOptions := make([]string, 0)
					for _, opt := range remoteOptions {
						if !slices.Contains(mergedOptions, opt) {
							mergedOptions = append(mergedOptions, opt)
						}
					}
					for _, opt := range input.Options {
						if !slices.Contains(mergedOptions, opt) {
							mergedOptions = append(mergedOptions, opt)
						}
					}
					flow.Inputs[i].Options = mergedOptions
				}
			}
		}
	}
	return nil
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

	// Merge remote options from URLs into the flow inputs before validation
	if err := h.mergeRemoteOptions(&f, req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("could not fetch remote options: %v", err), err, nil)
	}

	if err := f.ValidateInput(req); err != nil {
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

	var req LogStreamingReq
	if err := c.Bind(&req); err != nil {
		return wrapError(ErrValidationFailed, "invalid request", err, nil)
	}

	if err := h.validate.Struct(req); err != nil {
		return wrapError(ErrValidationFailed, fmt.Sprintf("request validation failed: %s", formatValidationErrors(err)), err, nil)
	}

	logID := req.LogID

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

	executions, pageCount, totalCount, err := h.co.GetAllExecutionSummaryPaginated(c.Request().Context(), namespace, req.Filter, req.Count, req.Count*req.Page)
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

	h.logger.Debug("flow input", "input", fmt.Sprintf("%+v", flow.Inputs))

	// Fetch remote options if OptionsURL is provided
	// Pass an empty context since there are no user inputs available yet
	_ = h.mergeRemoteOptions(&flow, make(map[string]interface{}))

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
			Namespace:    namespace,
			AllowOverlap: req.Meta.AllowOverlap,
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
	updatedMeta.AllowOverlap = req.AllowOverlap
	updatedMeta.UserSchedulable = req.UserSchedulable
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
			Schedules:       schedules,
			AllowOverlap:    f.Meta.AllowOverlap,
			UserSchedulable: f.Meta.UserSchedulable,
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

	// Verify the execution exists and user has permission to cancel it
	_, err := h.co.GetExecutionSummaryByExecID(c.Request().Context(), execID, namespace)
	if err != nil {
		return wrapError(ErrResourceNotFound, "execution not found", err, nil)
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

	err = h.co.RetryFlowExecution(c.Request().Context(), execID, user.ID, namespace)
	if err != nil {
		return wrapError(ErrOperationFailed, err.Error(), err, nil)
	}

	return c.NoContent(http.StatusCreated)
}
