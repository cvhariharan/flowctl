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
	"github.com/google/uuid"
)

var (
	ErrNoPendingApproval = errors.New("no pending approval")
	ErrNil               = errors.New("not found")
)

// ApproveOrRejectAction handles approval or rejection of an action request by a user.
// It takes the approval UUID, the ID of the user making the decision, and the approval status.
// The function updates the database with the decision.
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

	// Check if there's already a pending approval for this execution
	existingReq, err := c.store.GetApprovalRequestForExec(ctx, repo.GetApprovalRequestForExecParams{
		ExecID: execID,
		Uuid:   namespaceUUID,
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("error checking existing approval request: %w", err)
	}

	if err == nil && existingReq.Status == repo.ApprovalStatusPending {
		return "", fmt.Errorf("pending approval request: %s", existingReq.Uuid.String())
	}

	areq, err := c.store.RequestApprovalTx(ctx, execID, namespaceUUID, repo.RequestApprovalParam{ID: action.ID})
	if err != nil {
		return "", err
	}

	return areq.Uuid.String(), nil
}

func (c *Core) GetApprovalsRequestsForExec(ctx context.Context, execID string, namespaceID string) (models.ApprovalRequest, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return models.ApprovalRequest{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}

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

	existingReq := models.ApprovalRequest{
		UUID:        areq.Uuid.String(),
		Status:      models.ApprovalType(areq.Status),
		ActionID:    areq.ActionID,
		ExecID:      execID,
		RequestedBy: areq.RequestedBy,
	}

	return existingReq, nil
}

func (c *Core) GetApprovalRequest(ctx context.Context, approvalUUID string, namespaceID string) (models.ApprovalRequest, error) {
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

	approval := models.ApprovalRequest{
		UUID:        areq.Uuid.String(),
		Status:      models.ApprovalType(areq.Status),
		ActionID:    areq.ActionID,
		ExecID:      exec.ExecID,
		RequestedBy: areq.RequestedBy,
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
		DecidedBy: approval.DecidedByName.String,
		Inputs:    approval.ExecInputs,
		FlowName:  approval.FlowName,
		FlowID:    approval.FlowSlug,
		CreatedAt: approval.CreatedAt.Format(time.RFC3339),
		UpdatedAt: approval.UpdatedAt.Format(time.RFC3339),
	}

	return details, nil
}

func (c *Core) GetApprovalsPaginated(ctx context.Context, namespaceID, status, filter string, page, countPerPage int) ([]models.ApprovalPaginationDetails, int64, int64, error) {
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

	var details []models.ApprovalPaginationDetails
	var pageCount, totalCount int64
	for _, approval := range approvals {
		details = append(details, models.ApprovalPaginationDetails{
			ApprovalRequest: models.ApprovalRequest{
				UUID: approval.Uuid.String(),
				ActionID: approval.ActionID,
				ExecID: approval.ExecID,
				Status: models.ApprovalType(approval.Status),
				RequestedBy: approval.RequestedBy,
			},
			FlowName: approval.FlowName,
			CreatedAt: approval.CreatedAt.Format(TimeFormat),
			UpdatedAt: approval.UpdatedAt.Format(TimeFormat),
		})
		pageCount = approval.PageCount
		totalCount = approval.TotalCount
	}

	return details, pageCount, totalCount, nil
}
