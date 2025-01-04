package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cvhariharan/autopilot/internal/models"
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
func (c *Core) QueueFlowExecution(ctx context.Context, f models.Flow, input map[string]interface{}, execID string, userUUID string) (string, error) {

	// store the mapping between logID and flowID
	c.logMap[execID] = f.Meta.ID

	task, err := tasks.NewFlowExecution(f, input, 2, execID)
	if err != nil {
		return "", fmt.Errorf("error creating task: %v", err)
	}

	info, err := c.q.Enqueue(task, asynq.Retention(24*time.Hour))
	if err != nil {
		return "", err
	}

	inputB, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("could not marshal input for storing execution log: %w", err)
	}

	userID, err := uuid.Parse(userUUID)
	if err != nil {
		return "", fmt.Errorf("user id is not a UUID: %w", err)
	}

	_, err = c.store.AddExecutionLog(ctx, repo.AddExecutionLogParams{
		ExecID: execID,
		FlowID: f.Meta.DBID,
		Input:  inputB,
		Uuid:   userID,
	})
	if err != nil {
		return "", fmt.Errorf("could not add entry to execution log: %w", err)
	}

	return info.ID, nil
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
