package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/cvhariharan/flowctl/internal/scheduler"
	"github.com/expr-lang/expr"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
)

var (
	ErrFlowNotFound = errors.New("flow not found")
)

// resolveOverrideNodes finds a node-typed input in the flow and resolves its selected
// names/tags to scheduler nodes. Returns nil if no node input exists or nothing was selected.
func (c *Core) resolveOverrideNodes(ctx context.Context, f models.Flow, input map[string]any, namespaceUUID uuid.UUID) ([]scheduler.Node, error) {
	for _, inp := range f.Inputs {
		if inp.Type != models.INPUT_TYPE_NODE {
			continue
		}
		raw, ok := input[inp.Name]
		if !ok {
			continue
		}
		targets := toStringSlice(raw)
		if len(targets) == 0 {
			continue
		}
		nodes, err := models.ResolveTargets(ctx, targets, namespaceUUID, c.GetNodesByNames, c.GetNodesByTags)
		if err != nil {
			return nil, err
		}
		return models.NodesToScheduler(nodes), nil
	}
	return nil, nil
}

func toStringSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, it := range t {
			if s, ok := it.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	}
	return nil
}

// isSubpath checks if child is under parent (or equal to parent)
func isSubpath(parent, child string) (bool, error) {
	parent, err := filepath.Abs(parent)
	if err != nil {
		return false, err
	}
	child, err = filepath.Abs(child)
	if err != nil {
		return false, err
	}

	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false, err
	}

	return !strings.HasPrefix(rel, ".."), nil
}

// detectFlowFormat determines the flow format based on file extension
func detectFlowFormat(filePath string) models.FlowFormat {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".huml":
		return models.FlowFormatHUML
	case ".yaml", ".yml":
		return models.FlowFormatYAML
	default:
		return models.FlowFormatYAML // Default to YAML for backwards compatibility
	}
}

// isFlowFile checks if the file has a valid flow extension (.yaml, .yml, or .huml)
func isFlowFile(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".yml") ||
		strings.HasSuffix(lower, ".yaml") ||
		strings.HasSuffix(lower, ".huml")
}

// GetFlowByID returns a flow from memory using the flow slug (id) and namespace
func (c *Core) GetFlowByID(id string, namespaceID string) (models.Flow, error) {
	c.rwf.RLock()
	defer c.rwf.RUnlock()
	f, ok := c.flows[fmt.Sprintf("%s:%s", id, namespaceID)]
	if !ok {
		return models.Flow{}, ErrFlowNotFound
	}

	// Clone Inputs so callers can mutate per-input
	// fields without racing with concurrent readers of the cached flow.
	if len(f.Inputs) > 0 {
		inputs := make([]models.Input, len(f.Inputs))
		copy(inputs, f.Inputs)
		f.Inputs = inputs
	}

	return f, nil
}

func (c *Core) GetScheduledExecutionsByFlow(ctx context.Context, flowID int32, namespaceID string) ([]models.ScheduledExecution, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	execs, err := c.store.GetScheduledExecutionsByFlow(ctx, repo.GetScheduledExecutionsByFlowParams{
		FlowID: flowID,
		Uuid:   namespaceUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("could not get scheduled executions for flow %d: %w", flowID, err)
	}

	scheduledExecs := make([]models.ScheduledExecution, 0, len(execs))
	for _, exec := range execs {
		scheduledExecs = append(scheduledExecs, models.ScheduledExecution{
			ExecID:      exec.ExecID,
			ScheduledAt: exec.ScheduledAt.Time,
		})
	}

	return scheduledExecs, nil
}

func (c *Core) GetFlowsPaginated(ctx context.Context, namespaceID string, userID string, limit, offset int) ([]models.Flow, int64, int64, error) {
	c.rwf.RLock()
	defer c.rwf.RUnlock()
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	prefixes, hasFullAccess, err := c.getUserPrefixAccess(ctx, userID, namespaceID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("could not get user prefix access: %w", err)
	}

	var fs []models.Flow
	var pageCount, totalCount int64

	if hasFullAccess {
		flows, err := c.store.ListFlowsPaginated(ctx, repo.ListFlowsPaginatedParams{
			Uuid:   namespaceUUID,
			Limit:  int32(limit),
			Offset: int32(offset),
		})
		if err != nil {
			return nil, 0, 0, fmt.Errorf("could not get paginated flows for namespace %s: %w", namespaceID, err)
		}
		for _, v := range flows {
			fs = append(fs, c.flows[fmt.Sprintf("%s:%s", v.Slug, namespaceID)])
			pageCount = v.PageCount
			totalCount = v.TotalCount
		}
	} else {
		flows, err := c.store.ListFlowsPaginatedFiltered(ctx, repo.ListFlowsPaginatedFilteredParams{
			Uuid:    namespaceUUID,
			Limit:   int32(limit),
			Offset:  int32(offset),
			Column4: prefixes,
		})
		if err != nil {
			return nil, 0, 0, fmt.Errorf("could not get filtered paginated flows for namespace %s: %w", namespaceID, err)
		}
		for _, v := range flows {
			fs = append(fs, c.flows[fmt.Sprintf("%s:%s", v.Slug, namespaceID)])
			pageCount = v.PageCount
			totalCount = v.TotalCount
		}
	}

	return fs, pageCount, totalCount, nil
}

func (c *Core) SearchFlows(ctx context.Context, namespaceID string, userID string, query string, limit, offset int) ([]models.Flow, int64, int64, error) {
	c.rwf.RLock()
	defer c.rwf.RUnlock()
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	prefixes, hasFullAccess, err := c.getUserPrefixAccess(ctx, userID, namespaceID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("could not get user prefix access: %w", err)
	}

	var fs []models.Flow
	var pageCount, totalCount int64

	if hasFullAccess {
		flows, err := c.store.SearchFlowsPaginated(ctx, repo.SearchFlowsPaginatedParams{
			Uuid:    namespaceUUID,
			Column2: query,
			Limit:   int32(limit),
			Offset:  int32(offset),
		})
		if err != nil {
			return nil, 0, 0, fmt.Errorf("could not search flows for namespace %s: %w", namespaceID, err)
		}
		for _, v := range flows {
			fs = append(fs, c.flows[fmt.Sprintf("%s:%s", v.Slug, namespaceID)])
			pageCount = v.PageCount
			totalCount = v.TotalCount
		}
	} else {
		flows, err := c.store.SearchFlowsPaginatedFiltered(ctx, repo.SearchFlowsPaginatedFilteredParams{
			Uuid:    namespaceUUID,
			Column2: query,
			Limit:   int32(limit),
			Offset:  int32(offset),
			Column5: prefixes,
		})
		if err != nil {
			return nil, 0, 0, fmt.Errorf("could not search filtered flows for namespace %s: %w", namespaceID, err)
		}
		for _, v := range flows {
			fs = append(fs, c.flows[fmt.Sprintf("%s:%s", v.Slug, namespaceID)])
			pageCount = v.PageCount
			totalCount = v.TotalCount
		}
	}

	return fs, pageCount, totalCount, nil
}

// GetFlowsByPrefix returns all flows in a namespace with the given prefix name
func (c *Core) GetFlowsByPrefix(ctx context.Context, namespaceID, prefix string) ([]models.Flow, error) {
	c.rwf.RLock()
	defer c.rwf.RUnlock()

	prefixID, err := c.ResolvePrefixID(ctx, prefix, namespaceID)
	if err != nil {
		return nil, err
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	rows, err := c.store.GetFlowsByPrefix(ctx, repo.GetFlowsByPrefixParams{
		Uuid:     namespaceUUID,
		PrefixID: prefixID,
	})
	if err != nil {
		return nil, fmt.Errorf("could not get flows by prefix: %w", err)
	}

	var fs []models.Flow
	for _, v := range rows {
		if f, ok := c.flows[fmt.Sprintf("%s:%s", v.Slug, namespaceID)]; ok {
			fs = append(fs, f)
		}
	}
	return fs, nil
}

func (c *Core) GetFlowFromLogID(logID string, namespaceID string) (models.Flow, error) {
	f, ok := c.logMap[logID]
	if !ok {
		namespaceUUID, err := uuid.Parse(namespaceID)
		if err != nil {
			return models.Flow{}, fmt.Errorf("invalid namespace UUID: %w", err)
		}
		df, err := c.store.GetFlowFromExecID(context.Background(), repo.GetFlowFromExecIDParams{
			ExecID: logID,
			Uuid:   namespaceUUID,
		})
		if err != nil {
			return models.Flow{}, fmt.Errorf("could not get flow for exec id %s: %w", logID, err)
		}
		return c.GetFlowByID(df.Slug, namespaceID)
	}

	return c.GetFlowByID(f, namespaceID)
}

// QueueFlowExecution adds a flow in the execution queue. The ID returned is the execution queue ID.
// Exec ID should be universally unique, this is used to create the log stream and identify each execution
// If scheduledAt is provided, the flow will be scheduled to run at that time instead of immediately.
func (c *Core) QueueFlowExecution(ctx context.Context, f models.Flow, input map[string]interface{}, userUUID string, namespaceID string, scheduledAt *time.Time) (string, error) {
	return c.QueueFlowExecutionWithExecID(ctx, f, input, userUUID, namespaceID, "", scheduledAt)
}

// QueueFlowExecutionWithExecID adds a flow in the execution queue with a pre-generated execution ID.
// If execID is empty, a new UUID is generated. Use this when files need to be uploaded before queuing.
func (c *Core) QueueFlowExecutionWithExecID(ctx context.Context, f models.Flow, input map[string]interface{}, userUUID string, namespaceID string, execID string, scheduledAt *time.Time) (string, error) {
	if !f.Meta.AllowOverlap {
		namespaceUUID, err := uuid.Parse(namespaceID)
		if err != nil {
			return "", fmt.Errorf("invalid namespace UUID: %w", err)
		}
		execExists, err := c.store.ExecutionExistsForFlow(ctx, repo.ExecutionExistsForFlowParams{
			Slug: f.Meta.ID,
			Uuid: namespaceUUID,
		})
		if err != nil {
			return "", fmt.Errorf("error checking existing executions for flow %s: %w", f.Meta.ID, err)
		}
		if execExists {
			return "", fmt.Errorf("could not queue flow %s for execution: execution overlap is disabled", f.Meta.Name)
		}
	}

	info, err := c.queueFlow(ctx, f, input, execID, 0, userUUID, namespaceID, false, scheduledAt)
	if err != nil {
		return "", err
	}

	return info, nil
}

// ResumeFlowExecution moves the task to a resume queue for further processing.
func (c *Core) ResumeFlowExecution(ctx context.Context, execID string, actionID string, userUUID string, namespaceID string, retry bool) error {
	exec, err := c.GetExecutionByExecID(ctx, execID, namespaceID)
	if err != nil {
		return fmt.Errorf("could not get exec %s: %w", execID, err)
	}

	f, err := c.GetFlowFromLogID(execID, namespaceID)
	if err != nil {
		return err
	}

	actionIndex, err := f.GetActionIndexByID(actionID)
	if err != nil {
		return err
	}

	if _, err := c.queueFlow(ctx, f, exec.Input, execID, actionIndex, userUUID, namespaceID, retry, nil); err != nil {
		return err
	}

	return nil
}

// RetryFlowExecution retries a failed or cancelled execution from the point of failure.
// It automatically detects the retry point from CurrentActionID and resumes execution from there.
func (c *Core) RetryFlowExecution(ctx context.Context, execID string, userUUID string, namespaceID string) error {
	exec, err := c.GetExecutionSummaryByExecID(ctx, execID, namespaceID)
	if err != nil {
		return fmt.Errorf("could not get exec %s: %w", execID, err)
	}

	if exec.Status != models.ExecutionStatus(repo.ExecutionStatusErrored) && exec.Status != models.ExecutionStatus(repo.ExecutionStatusCancelled) {
		return fmt.Errorf("execution must be in errored or cancelled state to retry, current status: %s", exec.Status)
	}

	if exec.CurrentActionID == "" {
		return fmt.Errorf("cannot determine retry point - no current action ID")
	}

	return c.ResumeFlowExecution(ctx, execID, exec.CurrentActionID, userUUID, namespaceID, true)
}

// queueFlow adds a flow to the execution queue. If the actionIndex is not zero, it is moved to a resume queue.
// If scheduledAt is provided, the flow will be scheduled to run at that time instead of immediately.
func (c *Core) queueFlow(ctx context.Context, f models.Flow, input map[string]interface{}, execID string, actionIndex int, userUUID string, namespaceID string, retry bool, scheduledAt *time.Time) (string, error) {
	// If execID is empty, it is a new flow execution
	if execID == "" {
		execID = uuid.NewString()
	}

	userID, err := uuid.Parse(userUUID)
	if err != nil {
		return "", fmt.Errorf("user id is not a UUID: %w", err)
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return "", fmt.Errorf("invalid namespace UUID: %w", err)
	}

	fl, err := c.store.GetFlowBySlug(ctx, repo.GetFlowBySlugParams{
		Slug:     f.Meta.ID,
		Uuid:     namespaceUUID,
		IsActive: sql.NullBool{Bool: true, Valid: true},
	})
	if err != nil {
		return "", fmt.Errorf("error getting flow details for %s from DB: %w", f.Meta.ID, err)
	}

	// Convert to scheduler flow format
	schedulerFlow, err := models.ConvertToSchedulerFlow(ctx, f, namespaceUUID, c.GetNodesByNames, c.GetNodesByTags)
	if err != nil {
		return "", fmt.Errorf("error converting flow to scheduler model: %w", err)
	}

	// Determine trigger type based on scheduledAt parameter
	triggerType := scheduler.TriggerTypeManual
	dbTriggerType := repo.TriggerTypeManual
	if scheduledAt != nil {
		triggerType = scheduler.TriggerTypeScheduled
		dbTriggerType = repo.TriggerTypeScheduled
	}

	overrideNodes, err := c.resolveOverrideNodes(ctx, f, input, namespaceUUID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve override nodes: %w", err)
	}

	// Create flow execution payload for scheduler
	payload := scheduler.FlowExecutionPayload{
		Workflow:          schedulerFlow,
		Input:             input,
		StartingActionIdx: actionIndex,
		NamespaceID:       namespaceID,
		TriggerType:       triggerType,
		UserUUID:          userUUID,
		FlowDirectory:     filepath.Dir(fl.FilePath),
		OverrideNodes:     overrideNodes,
		Resumed:           retry,
	}

	// Create execution log for manual flows before queuing (needed for immediate API calls)
	inputB, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("could not marshal input to json: %w", err)
	}

	// Convert scheduledAt to sql.NullTime for database
	var scheduledAtDB sql.NullTime
	if scheduledAt != nil {
		scheduledAtDB = sql.NullTime{Time: *scheduledAt, Valid: true}
	}

	_, err = c.store.AddExecutionLog(ctx, repo.AddExecutionLogParams{
		ExecID:      execID,
		FlowID:      f.Meta.DBID,
		Input:       inputB,
		TriggerType: dbTriggerType,
		Uuid:        userID,
		Uuid_2:      namespaceUUID,
		ScheduledAt: scheduledAtDB,
	})
	if err != nil {
		return "", fmt.Errorf("could not add entry to execution log: %w", err)
	}

	// Queue the task using the scheduler
	maxRetries := f.Meta.MaxRetries
	if scheduledAt != nil {
		if maxRetries > 0 {
			_, err = c.scheduler.QueueScheduledTaskWithRetries(ctx, scheduler.PayloadTypeFlowExecution, execID, payload, *scheduledAt, maxRetries)
		} else {
			_, err = c.scheduler.QueueScheduledTask(ctx, scheduler.PayloadTypeFlowExecution, execID, payload, *scheduledAt)
		}
	} else {
		if maxRetries > 0 {
			_, err = c.scheduler.QueueTaskWithRetries(ctx, scheduler.PayloadTypeFlowExecution, execID, payload, maxRetries)
		} else {
			_, err = c.scheduler.QueueTask(ctx, scheduler.PayloadTypeFlowExecution, execID, payload)
		}
	}
	if err != nil {
		return "", err
	}

	return execID, nil
}

// CancelFlowExecution cancels the given execution using the scheduler
func (c *Core) CancelFlowExecution(ctx context.Context, execID string, namespaceID string) error {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	// Update execution status to cancelled in the database
	_, err = c.store.UpdateExecutionStatus(ctx, repo.UpdateExecutionStatusParams{
		Status: repo.ExecutionStatusCancelled,
		ExecID: execID,
		Uuid:   namespaceUUID,
	})
	if err != nil {
		return fmt.Errorf("failed to update execution status: %w", err)
	}

	return c.scheduler.CancelTask(ctx, execID)
}

func (c *Core) GetExecutionSummaryPaginated(ctx context.Context, f models.Flow, namespaceID string, callerID string, limit, offset int) ([]models.ExecutionSummary, int64, int64, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	callerUUID, err := uuid.Parse(callerID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid caller UUID: %w", err)
	}

	execs, err := c.store.GetExecutionsByFlowPaginated(ctx, repo.GetExecutionsByFlowPaginatedParams{
		ID:     f.Meta.DBID,
		Uuid:   namespaceUUID,
		Limit:  int32(limit),
		Offset: int32(offset),
		Uuid_2: callerUUID,
	})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("could not get paginated executions for %s: %w", f.Meta.ID, err)
	}

	var m []models.ExecutionSummary
	var pageCount, totalCount int64

	for _, v := range execs {
		actionRetries := make(map[string]int)
		if v.ActionRetries.Valid {
			if err := json.Unmarshal(v.ActionRetries.RawMessage, &actionRetries); err != nil {
				log.Printf("failed to unmarshal action_retries: %v", err)
			}
		}

		m = append(m, models.ExecutionSummary{
			ExecID:          v.ExecID,
			FlowName:        v.FlowName,
			FlowID:          v.FlowSlug,
			CreatedAt:       v.CreatedAt,
			StartedAt:       v.StartedAt.Time,
			CompletedAt:     v.CompletedAt.Time,
			TriggerType:     string(v.TriggerType),
			Status:          models.ExecutionStatus(v.Status),
			TriggeredByName: v.TriggeredByName,
			TriggeredByID:   v.TriggeredByUuid.String(),
			CurrentActionID: v.CurrentActionID.String,
			ActionRetries:   actionRetries,
			ScheduledAt:     v.ScheduledAt.Time,
		})
		pageCount = v.PageCount
		totalCount = v.TotalCount
	}

	return m, pageCount, totalCount, nil
}

func (c *Core) GetAllExecutionSummaryPaginated(ctx context.Context, namespaceID string, callerID string, filter string, limit, offset int) ([]models.ExecutionSummary, int64, int64, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	callerUUID, err := uuid.Parse(callerID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid caller UUID: %w", err)
	}

	execs, err := c.store.SearchExecutionsPaginated(ctx, repo.SearchExecutionsPaginatedParams{
		Uuid:    namespaceUUID,
		Column2: filter,
		Limit:   int32(limit),
		Offset:  int32(offset),
		Uuid_2:  callerUUID,
	})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("could not get all paginated executions: %w", err)
	}

	var m []models.ExecutionSummary
	var pageCount, totalCount int64

	for _, v := range execs {
		actionRetries := make(map[string]int)
		if v.ActionRetries.Valid {
			if err := json.Unmarshal(v.ActionRetries.RawMessage, &actionRetries); err != nil {
				log.Printf("failed to unmarshal action_retries: %v", err)
			}
		}

		m = append(m, models.ExecutionSummary{
			ExecID:          v.ExecID,
			FlowName:        v.FlowName,
			FlowID:          v.FlowSlug,
			CreatedAt:       v.CreatedAt,
			StartedAt:       v.StartedAt.Time,
			CompletedAt:     v.CompletedAt.Time,
			TriggerType:     string(v.TriggerType),
			Status:          models.ExecutionStatus(v.Status),
			TriggeredByName: v.TriggeredByName,
			TriggeredByID:   v.TriggeredByUuid.String(),
			CurrentActionID: v.CurrentActionID.String,
			ActionRetries:   actionRetries,
			ScheduledAt:     v.ScheduledAt.Time,
		})
		pageCount = v.PageCount
		totalCount = v.TotalCount
	}

	return m, pageCount, totalCount, nil
}

func (c *Core) GetExecutionSummaryByExecID(ctx context.Context, execID string, namespaceID string) (models.ExecutionSummary, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return models.ExecutionSummary{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}
	e, err := c.store.GetExecutionByExecID(ctx, repo.GetExecutionByExecIDParams{
		ExecID: execID,
		Uuid:   namespaceUUID,
	})
	if err != nil {
		return models.ExecutionSummary{}, fmt.Errorf("could not get exec %s by exec id: %w", execID, err)
	}

	// Parse action_retries JSONB
	actionRetries := make(map[string]int)
	if e.ActionRetries.Valid {
		if err := json.Unmarshal(e.ActionRetries.RawMessage, &actionRetries); err != nil {
			// Log error but don't fail - this is non-critical
			log.Printf("failed to unmarshal action_retries for exec %s: %v", execID, err)
		}
	}

	return models.ExecutionSummary{
		ExecID:          execID,
		Input:           e.Input,
		FlowName:        e.FlowName,
		FlowID:          e.FlowSlug,
		Status:          models.ExecutionStatus(e.Status),
		CreatedAt:       e.CreatedAt,
		StartedAt:       e.StartedAt.Time,
		CompletedAt:     e.CompletedAt.Time,
		TriggerType:     string(e.TriggerType),
		TriggeredByName: e.TriggeredByName,
		TriggeredByID:   e.TriggeredByUuid.String(),
		CurrentActionID: e.CurrentActionID.String,
		ActionRetries:   actionRetries,
		ScheduledAt:     e.ScheduledAt.Time,
	}, nil
}

func (c *Core) GetInputForExec(ctx context.Context, execID string, namespaceID string) (map[string]interface{}, error) {
	var input map[string]interface{}
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}
	in, err := c.store.GetInputForExecByUUID(ctx, repo.GetInputForExecByUUIDParams{
		ExecID: execID,
		Uuid:   namespaceUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("error getting input for %s: %w", execID, err)
	}

	if err := json.Unmarshal(in, &input); err != nil {
		return nil, fmt.Errorf("error unmarshaling input for %s: %w", execID, err)
	}

	return input, nil
}

func (c *Core) GetExecutionByExecID(ctx context.Context, execID string, namespaceID string) (models.Execution, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return models.Execution{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}
	e, err := c.store.GetExecutionByExecID(ctx, repo.GetExecutionByExecIDParams{
		ExecID: execID,
		Uuid:   namespaceUUID,
	})
	if err != nil {
		return models.Execution{}, fmt.Errorf("could not get execution for exec %s: %w", execID, err)
	}

	var input map[string]interface{}
	if err := json.Unmarshal(e.Input, &input); err != nil {
		return models.Execution{}, fmt.Errorf("error unmarshaling input for %s: %w", execID, err)
	}

	u, err := c.store.GetUserByID(ctx, e.TriggeredBy)
	if err != nil {
		return models.Execution{}, fmt.Errorf("could not get trigger person for %s: %w", execID, err)
	}

	return models.Execution{
		ExecID:      e.ExecID,
		Version:     int64(e.Version),
		Input:       input,
		ErrorMsg:    e.Error.String,
		TriggeredBy: u.Uuid.String(),
	}, nil
}

func (c *Core) CreateFlow(ctx context.Context, f models.Flow, namespaceID string) error {
	c.rwf.RLock()
	if _, exists := c.flows[f.Meta.ID]; exists {
		return fmt.Errorf("flow with id %s already exists", f.Meta.ID)
	}
	c.rwf.RUnlock()

	// Remove duplicate schedules
	f.Schedules = removeDuplicateSchedules(f.Schedules)

	n, err := c.GetNamespaceByID(ctx, namespaceID)
	if err != nil {
		return fmt.Errorf("could not get namespace details for %s: %w", namespaceID, err)
	}

	namespaceDirPath := filepath.Join(c.flowDirectory, n.Name)
	if err := os.MkdirAll(namespaceDirPath, 0755); err != nil {
		return fmt.Errorf("could not create namespace directory: %w", err)
	}

	flowDir := filepath.Join(namespaceDirPath, f.Meta.ID)
	if err := os.MkdirAll(flowDir, 0755); err != nil {
		return fmt.Errorf("could not create flow directory: %w", err)
	}

	yamlFilePath := filepath.Join(flowDir, fmt.Sprintf("%s.yaml", f.Meta.ID))
	if _, err := os.Stat(yamlFilePath); err == nil {
		return fmt.Errorf("flow with this ID already exists: %w", err)
	}

	yamlData, err := models.MarshalFlow(f, models.FlowFormatYAML)
	if err != nil {
		return fmt.Errorf("could not marshal flow to YAML: %w", err)
	}

	if err := os.WriteFile(yamlFilePath, yamlData, 0644); err != nil {
		return fmt.Errorf("could not write flow file: %w", err)
	}

	importedFlow, namespaceUUID, err := c.importFlowFromFile(ctx, yamlFilePath, n.Name)
	if err != nil {
		return fmt.Errorf("could not import flow after creation: %w", err)
	}

	c.rwf.Lock()
	defer c.rwf.Unlock()
	c.flows[fmt.Sprintf("%s:%s", importedFlow.Meta.ID, namespaceUUID)] = importedFlow
	return nil
}

func (c *Core) UpdateFlow(ctx context.Context, f models.Flow, namespaceID string) error {
	c.rwf.RLock()
	if _, exists := c.flows[fmt.Sprintf("%s:%s", f.Meta.ID, namespaceID)]; !exists {
		return fmt.Errorf("flow with id %s does not exist", f.Meta.ID)
	}
	c.rwf.RUnlock()

	// Remove duplicate schedules
	f.Schedules = removeDuplicateSchedules(f.Schedules)

	n, err := c.GetNamespaceByID(ctx, namespaceID)
	if err != nil {
		return fmt.Errorf("could not get namespace details for %s: %w", namespaceID, err)
	}

	// Get the existing flow from database to retrieve the file path
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	existingFlow, err := c.store.GetFlowBySlug(ctx, repo.GetFlowBySlugParams{
		Slug:     f.Meta.ID,
		Uuid:     namespaceUUID,
		IsActive: sql.NullBool{Bool: true, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("could not get existing flow: %w", err)
	}

	flowFilePath := existingFlow.FilePath
	if _, err := os.Stat(flowFilePath); err != nil {
		return fmt.Errorf("flow file does not exist at %s: %w", flowFilePath, err)
	}

	isSub, err := isSubpath(c.flowDirectory, flowFilePath)
	if err != nil || !isSub {
		return fmt.Errorf("cannot write to file outside flows root: %s", flowFilePath)
	}

	// Detect the format of the existing file and use the same format
	format := detectFlowFormat(flowFilePath)
	flowData, err := models.MarshalFlow(f, format)
	if err != nil {
		return fmt.Errorf("could not marshal flow to %s: %w", format, err)
	}

	if err := os.WriteFile(flowFilePath, flowData, 0644); err != nil {
		return fmt.Errorf("could not write flow file: %w", err)
	}

	importedFlow, namespaceUUIDStr, err := c.importFlowFromFile(ctx, flowFilePath, n.Name)
	if err != nil {
		return fmt.Errorf("could not import flow after update: %w", err)
	}

	c.rwf.Lock()
	defer c.rwf.Unlock()
	c.flows[fmt.Sprintf("%s:%s", importedFlow.Meta.ID, namespaceUUIDStr)] = importedFlow
	return nil
}

func (c *Core) DeleteFlow(ctx context.Context, flowID, namespaceID string) error {
	c.rwf.RLock()
	if _, exists := c.flows[fmt.Sprintf("%s:%s", flowID, namespaceID)]; !exists {
		return fmt.Errorf("flow with id %s does not exist", flowID)
	}
	c.rwf.RUnlock()

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	// Get the file path from the database
	existingFlow, err := c.store.GetFlowBySlug(ctx, repo.GetFlowBySlugParams{
		Slug:     flowID,
		Uuid:     namespaceUUID,
		IsActive: sql.NullBool{Valid: false},
	})
	if err != nil {
		return fmt.Errorf("could not get flow details: %w", err)
	}

	// Delete the flow directory
	flowDir := filepath.Dir(existingFlow.FilePath)
	isSub, err := isSubpath(c.flowDirectory, flowDir)
	if err != nil || !isSub {
		return fmt.Errorf("cannot delete directory outside flows root: %s", flowDir)
	}

	if err := os.RemoveAll(flowDir); err != nil {
		return fmt.Errorf("could not delete flow: %w", err)
	}

	if err := c.store.DeleteFlow(ctx, repo.DeleteFlowParams{
		Slug: flowID,
		Uuid: namespaceUUID,
	}); err != nil {
		return fmt.Errorf("could not delete flow %s from DB: %w", flowID, err)
	}

	c.rwf.Lock()
	defer c.rwf.Unlock()
	delete(c.flows, fmt.Sprintf("%s:%s", flowID, namespaceID))
	return nil
}

func (c *Core) LoadFlows(ctx context.Context) error {
	m := make(map[string]models.Flow)

	// Read immediate subdirectories
	entries, err := os.ReadDir(c.flowDirectory)
	if err != nil {
		return fmt.Errorf("error reading flow directory: %w", err)
	}

	// Each subdirectory in the root flows directory should be a namespace
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		namespaceDir := filepath.Join(c.flowDirectory, entry.Name())
		namespaceFlows, err := c.processNamespaceFlows(ctx, namespaceDir)
		if err != nil {
			log.Printf("could not process flows from namespace %s: %v", entry.Name(), err)
			continue
		}

		maps.Copy(m, namespaceFlows)
	}
	c.flows = m
	return nil
}

// processNamespaceFlows iterates through directories in the namespace directory and imports flows.
// Each subdirectory under flows/<namespace>/ is treated as a flow directory.
func (c *Core) processNamespaceFlows(ctx context.Context, namespaceDir string) (map[string]models.Flow, error) {
	m := make(map[string]models.Flow)
	namespaceName := filepath.Base(namespaceDir)

	ns, err := c.store.GetNamespaceByName(context.Background(), namespaceName)
	if err != nil {
		return nil, fmt.Errorf("error getting namespace %s: %w", namespaceName, err)
	}

	err = c.store.MarkAllFlowsInactiveForNamespace(context.Background(), ns.Uuid)
	if err != nil {
		log.Printf("error marking flows inactive for namespace %s: %v", namespaceName, err)
	}

	entries, err := os.ReadDir(namespaceDir)
	if err != nil {
		return nil, fmt.Errorf("error reading namespace %s directory: %w", namespaceDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		flowDir := filepath.Join(namespaceDir, entry.Name())
		flowPath := findFlowFile(flowDir)
		if flowPath == "" {
			continue
		}

		f, nsUUID, err := c.importFlowFromFile(ctx, flowPath, namespaceName)
		if err != nil {
			log.Printf("error importing flow from %s: %v", flowPath, err)
			continue
		}
		m[fmt.Sprintf("%s:%s", f.Meta.ID, nsUUID)] = f
	}

	return m, nil
}

// findFlowFile returns the path to the first flow file in the given directory
func findFlowFile(dir string) string {
	files, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, file := range files {
		if !file.IsDir() && isFlowFile(file.Name()) {
			return filepath.Join(dir, file.Name())
		}
	}
	return ""
}

func (c *Core) importFlowFromFile(ctx context.Context, flowFilePath, namespaceName string) (models.Flow, string, error) {
	data, err := os.ReadFile(flowFilePath)
	if err != nil {
		return models.Flow{}, "", fmt.Errorf("error reading file %s: %w", flowFilePath, err)
	}

	h := sha256.New()
	h.Write(data)
	checksum := hex.EncodeToString(h.Sum(nil))

	// Detect format based on file extension and unmarshal accordingly
	format := detectFlowFormat(flowFilePath)
	f, err := models.UnmarshalFlow(data, format)
	if err != nil {
		return models.Flow{}, "", fmt.Errorf("error parsing flow file in %s: %w", flowFilePath, err)
	}

	if err := f.Validate(); err != nil {
		return models.Flow{}, "", fmt.Errorf("validation error in %s: %w", flowFilePath, err)
	}

	f.Meta.SrcDir = filepath.Base(filepath.Dir(flowFilePath))
	if f.Meta.Namespace == "" {
		f.Meta.Namespace = namespaceName
	}

	if f.Meta.Namespace != namespaceName {
		return models.Flow{}, "", fmt.Errorf("flow namespace %s does not match expected namespace %s", f.Meta.Namespace, namespaceName)
	}

	ns, err := c.store.GetNamespaceByName(context.Background(), f.Meta.Namespace)
	if err != nil {
		return models.Flow{}, "", fmt.Errorf("error getting namespace %s: %w", f.Meta.Namespace, err)
	}

	var schedules []struct {
		Cron     string
		Timezone string
	}
	for _, sched := range f.Schedules {
		schedules = append(schedules, struct {
			Cron     string
			Timezone string
		}{
			Cron:     sched.Cron,
			Timezone: sched.Timezone,
		})
	}

	prefixID, err := c.getOrCreateFlowPrefix(context.Background(), f.Meta.Prefix, ns.Uuid)
	if err != nil {
		return models.Flow{}, "", fmt.Errorf("failed to resolve prefix for flow %s: %w", f.Meta.ID, err)
	}

	fd, err := c.store.GetFlowBySlug(context.Background(), repo.GetFlowBySlugParams{
		Slug:     f.Meta.ID,
		Uuid:     ns.Uuid,
		IsActive: sql.NullBool{Valid: false},
	})
	if err != nil {
		fd, err = c.store.CreateFlowTx(context.Background(), repo.CreateFlowTxParams{
			Slug:        f.Meta.ID,
			Name:        f.Meta.Name,
			Description: f.Meta.Description,
			Checksum:    checksum,
			FilePath:    flowFilePath,
			Namespace:   f.Meta.Namespace,
			PrefixID:    prefixID,
			Schedules:   schedules,
		})
	} else if fd.Checksum != checksum {
		fd, err = c.store.UpdateFlowTx(context.Background(), repo.UpdateFlowTxParams{
			Slug:            f.Meta.ID,
			Name:            f.Meta.Name,
			Description:     f.Meta.Description,
			Checksum:        checksum,
			FilePath:        flowFilePath,
			Namespace:       f.Meta.Namespace,
			PrefixID:        prefixID,
			UserSchedulable: f.Meta.UserSchedulable,
			Schedules:       schedules,
		})
	}
	if err != nil {
		return models.Flow{}, "", fmt.Errorf("database operation failed for flow %s: %w", f.Meta.ID, err)
	}

	err = c.store.MarkFlowActive(context.Background(), repo.MarkFlowActiveParams{
		Slug: f.Meta.ID,
		Uuid: ns.Uuid,
	})
	if err != nil {
		return models.Flow{}, "", fmt.Errorf("failed to mark flow %s as active: %w", f.Meta.ID, err)
	}

	f.Meta.DBID = fd.ID
	return f, ns.Uuid.String(), nil
}

// GetScheduledFlows returns all flows that have a cron schedule configured
func (c *Core) GetScheduledFlows() []models.Flow {
	c.rwf.RLock()
	defer c.rwf.RUnlock()

	ctx := context.Background()
	scheduledFlowRows, err := c.store.GetScheduledFlows(ctx)
	if err != nil {
		log.Printf("error getting scheduled flows from database: %v", err)
		return nil
	}

	var scheduledFlows []models.Flow
	for _, row := range scheduledFlowRows {
		flowKey := fmt.Sprintf("%s:%s", row.Slug, row.NamespaceUuid.String())
		if flow, exists := c.flows[flowKey]; exists {
			scheduledFlows = append(scheduledFlows, flow)
		}
	}

	return scheduledFlows
}

// GetSchedulerFlow loads a flow and converts it to scheduler.Flow format
// This function can be used as a FlowLoaderFn for the scheduler
func (c *Core) GetSchedulerFlow(ctx context.Context, flowSlug string, namespaceUUID string) (scheduler.Flow, error) {
	// Load the flow from the in-memory cache
	flow, err := c.GetFlowByID(flowSlug, namespaceUUID)
	if err != nil {
		return scheduler.Flow{}, fmt.Errorf("failed to get flow %s: %w", flowSlug, err)
	}

	// Parse namespace UUID
	nsUUID, err := uuid.Parse(namespaceUUID)
	if err != nil {
		return scheduler.Flow{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	// Convert to scheduler format with nodes resolved
	return models.ConvertToSchedulerFlow(ctx, flow, nsUUID, c.GetNodesByNames, c.GetNodesByTags)
}

// removeDuplicateSchedules removes duplicate schedules from a slice
func removeDuplicateSchedules(schedules []models.Schedule) []models.Schedule {
	if len(schedules) == 0 {
		return schedules
	}

	seen := make(map[string]bool)
	result := make([]models.Schedule, 0, len(schedules))

	for _, sched := range schedules {
		key := sched.Cron + ":" + sched.Timezone
		if sched.Cron != "" && !seen[key] {
			seen[key] = true
			result = append(result, sched)
		}
	}

	return result
}

// SyncScheduledFlowJobs loads scheduled flows from the database and converts them to scheduled jobs
// This function can be used as a JobSyncerFn for the scheduler
func (c *Core) SyncScheduledFlowJobs(ctx context.Context) ([]scheduler.ScheduledJob, error) {
	flows, err := c.store.GetScheduledFlows(ctx)
	if err != nil {
		return nil, err
	}

	jobs := make([]scheduler.ScheduledJob, 0, len(flows))
	for _, flow := range flows {
		namespace, err := c.store.GetNamespaceByUUID(ctx, flow.NamespaceUuid)
		if err != nil {
			log.Printf("failed to get namespace for flow %s: %v", flow.Name, err)
			continue
		}

		schedulerFlow, err := c.GetSchedulerFlow(ctx, flow.Slug, flow.NamespaceUuid.String())
		if err != nil {
			log.Printf("failed to load flow %s: %v", flow.Slug, err)
			continue
		}

		input := make(map[string]interface{})

		if flow.IsUserCreated && flow.Inputs.Valid {
			var userInputs map[string]interface{}
			if err := json.Unmarshal(flow.Inputs.RawMessage, &userInputs); err != nil {
				log.Printf("failed to unmarshal user inputs for schedule %d: %v", flow.ScheduleID, err)
				continue
			}
			for k, v := range userInputs {
				input[k] = v
			}
		}

		userUUID := SystemUserUUID
		if flow.IsUserCreated {
			user, err := c.store.GetUserByID(ctx, flow.CreatedBy)
			if err != nil {
				log.Printf("failed to get user for schedule %d: %v", flow.ScheduleID, err)
			} else {
				userUUID = user.Uuid.String()
			}
		}

		modelFlow, err := c.GetFlowByID(flow.Slug, flow.NamespaceUuid.String())
		var overrideNodes []scheduler.Node
		if err == nil {
			overrideNodes, err = c.resolveOverrideNodes(ctx, modelFlow, input, flow.NamespaceUuid)
			if err != nil {
				log.Printf("failed to resolve override nodes for schedule %d: %v", flow.ScheduleID, err)
				overrideNodes = nil
			}
		}

		payload := scheduler.FlowExecutionPayload{
			Workflow:          schedulerFlow,
			Input:             input,
			StartingActionIdx: 0,
			NamespaceID:       namespace.Uuid.String(),
			TriggerType:       scheduler.TriggerTypeScheduled,
			UserUUID:          userUUID,
			FlowDirectory:     filepath.Dir(flow.FilePath),
			OverrideNodes:     overrideNodes,
		}

		jobs = append(jobs, scheduler.ScheduledJob{
			ID:          fmt.Sprintf("schedule_%d", flow.ScheduleID),
			Name:        fmt.Sprintf("%s (%s)", flow.Name, flow.Cron),
			Cron:        flow.Cron,
			Timezone:    flow.Timezone,
			PayloadType: scheduler.PayloadTypeFlowExecution,
			Payload:     payload,
		})
	}

	return jobs, nil
}

func (c *Core) CreateSchedule(ctx context.Context, flowID, cron, timezone string, inputs map[string]interface{}, userUUID, namespaceID string) (models.Schedule, error) {
	flow, err := c.GetFlowByID(flowID, namespaceID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("flow not found: %w", err)
	}

	if !flow.Meta.UserSchedulable {
		return models.Schedule{}, fmt.Errorf("user schedules are not enabled for this flow")
	}

	if verr := c.PrepareAndValidateInputs(ctx, &flow, namespaceID, inputs, ""); verr != nil {
		return models.Schedule{}, verr
	}

	existing, err := c.store.GetScheduleByFlowAndCron(ctx, repo.GetScheduleByFlowAndCronParams{
		FlowID:        flow.Meta.DBID,
		Cron:          cron,
		Timezone:      timezone,
		IsUserCreated: true,
	})
	if err == nil && existing.ID > 0 {
		return models.Schedule{}, fmt.Errorf("schedule with same cron expression already exists for this flow")
	}

	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("could not marshal inputs: %w", err)
	}

	userID, _ := uuid.Parse(userUUID)

	schedule, err := c.store.CreateUserSchedule(ctx, repo.CreateUserScheduleParams{
		FlowID:   flow.Meta.DBID,
		Cron:     cron,
		Timezone: timezone,
		Inputs:   pqtype.NullRawMessage{RawMessage: inputsJSON, Valid: inputsJSON != nil},
		Uuid:     userID,
	})
	if err != nil {
		return models.Schedule{}, fmt.Errorf("could not create schedule: %w", err)
	}

	return models.Schedule{
		UUID:          schedule.Uuid.String(),
		FlowSlug:      flow.Meta.ID,
		FlowName:      flow.Meta.Name,
		Cron:          schedule.Cron,
		Timezone:      schedule.Timezone,
		Inputs:        inputs,
		CreatedByID:   userUUID,
		IsActive:      schedule.IsActive,
		IsUserCreated: schedule.IsUserCreated,
		CreatedAt:     schedule.CreatedAt,
		UpdatedAt:     schedule.UpdatedAt,
	}, nil
}

func (c *Core) GetSchedule(ctx context.Context, scheduleUUID, userUUID, namespaceID string) (models.Schedule, error) {
	userID, err := uuid.Parse(userUUID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("invalid user UUID: %w", err)
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	schedID, err := uuid.Parse(scheduleUUID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("invalid schedule UUID: %w", err)
	}

	schedule, err := c.store.GetUserScheduleByUUID(ctx, repo.GetUserScheduleByUUIDParams{
		Uuid:   schedID,
		Uuid_2: userID,
		Uuid_3: namespaceUUID,
	})
	if err != nil {
		return models.Schedule{}, fmt.Errorf("schedule not found: %w", err)
	}

	var inputs map[string]interface{}
	if schedule.Inputs.Valid {
		if err := json.Unmarshal(schedule.Inputs.RawMessage, &inputs); err != nil {
			return models.Schedule{}, fmt.Errorf("could not unmarshal inputs: %w", err)
		}
	}

	return models.Schedule{
		UUID:          schedule.Uuid.String(),
		FlowSlug:      schedule.FlowSlug,
		FlowName:      schedule.FlowName,
		Cron:          schedule.Cron,
		Timezone:      schedule.Timezone,
		Inputs:        inputs,
		CreatedByID:   schedule.CreatedByUuid.String(),
		CreatedByName: schedule.CreatedByName,
		IsUserCreated: schedule.IsUserCreated,
		IsActive:      schedule.IsActive,
		CreatedAt:     schedule.CreatedAt,
		UpdatedAt:     schedule.UpdatedAt,
	}, nil
}

// ListSchedules returns a paginated list of all schedules (both user and system schedules)
func (c *Core) ListSchedules(ctx context.Context, flowSlug, userUUID, namespaceID string, limit, offset int) ([]models.Schedule, int64, int64, error) {
	userID, err := uuid.Parse(userUUID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid user UUID: %w", err)
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	var flowID int32
	if flowSlug != "" {
		flow, err := c.GetFlowByID(flowSlug, namespaceID)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("flow not found: %w", err)
		}
		flowID = flow.Meta.DBID
	}

	schedules, err := c.store.ListSchedules(ctx, repo.ListSchedulesParams{
		Column1: flowID,
		Uuid:    userID,
		Uuid_2:  namespaceUUID,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("could not list schedules: %w", err)
	}

	var result []models.Schedule
	var pageCount, totalCount int64

	for _, s := range schedules {
		var inputs map[string]interface{}
		if s.Inputs.Valid {
			if err := json.Unmarshal(s.Inputs.RawMessage, &inputs); err != nil {
				continue
			}
		}

		result = append(result, models.Schedule{
			UUID:          s.Uuid.String(),
			FlowSlug:      s.FlowSlug,
			FlowName:      s.FlowName,
			Cron:          s.Cron,
			Timezone:      s.Timezone,
			Inputs:        inputs,
			CreatedByID:   s.CreatedByUuid.String(),
			CreatedByName: s.CreatedByName,
			IsActive:      s.IsActive,
			IsUserCreated: s.IsUserCreated,
			CreatedAt:     s.CreatedAt,
			UpdatedAt:     s.UpdatedAt,
		})

		pageCount = s.PageCount
		totalCount = s.TotalCount
	}

	return result, pageCount, totalCount, nil
}

func (c *Core) UpdateSchedule(ctx context.Context, scheduleUUID, cron, timezone string, inputs map[string]interface{}, isActive bool, userUUID, namespaceID string) (models.Schedule, error) {
	userID, err := uuid.Parse(userUUID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("invalid user UUID: %w", err)
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	schedID, err := uuid.Parse(scheduleUUID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("invalid schedule UUID: %w", err)
	}

	existing, err := c.store.GetUserScheduleByUUID(ctx, repo.GetUserScheduleByUUIDParams{
		Uuid:   schedID,
		Uuid_2: userID,
		Uuid_3: namespaceUUID,
	})
	if err != nil {
		return models.Schedule{}, fmt.Errorf("schedule not found: %w", err)
	}

	flow, err := c.GetFlowByID(existing.FlowSlug, namespaceID)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("flow not found: %w", err)
	}

	if !flow.Meta.UserSchedulable {
		return models.Schedule{}, fmt.Errorf("user schedules are not enabled for this flow")
	}

	if verr := c.PrepareAndValidateInputs(ctx, &flow, namespaceID, inputs, ""); verr != nil {
		return models.Schedule{}, verr
	}

	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return models.Schedule{}, fmt.Errorf("could not marshal inputs: %w", err)
	}

	updated, err := c.store.UpdateUserScheduleByUUID(ctx, repo.UpdateUserScheduleByUUIDParams{
		Uuid:     schedID,
		Cron:     cron,
		Timezone: timezone,
		Inputs:   pqtype.NullRawMessage{RawMessage: inputsJSON, Valid: inputsJSON != nil},
		IsActive: isActive,
		Uuid_2:   userID,
		Uuid_3:   namespaceUUID,
	})
	if err != nil {
		return models.Schedule{}, fmt.Errorf("could not update schedule: %w", err)
	}

	return models.Schedule{
		UUID:          updated.Uuid.String(),
		FlowSlug:      existing.FlowSlug,
		FlowName:      existing.FlowName,
		Cron:          updated.Cron,
		Timezone:      updated.Timezone,
		Inputs:        inputs,
		IsActive:      updated.IsActive,
		IsUserCreated: updated.IsUserCreated,
		UpdatedAt:     updated.UpdatedAt,
	}, nil
}

func (c *Core) DeleteSchedule(ctx context.Context, scheduleUUID, userUUID, namespaceID string) error {
	userID, err := uuid.Parse(userUUID)
	if err != nil {
		return fmt.Errorf("invalid user UUID: %w", err)
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	schedID, err := uuid.Parse(scheduleUUID)
	if err != nil {
		return fmt.Errorf("invalid schedule UUID: %w", err)
	}

	rowsAffected, err := c.store.DeleteUserScheduleByUUID(ctx, repo.DeleteUserScheduleByUUIDParams{
		Uuid:   schedID,
		Uuid_2: userID,
		Uuid_3: namespaceUUID,
	})
	if err != nil {
		return fmt.Errorf("could not delete schedule: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("schedule not found or permission denied")
	}

	return nil
}

// PopulateRemoteOptions fetches remote options for all select inputs in the flow
// that have RemoteOptions configured and populates flow.Inputs[i].Options.
// namespaceID is used to look up flow secrets for header interpolation.
// inputVals can be nil when called at display time.
// Returns a map of inputName → fetch error for any inputs that failed; successful
// inputs are already populated in place. Callers decide how to handle each error.
func (c *Core) PopulateRemoteOptions(ctx context.Context, flow *models.Flow, namespaceID string, inputVals map[string]interface{}) map[string]error {
	errs := make(map[string]error)
	for i, input := range flow.Inputs {
		if input.Type != models.INPUT_TYPE_SELECT || input.RemoteOptions == nil {
			continue
		}
		secrets, err := c.GetMergedSecretsForFlow(ctx, flow.Meta.ID, namespaceID)
		if err != nil {
			secrets = make(map[string]string)
		}
		opts, err := c.ResolveRemoteOptions(ctx, *input.RemoteOptions, secrets, inputVals, nil)
		if err != nil {
			errs[input.Name] = err
			continue
		}
		flow.Inputs[i].Options = opts
	}
	return errs
}

// ResolveRemoteOptions fetches the list of options for a select input from the
// configured remote URL. The API must return {"options": ["a", "b", ...]}.
// secrets is used for {{ expression }} interpolation in header values.
// inputVals and outputs can be nil when called at /inputs time.
func (c *Core) ResolveRemoteOptions(ctx context.Context, ro models.RemoteOptions, secrets map[string]string, inputVals map[string]interface{}, outputs map[string]interface{}) ([]string, error) {
	method := ro.Method
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, ro.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("could not build remote options request: %w", err)
	}

	for k, v := range ro.Headers {
		interpolated, err := interpolateRemoteHeader(v, secrets, inputVals, outputs)
		if err != nil {
			return nil, fmt.Errorf("could not interpolate header %q: %w", k, err)
		}
		req.Header.Set(k, interpolated)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remote options request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remote options request returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read remote options response: %w", err)
	}

	var result struct {
		Options []string `json:"options"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("could not parse remote options response: %w", err)
	}

	return result.Options, nil
}

// interpolateRemoteHeader evaluates {{ expression }} placeholders in a header
// value using expr-lang. The available env keys are secrets, inputs, outputs.
func interpolateRemoteHeader(value string, secrets map[string]string, inputs map[string]interface{}, outputs map[string]interface{}) (string, error) {
	re := regexp.MustCompile(`{{\s*([^}]+)\s*}}`)

	if inputs == nil {
		inputs = make(map[string]interface{})
	}
	if outputs == nil {
		outputs = make(map[string]interface{})
	}

	var evalErr error
	result := re.ReplaceAllStringFunc(value, func(match string) string {
		if evalErr != nil {
			return ""
		}
		sub := re.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		exprStr := strings.TrimSpace(sub[1])

		env := map[string]interface{}{
			"secrets": secrets,
			"inputs":  inputs,
			"outputs": outputs,
		}

		program, err := expr.Compile(exprStr, expr.Env(env))
		if err != nil {
			evalErr = fmt.Errorf("failed to compile header expression %q: %w", exprStr, err)
			return ""
		}

		out, err := expr.Run(program, env)
		if err != nil {
			evalErr = fmt.Errorf("failed to evaluate header expression %q: %w", exprStr, err)
			return ""
		}

		if out == nil {
			return ""
		}
		return fmt.Sprintf("%v", out)
	})

	if evalErr != nil {
		return "", evalErr
	}
	return result, nil
}

type remoteOptionsCacheEntry struct {
	options   map[string][]string
	expiresAt time.Time
}

const remoteOptionsCacheTTL = 5 * time.Minute

// StoreRemoteOptionsCache stores a resolved options map (inputName → options) under
// a new random key and returns that key. The entry expires after remoteOptionsCacheTTL.
func (c *Core) StoreRemoteOptionsCache(options map[string][]string) string {
	b := make([]byte, 8)
	rand.Read(b)
	id := hex.EncodeToString(b)
	c.remoteOptionsCacheMu.Lock()
	c.remoteOptionsCache[id] = remoteOptionsCacheEntry{
		options:   options,
		expiresAt: time.Now().Add(remoteOptionsCacheTTL),
	}
	c.remoteOptionsCacheMu.Unlock()
	return id
}

// LookupRemoteOptionsCache returns the cached options map for the given ID,
// or nil if the entry is missing or has expired.
func (c *Core) LookupRemoteOptionsCache(id string) map[string][]string {
	c.remoteOptionsCacheMu.RLock()
	entry, ok := c.remoteOptionsCache[id]
	c.remoteOptionsCacheMu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}
	return entry.options
}

// PrepareAndValidateInputs resolves remote select options (from cache or by
// re-fetching) and validates the submitted inputs against the flow definition.
// optionsRequestID is the ID returned by a prior GetFlowInputs call; pass an
// empty string to always re-fetch. namespaceID is used to decrypt flow secrets.
func (c *Core) PrepareAndValidateInputs(ctx context.Context, flow *models.Flow, namespaceID string, inputs map[string]interface{}, optionsRequestID string) *models.FlowValidationError {
	if optionsRequestID != "" {
		if cached := c.LookupRemoteOptionsCache(optionsRequestID); cached != nil {
			for i, input := range flow.Inputs {
				if input.Type == models.INPUT_TYPE_SELECT && input.RemoteOptions != nil {
					if opts, ok := cached[input.Name]; ok {
						flow.Inputs[i].Options = opts
					}
				}
			}
		} else {
			// Cache miss — re-fetch
			if errs := c.PopulateRemoteOptions(ctx, flow, namespaceID, inputs); len(errs) > 0 {
				for name, err := range errs {
					return &models.FlowValidationError{FieldName: name, Msg: "could not fetch remote options", Err: err}
				}
			}
		}
	} else {
		if errs := c.PopulateRemoteOptions(ctx, flow, namespaceID, inputs); len(errs) > 0 {
			for name, err := range errs {
				return &models.FlowValidationError{FieldName: name, Msg: "could not fetch remote options", Err: err}
			}
		}
	}

	return flow.ValidateInput(inputs)
}
