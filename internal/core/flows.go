package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/cvhariharan/autopilot/internal/tasks"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

var (
	ErrFlowNotFound = errors.New("flow not found")
)

func (c *Core) GetFlowByID(id string) (models.Flow, error) {
	f, ok := c.flows[id]
	if !ok {
		return models.Flow{}, ErrFlowNotFound
	}

	return f, nil
}

func (c *Core) GetAllFlows() ([]models.Flow, error) {
	var fs []models.Flow
	for _, v := range c.flows {
		fs = append(fs, v)
	}
	return fs, nil
}

func (c *Core) GetFlowFromLogID(logID string) (models.Flow, error) {
	f, ok := c.logMap[logID]
	if !ok {
		df, err := c.store.GetFlowFromExecID(context.Background(), logID)
		if err != nil {
			return models.Flow{}, fmt.Errorf("could not get flow for exec id %s: %w", logID, err)
		}
		return c.GetFlowByID(df.Slug)
	}

	return c.GetFlowByID(f)
}

// QueueFlowExecution adds a flow in the execution queue. The ID returned is the execution queue ID.
// Exec ID should be universally unique, this is used to create the log stream and identify each execution
func (c *Core) QueueFlowExecution(ctx context.Context, f models.Flow, input map[string]interface{}, userUUID string) (string, error) {

	info, err := c.queueFlow(ctx, f, input, "", 0, userUUID)
	if err != nil {
		return "", err
	}

	return info, nil
}

// ResumeFlowExecution moves the task to a resume queue for further processing.
func (c *Core) ResumeFlowExecution(ctx context.Context, execID string, actionID string, userUUID string) error {
	exec, err := c.GetExecutionByExecID(ctx, execID)
	if err != nil {
		return fmt.Errorf("could not get exec %s: %w", execID, err)
	}

	f, err := c.GetFlowFromLogID(execID)
	if err != nil {
		return err
	}

	actionIndex, err := f.GetActionIndexByID(actionID)
	if err != nil {
		return err
	}

	if _, err := c.queueFlow(ctx, f, exec.Input, execID, actionIndex, userUUID); err != nil {
		return err
	}

	return nil
}

// queueFlow adds a flow to the execution queue. If the actionIndex is not zero, it is moved to a resume queue.
func (c *Core) queueFlow(ctx context.Context, f models.Flow, input map[string]interface{}, parentExecID string, actionIndex int, userUUID string) (string, error) {
	execID := uuid.NewString()
	// store the mapping between logID and flowID
	c.logMap[execID] = f.Meta.ID

	userID, err := uuid.Parse(userUUID)
	if err != nil {
		return "", fmt.Errorf("user id is not a UUID: %w", err)
	}

	task, err := tasks.NewFlowExecution(models.ToTaskFlowModel(f), input, actionIndex, execID, parentExecID)
	if err != nil {
		return "", fmt.Errorf("error creating task: %v", err)
	}

	queue := "default"
	if actionIndex > 0 {
		queue = "resume"
	}
	_, err = c.q.Enqueue(task, asynq.Retention(24*time.Hour), asynq.Queue(queue))
	if err != nil {
		return "", err
	}

	inputB, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("could not marshal input to json: %w", err)
	}

	_, err = c.store.AddExecutionLog(ctx, repo.AddExecutionLogParams{
		ExecID:       execID,
		ParentExecID: sql.NullString{String: parentExecID, Valid: len(parentExecID) > 0},
		FlowID:       f.Meta.DBID,
		Input:        inputB,
		Uuid:         userID,
	})
	if err != nil {
		return "", fmt.Errorf("could not add entry to execution log: %w", err)
	}

	return execID, err
}

func (c *Core) GetAllExecutionSummary(ctx context.Context, f models.Flow, triggeredBy string) ([]models.ExecutionSummary, error) {
	userID, err := uuid.Parse(triggeredBy)
	if err != nil {
		return nil, fmt.Errorf("user id is not a UUID: %w", err)
	}

	execs, err := c.store.GetExecutionsByFlow(ctx, repo.GetExecutionsByFlowParams{
		FlowID: f.Meta.DBID,
		Uuid:   userID,
	})
	if err != nil {
		return nil, fmt.Errorf("could not get executions for %s: %w", f.Meta.ID, err)
	}

	var m []models.ExecutionSummary
	for _, v := range execs {

		m = append(m, models.ExecutionSummary{
			ExecID:      v.ExecID,
			CreatedAt:   v.CreatedAt,
			CompletedAt: v.UpdatedAt,
			Status:      models.ExecutionStatus(v.Status),
		})
	}

	return m, nil
}

func (c *Core) GetExecutionSummaryByExecID(ctx context.Context, execID string) (models.ExecutionSummary, error) {
	e, err := c.store.GetExecutionByExecID(ctx, execID)
	if err != nil {
		return models.ExecutionSummary{}, fmt.Errorf("could not get exec %s by exec id: %w", execID, err)
	}

	f, err := c.GetFlowFromLogID(execID)
	if err != nil {
		return models.ExecutionSummary{}, fmt.Errorf("could not get flow for exec %s: %w", execID, err)
	}

	return models.ExecutionSummary{
		ExecID:      execID,
		Flow:        f,
		CreatedAt:   e.CreatedAt,
		CompletedAt: e.UpdatedAt,
		TriggeredBy: e.TriggeredByUuid.String(),
	}, nil
}

func (c *Core) GetInputForExec(ctx context.Context, execID string) (map[string]interface{}, error) {
	var input map[string]interface{}
	in, err := c.store.GetInputForExecByUUID(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("error getting input for %s: %w", execID, err)
	}

	if err := json.Unmarshal(in, &input); err != nil {
		return nil, fmt.Errorf("error unmarshaling input for %s: %w", execID, err)
	}

	return input, nil
}

func (c *Core) GetExecutionByExecID(ctx context.Context, execID string) (models.Execution, error) {
	e, err := c.store.GetExecutionByExecID(ctx, execID)
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
		ExecID:       e.ExecID,
		ParentExecID: e.ParentExecID.String,
		Input:        input,
		ErrorMsg:     e.Error.String,
		TriggeredBy:  u.Uuid.String(),
	}, nil
}
