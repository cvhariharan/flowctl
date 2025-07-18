package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/cvhariharan/autopilot/internal/tasks"
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

	var approval models.ApprovalRequest
	var execLogID int32
	switch status {
	case models.ApprovalStatusApproved:
		a, err := c.store.ApproveRequestByUUID(ctx, repo.ApproveRequestByUUIDParams{
			Uuid:      uid,
			DecidedBy: sql.NullInt32{Int32: user.ID, Valid: true},
			Uuid_2:    namespaceUUID,
		})
		if err != nil {
			return fmt.Errorf("could not approve request %s: %w", approvalUUID, err)
		}

		approvers, err := models.ConvertJSONApproversToList(a.Approvers)
		if err != nil {
			return fmt.Errorf("could not convert approvers to list: %w", err)
		}
		approval = models.ApprovalRequest{
			UUID:        a.Uuid.String(),
			Status:      models.ApprovalType(a.Status),
			ActionID:    a.ActionID,
			RequestedBy: a.RequestedBy,
			Approvers:   approvers,
		}
		execLogID = a.ExecLogID
	case models.ApprovalStatusRejected:
		a, err := c.store.RejectRequestByUUID(ctx, repo.RejectRequestByUUIDParams{
			Uuid:      uid,
			DecidedBy: sql.NullInt32{Int32: user.ID, Valid: true},
			Uuid_2:    namespaceUUID,
		})
		if err != nil {
			return fmt.Errorf("could not reject request %s: %w", approvalUUID, err)
		}
		approvers, err := models.ConvertJSONApproversToList(a.Approvers)
		if err != nil {
			return fmt.Errorf("could not convert approvers to list: %w", err)
		}
		approval = models.ApprovalRequest{
			UUID:        a.Uuid.String(),
			Status:      models.ApprovalType(a.Status),
			ActionID:    a.ActionID,
			RequestedBy: a.RequestedBy,
			Approvers:   approvers,
		}
		execLogID = a.ExecLogID
	}

	exec, err := c.store.GetExecutionByID(ctx, repo.GetExecutionByIDParams{
		ID:   execLogID,
		Uuid: namespaceUUID,
	})
	if err != nil {
		return fmt.Errorf("could not get execution for approval %s: %w", approvalUUID, err)
	}
	approval.ExecID = exec.ExecID

	// Update the cache using approval UUID
	if err := c.cacheApproval(ctx, execLogID, approval); err != nil {
		return err
	}

	// If approved, move to resume queue
	if status == models.ApprovalStatusApproved {
		if err := c.ResumeFlowExecution(ctx, exec.ExecID, approval.ActionID, decidedBy, namespaceID); err != nil {
			return fmt.Errorf("could not resume task %s: %w", exec.ExecID, err)
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

	areq, err := c.store.RequestApprovalTx(ctx, execID, namespaceUUID, repo.RequestApprovalParam{ID: action.ID, Approvers: action.Approval})
	if err != nil {
		return "", err
	}

	approvers, err := models.ConvertJSONApproversToList(areq.Approvers)
	if err != nil {
		return "", fmt.Errorf("could not convert approvers to list: %w", err)
	}

	approvalReq = models.ApprovalRequest{
		UUID:        areq.Uuid.String(),
		Status:      models.ApprovalType(areq.Status),
		ActionID:    action.ID,
		ExecID:      execID,
		RequestedBy: areq.RequestedBy,
		Approvers:   approvers,
	}

	if err := c.cacheApproval(ctx, exec.ID, approvalReq); err != nil {
		return "", err
	}

	return areq.Uuid.String(), nil
}

func (c *Core) GetPendingApprovalsForExec(ctx context.Context, execID string, namespaceID string) (models.ApprovalRequest, error) {
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
		namespaceUUID, err := uuid.Parse(namespaceID)
		if err != nil {
			return models.ApprovalRequest{}, fmt.Errorf("invalid namespace UUID: %w", err)
		}

		areq, err := c.store.GetPendingApprovalRequestForExec(ctx, repo.GetPendingApprovalRequestForExecParams{
			ExecID: execID,
			Uuid:   namespaceUUID,
		})
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return models.ApprovalRequest{}, fmt.Errorf("could not get approval request from DB for exec %s: %w", execID, err)
		}

		if errors.Is(err, sql.ErrNoRows) {
			return models.ApprovalRequest{}, ErrNil
		}

		approvers, err := models.ConvertJSONApproversToList(areq.Approvers)
		if err != nil {
			return models.ApprovalRequest{}, fmt.Errorf("could not convert approvers to list: %w", err)
		}

		existingReq = models.ApprovalRequest{
			UUID:        areq.Uuid.String(),
			Status:      models.ApprovalType(areq.Status),
			ActionID:    areq.ActionID,
			ExecID:      execID,
			RequestedBy: areq.RequestedBy,
			Approvers:   approvers,
		}

		if err := c.cacheApproval(ctx, exec.ID, existingReq); err != nil {
			return models.ApprovalRequest{}, err
		}
	}

	if existingReq.Status == models.ApprovalStatusPending {
		return existingReq, nil
	}

	return models.ApprovalRequest{}, nil
}

func (c *Core) BeforeActionHook(ctx context.Context, execID, parentExecID string, action tasks.Action, namespaceID string) error {
	if len(action.Approval) == 0 {
		return nil
	}

	// use parent exec ID if available for approval requests
	eID := execID
	if parentExecID != "" {
		eID = parentExecID
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
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
		_, err = c.RequestApproval(ctx, eID, models.TaskActionToAction(action), namespaceID)
		if err != nil {
			return err
		}
	}

	return tasks.ErrPendingApproval
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

		approvers, err := models.ConvertJSONApproversToList(areq.Approvers)
		if err != nil {
			return models.ApprovalRequest{}, fmt.Errorf("could not convert approvers to list: %w", err)
		}

		approval = models.ApprovalRequest{
			UUID:        areq.Uuid.String(),
			Status:      models.ApprovalType(areq.Status),
			ActionID:    areq.ActionID,
			ExecID:      exec.ExecID,
			RequestedBy: areq.RequestedBy,
			Approvers:   approvers,
		}

		if err := c.cacheApproval(ctx, areq.ExecLogID, approval); err != nil {
			return models.ApprovalRequest{}, fmt.Errorf("error caching approval: %w", err)
		}
	}

	log.Printf("Approval request: %+v\n", approval)

	return approval, nil
}
