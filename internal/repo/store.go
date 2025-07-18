package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type RequestApprovalParam struct {
	ID string
	Approvers []string
}

type Store interface {
	Querier
	OverwriteGroupsForUserTx(ctx context.Context, userUUID uuid.UUID, groups []string) error
	RequestApprovalTx(ctx context.Context, execID string, namespaceUUID uuid.UUID, action RequestApprovalParam) (AddApprovalRequestRow, error)
}

type PostgresStore struct {
	*Queries
	db *sqlx.DB
}

func NewPostgresStore(db *sqlx.DB) Store {
	return &PostgresStore{
		db:      db,
		Queries: New(db),
	}
}

func (p *PostgresStore) OverwriteGroupsForUserTx(ctx context.Context, userUUID uuid.UUID, groups []string) error {
	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	q := Queries{db: tx}

	if err := q.RemoveAllGroupsForUserByUUID(ctx, userUUID); err != nil {
		return err
	}

	for _, group := range groups {
		gid, err := uuid.Parse(group)
		if err != nil {
			return fmt.Errorf("group ID should be a UUID: %w", err)
		}

		if err := q.AddGroupToUserByUUID(ctx, AddGroupToUserByUUIDParams{
			UserUuid:  userUUID,
			GroupUuid: gid,
		}); err != nil {
			return fmt.Errorf("could not add group %s to user %s: %w", group, userUUID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("could not commit transaction: %w", err)
	}

	return nil
}

func (p *PostgresStore) RequestApprovalTx(ctx context.Context, execID string, namespaceUUID uuid.UUID, action RequestApprovalParam) (AddApprovalRequestRow, error) {
	if len(action.Approvers) == 0 {
		return AddApprovalRequestRow{}, fmt.Errorf("no approvers specified")
	}

	tx, err := p.db.Begin()
	if err != nil {
		return AddApprovalRequestRow{}, fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	e, err := p.GetExecutionByExecID(ctx, GetExecutionByExecIDParams{
		ExecID: execID,
		Uuid:   namespaceUUID,
	})
	if err != nil {
		return AddApprovalRequestRow{}, fmt.Errorf("could not get exec details for %s: %w", execID, err)
	}

	approvers, err := json.Marshal(action.Approvers)
	if err != nil {
		return AddApprovalRequestRow{}, fmt.Errorf("could not marshal approvers list: %w", err)
	}

	a, err := p.AddApprovalRequest(ctx, AddApprovalRequestParams{
		ExecLogID: e.ID,
		Approvers: approvers,
		ActionID:  action.ID,
	})
	if err != nil {
		return AddApprovalRequestRow{}, fmt.Errorf("could not create approval request: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return AddApprovalRequestRow{}, fmt.Errorf("coudl not commit transaction: %w", err)
	}

	return a, nil
}
