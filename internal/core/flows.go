package core

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

func (c *Core) GetFlowByID(id string, namespaceID string) (models.Flow, error) {
	f, ok := c.flows[fmt.Sprintf("%s:%s", id, namespaceID)]
	if !ok {
		return models.Flow{}, ErrFlowNotFound
	}

	return f, nil
}

func (c *Core) GetAllFlows(ctx context.Context, namespaceID string) ([]models.Flow, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	flows, err := c.store.GetFlowsByNamespace(ctx, namespaceUUID)
	if err != nil {
		return nil, fmt.Errorf("could not get flows for namespace %s: %w", namespaceID, err)
	}

	var fs []models.Flow
	for _, v := range flows {
		fs = append(fs, c.flows[fmt.Sprintf("%s:%s", v.Slug, namespaceID)])
	}
	return fs, nil
}

func (c *Core) GetFlowsPaginated(ctx context.Context, namespaceID string, limit, offset int) ([]models.Flow, int64, int64, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	flows, err := c.store.ListFlowsPaginated(ctx, repo.ListFlowsPaginatedParams{
		Uuid:   namespaceUUID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("could not get paginated flows for namespace %s: %w", namespaceID, err)
	}

	var fs []models.Flow
	var pageCount, totalCount int64
	
	for _, v := range flows {
		fs = append(fs, c.flows[fmt.Sprintf("%s:%s", v.Slug, namespaceID)])
		pageCount = v.PageCount
		totalCount = v.TotalCount
	}

	return fs, pageCount, totalCount, nil
}

func (c *Core) SearchFlows(ctx context.Context, namespaceID string, query string, limit, offset int) ([]models.Flow, int64, int64, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	flows, err := c.store.SearchFlowsPaginated(ctx, repo.SearchFlowsPaginatedParams{
		Uuid:    namespaceUUID,
		Column2: query,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("could not search flows for namespace %s: %w", namespaceID, err)
	}

	var fs []models.Flow
	var pageCount, totalCount int64
	
	for _, v := range flows {
		fs = append(fs, c.flows[fmt.Sprintf("%s:%s", v.Slug, namespaceID)])
		pageCount = v.PageCount
		totalCount = v.TotalCount
	}

	return fs, pageCount, totalCount, nil
}

func (c *Core) GetFlowFromLogID(logID string, namespaceID string) (models.Flow, error) {
	f, ok := c.logMap[logID]
	if !ok {
		df, err := c.store.GetFlowFromExecID(context.Background(), logID)
		if err != nil {
			return models.Flow{}, fmt.Errorf("could not get flow for exec id %s: %w", logID, err)
		}
		return c.GetFlowByID(df.Slug, namespaceID)
	}

	return c.GetFlowByID(f, namespaceID)
}

// QueueFlowExecution adds a flow in the execution queue. The ID returned is the execution queue ID.
// Exec ID should be universally unique, this is used to create the log stream and identify each execution
func (c *Core) QueueFlowExecution(ctx context.Context, f models.Flow, input map[string]interface{}, userUUID string, namespaceID string) (string, error) {

	info, err := c.queueFlow(ctx, f, input, "", 0, userUUID, namespaceID)
	if err != nil {
		return "", err
	}

	return info, nil
}

// ResumeFlowExecution moves the task to a resume queue for further processing.
func (c *Core) ResumeFlowExecution(ctx context.Context, execID string, actionID string, userUUID string, namespaceID string) error {
	exec, err := c.GetExecutionByExecID(ctx, execID)
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

	if _, err := c.queueFlow(ctx, f, exec.Input, execID, actionIndex, userUUID, namespaceID); err != nil {
		return err
	}

	return nil
}

// getNodesByNames retrieves nodes by their names and returns a slice of models.Node
// This is used as a lookup function for converting flows to task models
func (c *Core) getNodesByNames(ctx context.Context, nodeNames []string, namespaceUUID uuid.UUID) ([]models.Node, error) {
	if len(nodeNames) == 0 {
		return nil, nil
	}

	n, err := c.store.GetNodesByNames(ctx, repo.GetNodesByNamesParams{
		Column1: nodeNames,
		Uuid:    namespaceUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("could not get nodes by names %v: %w", nodeNames, err)
	}

	var nodes []models.Node
	for _, v := range n {
		var key string
		if v.AuthMethod == repo.AuthenticationMethod(models.AuthMethodSSHKey) {
			key = v.CredentialPrivateKey.String
		} else {
			key = v.CredentialPassword.String
		}

		// decrypt the key
		dKey, err := hex.DecodeString(key)
		if err != nil {
			return nil, fmt.Errorf("could not decode key for node %s: %w", v.Name, err)
		}

		decryptedKey, err := c.keeper.Decrypt(ctx, []byte(dKey))
		if err != nil {
			return nil, fmt.Errorf("could not decrypt key for node %s: %w", v.Name, err)
		}

		nodes = append(nodes, models.Node{
			ID:       v.Uuid.String(),
			Name:     v.Name,
			Hostname: v.Hostname,
			Port:     int(v.Port),
			Username: v.Username,
			OSFamily: v.OsFamily,
			Tags:     v.Tags,
			Auth: models.NodeAuth{
				CredentialID: v.CredentialUuid.UUID.String(),
				Method:       models.AuthMethod(v.AuthMethod),
				Key:          string(decryptedKey),
			},
		})
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes found for names %v", nodeNames)
	}

	return nodes, nil
}

// queueFlow adds a flow to the execution queue. If the actionIndex is not zero, it is moved to a resume queue.
func (c *Core) queueFlow(ctx context.Context, f models.Flow, input map[string]interface{}, parentExecID string, actionIndex int, userUUID string, namespaceID string) (string, error) {
	execID := uuid.NewString()
	// store the mapping between logID and flowID
	c.logMap[execID] = f.Meta.ID

	userID, err := uuid.Parse(userUUID)
	if err != nil {
		return "", fmt.Errorf("user id is not a UUID: %w", err)
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return "", fmt.Errorf("invalid namespace UUID: %w", err)
	}

	taskFlow, err := models.ToTaskFlowModel(f, func(nodeNames []string) ([]models.Node, error) {
		return c.getNodesByNames(ctx, nodeNames, namespaceUUID)
	})
	if err != nil {
		return "", fmt.Errorf("error converting flow to task model: %w", err)
	}

	log.Println(taskFlow)

	task, err := tasks.NewFlowExecution(taskFlow, input, actionIndex, execID, parentExecID, namespaceID)
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

func (c *Core) GetAllExecutionSummary(ctx context.Context, f models.Flow, triggeredBy string, namespaceID string) ([]models.ExecutionSummary, error) {
	userID, err := uuid.Parse(triggeredBy)
	if err != nil {
		return nil, fmt.Errorf("user id is not a UUID: %w", err)
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	execs, err := c.store.GetExecutionsByFlow(ctx, repo.GetExecutionsByFlowParams{
		ID:     f.Meta.DBID,
		Uuid:   userID,
		Uuid_2: namespaceUUID,
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

func (c *Core) GetExecutionSummaryByExecID(ctx context.Context, execID string, namespaceID string) (models.ExecutionSummary, error) {
	e, err := c.store.GetExecutionByExecID(ctx, execID)
	if err != nil {
		return models.ExecutionSummary{}, fmt.Errorf("could not get exec %s by exec id: %w", execID, err)
	}

	f, err := c.GetFlowFromLogID(execID, namespaceID)
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
