package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/cvhariharan/autopilot/internal/tasks"
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
		return models.Flow{}, ErrFlowNotFound
	}

	return c.GetFlowByID(f)
}

// QueueFlowExecution adds a flow in the execution queue. The ID returned is the execution queue ID.
// Exec ID should be universally unique, this is used to create the log stream and identify each execution
func (c *Core) QueueFlowExecution(ctx context.Context, f models.Flow, input map[string]interface{}, execID string, userID int32) (string, error) {

	// store the mapping between logID and flowID
	c.logMap[execID] = f.Meta.ID

	task, err := tasks.NewFlowExecution(f, input, execID)
	if err != nil {
		return "", fmt.Errorf("error creating task: %v", err)
	}

	info, err := c.q.Enqueue(task)
	if err != nil {
		return "", err
	}

	inputB, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("could not marshal input for storing execution log: %w", err)
	}

	_, err = c.store.AddExecutionLog(ctx, repo.AddExecutionLogParams{
		ExecID:      execID,
		FlowID:      f.Meta.DBID,
		Input:       inputB,
		TriggeredBy: userID,
	})
	if err != nil {
		return "", fmt.Errorf("could not add entry to execution log: %w", err)
	}

	return info.ID, nil
}

func (c *Core) GetExecutionSummary(ctx context.Context, f models.Flow, triggeredBy int32) ([]models.ExecutionSummary, error) {
	execs, err := c.store.GetExecutionsByFlow(ctx, repo.GetExecutionsByFlowParams{
		FlowID:      f.Meta.DBID,
		TriggeredBy: triggeredBy,
	})
	if err != nil {
		return nil, fmt.Errorf("could not get executions for %s: %w", f.Meta.ID, err)
	}

	var m []models.ExecutionSummary
	for _, v := range execs {
		m = append(m, models.ExecutionSummary{ExecID: v.ExecID, CreatedAt: v.CreatedAt, CompletedAt: v.UpdatedAt, Status: models.ExecutionStatus(v.Status)})
	}

	return m, nil
}
