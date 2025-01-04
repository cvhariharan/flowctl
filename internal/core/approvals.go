package core

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrPendingApproval = errors.New("pending approval for action")
)

const (
	ApprovalIDPrefix     = "approval:%d"
	ApprovalCacheTimeout = 1 * time.Hour
)

func (c *Core) ApproveOrRejectAction(ctx context.Context, approvalUUID string, status models.ApprovalType) error {
	var err error
	uid, err := uuid.Parse(approvalUUID)
	if err != nil {
		return fmt.Errorf("approval UUID is not a UUID: %w", err)
	}

	var approval repo.Approval
	switch status {
	case models.ApprovalStatusApproved:
		approval, err = c.store.ApproveRequestByUUID(ctx, uid)
		if err != nil {
			return fmt.Errorf("could not approve request %s: %w", approvalUUID, err)
		}
	case models.ApprovalStatusRejected:
		approval, err = c.store.RejectRequestByUUID(ctx, uid)
		if err != nil {
			return fmt.Errorf("could not reject request %s: %w", approvalUUID, err)
		}
	}

	// Update the cache
	if err := c.redisClient.Set(ctx, fmt.Sprintf(ApprovalIDPrefix, approval.ExecLogID),
		models.ApprovalRequest{UUID: approval.Uuid.String(), Status: string(approval.Status), ActionID: approval.ActionID},
		ApprovalCacheTimeout).Err(); err != nil {
		return err
	}

	return nil
}

func (c *Core) RequestApproval(ctx context.Context, execID string, action models.Action) (string, error) {
	exec, err := c.store.GetExecutionByExecID(ctx, execID)
	if err != nil {
		return "", fmt.Errorf("error getting execution for exec ID %s: %w", execID, err)
	}

	var approvalReq models.ApprovalRequest
	err = c.redisClient.Get(ctx, fmt.Sprintf(ApprovalIDPrefix, exec.ID)).Scan(&approvalReq)
	if err != nil && !errors.Is(err, redis.Nil) {
		return "", fmt.Errorf("error performing existing approval request check: %w", err)
	}

	if approvalReq.Status == string(models.ApprovalStatusPending) {
		return "", fmt.Errorf("pending approval request: %s", approvalReq.UUID)
	}

	areq, err := c.store.RequestApprovalTx(ctx, execID, action)
	if err != nil {
		return "", err
	}

	if _, err := c.redisClient.Set(ctx, fmt.Sprintf(ApprovalIDPrefix, exec.ID),
		models.ApprovalRequest{UUID: areq.Uuid.String(), Status: string(areq.Status), ActionID: action.ID}, ApprovalCacheTimeout).Result(); err != nil {
		return "", err
	}

	return areq.Uuid.String(), nil
}
