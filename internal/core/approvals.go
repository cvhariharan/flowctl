package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/cvhariharan/flowctl/internal/scheduler"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrNoPendingApproval = errors.New("no pending approval")
	ErrNil               = errors.New("not found")
)

const (
	ApprovalIDPrefix     = "approval:execid:%d"
	ApprovalUUIDPrefix   = "approval:uuid:%s"
	ApprovalCacheTimeout = 1 * time.Hour
)

// Helper function to cache approval in both locations
func (c *Core) cacheApproval(ctx context.Context, execID int32, approval models.ApprovalRequest) error {
	// Cache by execID
	if err := c.redisClient.Set(ctx,
		fmt.Sprintf(ApprovalIDPrefix, execID),
		approval,
		ApprovalCacheTimeout).Err(); err != nil {
		return fmt.Errorf("failed to cache approval by execID: %w", err)
	}

	// Cache by UUID
	if err := c.redisClient.Set(ctx,
		fmt.Sprintf(ApprovalUUIDPrefix, approval.UUID),
		approval,
		ApprovalCacheTimeout).Err(); err != nil {
		return fmt.Errorf("failed to cache approval by UUID: %w", err)
	}

	return nil
}

// ApproveOrRejectAction handles approval or rejection of an action request by a user.
// It takes the approval UUID, the ID of the user making the decision, and the approval status.
// The function updates both the database and Redis cache with the decision.
// Once approved, the task is moved to a resume queue for further processing.
func (c *Core) ApproveOrRejectAction(ctx context.Context, approvalUUID, decidedBy string, status models.ApprovalType, namespaceID string) error {
	var err error
	uid, err := uuid.Parse(approvalUUID)
	if err != nil {
		return fmt.Errorf("approval UUID is not a UUID: %w", err)
	}

	areq, err := c.GetApprovalRequest(ctx, approvalUUID, namespaceID)
	if err != nil {
		return fmt.Errorf("could not retrieve approval request %s: %w", approvalUUID, err)
	}

	if areq.Status != models.ApprovalStatusPending {
		return fmt.Errorf("request has already been processed")
	}

	userid, err := uuid.Parse(decidedBy)
	if err != nil {
		return fmt.Errorf("decidedby UUID is not a UUID: %w", err)
	}

	user, err := c.store.GetUserByUUID(ctx, userid)
	if err != nil {
		return err
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	var cancellationNote string
	if status == models.ApprovalStatusRejected {
		cancellationNote = fmt.Sprintf("Flow execution cancelled due to approval rejection by %s", user.Name)
	}

	// Process approval decision
	result, err := c.store.ProcessApprovalDecisionTx(ctx, repo.ApprovalDecisionTxParams{
		ApprovalUUID:     uid,
		NamespaceUUID:    namespaceUUID,
		DecidedByUserID:  user.ID,
		Status:           repo.ApprovalStatus(status),
		CancellationNote: cancellationNote,
	})
	if err != nil {
		return fmt.Errorf("could not process approval decision for %s: %w", approvalUUID, err)
	}

	approval := models.ApprovalRequest{
		UUID:        result.Uuid.String(),
		Status:      models.ApprovalType(result.Status),
		ActionID:    result.ActionID,
		ExecID:      result.ExecID,
		RequestedBy: result.RequestedBy,
	}

	// Update the cache using approval UUID
	if err := c.cacheApproval(ctx, result.ExecLogID, approval); err != nil {
		return err
	}

	// If approved, move to resume queue
	if status == models.ApprovalStatusApproved {
		if err := c.ResumeFlowExecution(ctx, result.ExecID, approval.ActionID, decidedBy, namespaceID); err != nil {
			return fmt.Errorf("could not resume task %s: %w", result.ExecID, err)
		}
	}

	return nil
}

func (c *Core) RequestApproval(ctx context.Context, execID string, action models.Action, namespaceID string) (string, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return "", fmt.Errorf("invalid namespace UUID: %w", err)
	}
	exec, err := c.store.GetExecutionByExecID(ctx, repo.GetExecutionByExecIDParams{
		ExecID: execID,
		Uuid:   namespaceUUID,
	})
	if err != nil {
		return "", fmt.Errorf("error getting execution for exec ID %s: %w", execID, err)
	}

	var approvalReq models.ApprovalRequest
	err = c.redisClient.Get(ctx, fmt.Sprintf(ApprovalIDPrefix, exec.ID)).Scan(&approvalReq)
	if err != nil && !errors.Is(err, redis.Nil) {
		return "", fmt.Errorf("error performing existing approval request check: %w", err)
	}

	if approvalReq.Status == models.ApprovalStatusPending {
		return "", fmt.Errorf("pending approval request: %s", approvalReq.UUID)
	}

	areq, err := c.store.RequestApprovalTx(ctx, execID, namespaceUUID, repo.RequestApprovalParam{ID: action.ID})
	if err != nil {
		return "", err
	}

	approvalReq = models.ApprovalRequest{
		UUID:        areq.Uuid.String(),
		Status:      models.ApprovalType(areq.Status),
		ActionID:    action.ID,
		ExecID:      execID,
		RequestedBy: areq.RequestedBy,
	}

	if err := c.cacheApproval(ctx, exec.ID, approvalReq); err != nil {
		return "", err
	}

	return areq.Uuid.String(), nil
}

func (c *Core) GetApprovalsRequestsForExec(ctx context.Context, execID string, namespaceID string) (models.ApprovalRequest, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return models.ApprovalRequest{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}
	exec, err := c.store.GetExecutionByExecID(ctx, repo.GetExecutionByExecIDParams{
		ExecID: execID,
		Uuid:   namespaceUUID,
	})
	if err != nil {
		return models.ApprovalRequest{}, fmt.Errorf("error getting execution for exec ID %s: %w", execID, err)
	}

	var existingReq models.ApprovalRequest
	err = c.redisClient.Get(ctx, fmt.Sprintf(ApprovalIDPrefix, exec.ID)).Scan(&existingReq)
	if err != nil && !errors.Is(err, redis.Nil) {
		return models.ApprovalRequest{}, fmt.Errorf("error getting pending approval request for %s: %w", execID, err)
	}

	// Get from DB
	if errors.Is(err, redis.Nil) {
		areq, err := c.store.GetApprovalRequestForExec(ctx, repo.GetApprovalRequestForExecParams{
			ExecID: execID,
			Uuid:   namespaceUUID,
		})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return models.ApprovalRequest{}, fmt.Errorf("could not get approval request from DB for exec %s: %w", execID, err)
		}

		if errors.Is(err, sql.ErrNoRows) {
			return models.ApprovalRequest{}, ErrNil
		}

		existingReq = models.ApprovalRequest{
			UUID:        areq.Uuid.String(),
			Status:      models.ApprovalType(areq.Status),
			ActionID:    areq.ActionID,
			ExecID:      execID,
			RequestedBy: areq.RequestedBy,
		}

		if err := c.cacheApproval(ctx, exec.ID, existingReq); err != nil {
			return models.ApprovalRequest{}, err
		}
	}

	return existingReq, nil
}

func (c *Core) BeforeActionHook(ctx context.Context, execID string, action scheduler.Action, namespaceID string) error {
	// use parent exec ID if available for approval requests
	eID := execID

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	// Set the current action ID
	log.Println("current action ID: ", action.ID)
	if _, err := c.store.UpdateExecutionActionID(ctx, repo.UpdateExecutionActionIDParams{
		CurrentActionID: sql.NullString{String: action.ID, Valid: action.ID != ""},
		ExecID:          execID,
		Uuid:            namespaceUUID,
	}); err != nil {
		return fmt.Errorf("could not update current action ID in exec %s: %w", execID, err)
	}

	if !action.Approval {
		return nil
	}

	// check if pending approval, exit if not approved
	a, err := c.store.GetApprovalRequestForActionAndExec(ctx, repo.GetApprovalRequestForActionAndExecParams{
		ExecID:   eID,
		ActionID: action.ID,
		Uuid:     namespaceUUID,
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	// continue execution if approved
	if a.Status == repo.ApprovalStatusApproved {
		return nil
	}

	if a.Status == repo.ApprovalStatusRejected {
		return fmt.Errorf("request for running action %q is rejected", action.Name)
	}

	if a.Status == "" {
		_, err = c.RequestApproval(ctx, eID, models.SchedulerActionToAction(action), namespaceID)
		if err != nil {
			return err
		}
	}

	return scheduler.ErrPendingApproval
}

func (c *Core) GetApprovalRequest(ctx context.Context, approvalUUID string, namespaceID string) (models.ApprovalRequest, error) {
	var approval models.ApprovalRequest
	err := c.redisClient.Get(ctx, fmt.Sprintf(ApprovalUUIDPrefix, approvalUUID)).Scan(&approval)
	if err != nil && !errors.Is(err, redis.Nil) {
		return models.ApprovalRequest{}, fmt.Errorf("error getting approval request by UUID %s: %w", approvalUUID, err)
	}

	if errors.Is(err, redis.Nil) {
		uid, err := uuid.Parse(approvalUUID)
		if err != nil {
			return models.ApprovalRequest{}, fmt.Errorf("invalid approval UUID: %w", err)
		}

		namespaceUUID, err := uuid.Parse(namespaceID)
		if err != nil {
			return models.ApprovalRequest{}, fmt.Errorf("invalid namespace UUID: %w", err)
		}

		areq, err := c.store.GetApprovalByUUID(ctx, repo.GetApprovalByUUIDParams{
			Uuid:   uid,
			Uuid_2: namespaceUUID,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return models.ApprovalRequest{}, ErrNil
			}
			return models.ApprovalRequest{}, fmt.Errorf("error getting approval from store: %w", err)
		}

		exec, err := c.store.GetExecutionByID(ctx, repo.GetExecutionByIDParams{
			ID:   areq.ExecLogID,
			Uuid: namespaceUUID,
		})
		if err != nil {
			return models.ApprovalRequest{}, fmt.Errorf("error getting execution: %w", err)
		}

		approval = models.ApprovalRequest{
			UUID:        areq.Uuid.String(),
			Status:      models.ApprovalType(areq.Status),
			ActionID:    areq.ActionID,
			ExecID:      exec.ExecID,
			RequestedBy: areq.RequestedBy,
		}

		if err := c.cacheApproval(ctx, areq.ExecLogID, approval); err != nil {
			return models.ApprovalRequest{}, fmt.Errorf("error caching approval: %w", err)
		}
	}

	log.Printf("Approval request: %+v\n", approval)

	return approval, nil
}

func (c *Core) GetApprovalWithInputs(ctx context.Context, approvalUUID string, namespaceID string) (models.ApprovalDetails, error) {
	uid, err := uuid.Parse(approvalUUID)
	if err != nil {
		return models.ApprovalDetails{}, fmt.Errorf("invalid approval UUID: %w", err)
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return models.ApprovalDetails{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	approval, err := c.store.GetApprovalWithInputsByUUID(ctx, repo.GetApprovalWithInputsByUUIDParams{
		Uuid:   uid,
		Uuid_2: namespaceUUID,
	})
	if err != nil {
		return models.ApprovalDetails{}, fmt.Errorf("failed to get approval with inputs: %w", err)
	}

	// Convert to model
	details := models.ApprovalDetails{
		ApprovalRequest: models.ApprovalRequest{
			UUID:        approval.Uuid.String(),
			ActionID:    approval.ActionID,
			Status:      models.ApprovalType(approval.Status),
			ExecID:      approval.ExecID,
			RequestedBy: approval.RequestedBy,
		},
		Inputs:    approval.ExecInputs,
		FlowName:  approval.FlowName,
		FlowID:    approval.FlowSlug,
		CreatedAt: approval.CreatedAt.Format(time.RFC3339),
		UpdatedAt: approval.UpdatedAt.Format(time.RFC3339),
	}

	return details, nil
}

func (c *Core) GetApprovalsPaginated(ctx context.Context, namespaceID, status, filter string, page, countPerPage int) ([]repo.GetApprovalsPaginatedRow, int64, int64, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, -1, -1, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	offset := (page - 1) * countPerPage

	approvals, err := c.store.GetApprovalsPaginated(ctx, repo.GetApprovalsPaginatedParams{
		Uuid:    namespaceUUID,
		Column2: status,
		Column3: filter,
		Limit:   int32(countPerPage),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, -1, -1, fmt.Errorf("failed to get paginated approvals: %w", err)
	}

	if len(approvals) == 0 {
		return approvals, 0, 0, nil
	}

	return approvals, approvals[0].PageCount, approvals[0].TotalCount, nil
}
