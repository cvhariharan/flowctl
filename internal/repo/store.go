package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type RequestApprovalParam struct {
	ID string
}

type CreateUserTxParams struct {
	Name                   string
	Username               string
	LoginType              UserLoginType
	Role                   UserRoleType
	Groups                 []string
	AssignDefaultNamespace bool
	DefaultNamespaceUUID   uuid.UUID
}

type UpdateUserTxParams struct {
	UserUUID uuid.UUID
	Name     string
	Username string
	Groups   []string
}

type ApprovalDecisionTxParams struct {
	ApprovalUUID     uuid.UUID
	NamespaceUUID    uuid.UUID
	DecidedByUserID  int32
	Status           ApprovalStatus
	CancellationNote string
}

type ApprovalDecisionResult struct {
	Uuid        uuid.UUID
	Status      ApprovalStatus
	ActionID    string
	RequestedBy string
	ExecLogID   int32
	ExecID      string
}

type Store interface {
	Querier
	RequestApprovalTx(ctx context.Context, execID string, namespaceUUID uuid.UUID, action RequestApprovalParam) (AddApprovalRequestRow, error)
	CreateUserTx(ctx context.Context, params CreateUserTxParams) (UserView, error)
	UpdateUserTx(ctx context.Context, params UpdateUserTxParams) (UserView, error)
	ProcessApprovalDecisionTx(ctx context.Context, params ApprovalDecisionTxParams) (ApprovalDecisionResult, error)
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

func (p *PostgresStore) RequestApprovalTx(ctx context.Context, execID string, namespaceUUID uuid.UUID, action RequestApprovalParam) (AddApprovalRequestRow, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return AddApprovalRequestRow{}, fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	q := Queries{db: tx}

	e, err := q.GetExecutionByExecID(ctx, GetExecutionByExecIDParams{
		ExecID: execID,
		Uuid:   namespaceUUID,
	})
	if err != nil {
		return AddApprovalRequestRow{}, fmt.Errorf("could not get exec details for %s: %w", execID, err)
	}

	a, err := q.AddApprovalRequest(ctx, AddApprovalRequestParams{
		ExecLogID: e.ID,
		ActionID:  action.ID,
		Uuid:      namespaceUUID,
	})
	if err != nil {
		return AddApprovalRequestRow{}, fmt.Errorf("could not create approval request: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return AddApprovalRequestRow{}, fmt.Errorf("coudl not commit transaction: %w", err)
	}

	return a, nil
}

func (p *PostgresStore) CreateUserTx(ctx context.Context, params CreateUserTxParams) (UserView, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return UserView{}, fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	q := Queries{db: tx}

	user, err := q.CreateUser(ctx, CreateUserParams{
		Name:      params.Name,
		Username:  params.Username,
		LoginType: params.LoginType,
		Role:      params.Role,
	})
	if err != nil {
		return UserView{}, fmt.Errorf("could not create user %s: %w", params.Username, err)
	}

	if len(params.Groups) > 0 {
		for _, group := range params.Groups {
			gid, err := uuid.Parse(group)
			if err != nil {
				return UserView{}, fmt.Errorf("group ID should be a UUID: %w", err)
			}

			if err := q.AddGroupToUserByUUID(ctx, AddGroupToUserByUUIDParams{
				UserUuid:  user.Uuid,
				GroupUuid: gid,
			}); err != nil {
				return UserView{}, fmt.Errorf("could not add group %s to user %s: %w", group, params.Username, err)
			}
		}
	}

	if params.AssignDefaultNamespace {
		_, err = q.AssignUserNamespaceRole(ctx, AssignUserNamespaceRoleParams{
			Uuid:   user.Uuid,
			Uuid_2: params.DefaultNamespaceUUID,
			Role:   "user",
		})
		if err != nil {
			return UserView{}, fmt.Errorf("could not assign user %s to default namespace: %w", params.Username, err)
		}
	}

	userWithGroups, err := q.GetUserByUUIDWithGroups(ctx, user.Uuid)
	if err != nil {
		return UserView{}, fmt.Errorf("could not get created user with groups %s: %w", params.Username, err)
	}

	if err := tx.Commit(); err != nil {
		return UserView{}, fmt.Errorf("could not commit transaction: %w", err)
	}

	return userWithGroups, nil
}

func (p *PostgresStore) UpdateUserTx(ctx context.Context, params UpdateUserTxParams) (UserView, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return UserView{}, fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	q := Queries{db: tx}

	_, err = q.UpdateUserByUUID(ctx, UpdateUserByUUIDParams{
		Uuid:     params.UserUUID,
		Name:     params.Name,
		Username: params.Username,
	})
	if err != nil {
		return UserView{}, fmt.Errorf("could not update user info: %w", err)
	}

	if err := q.RemoveAllGroupsForUserByUUID(ctx, params.UserUUID); err != nil {
		return UserView{}, fmt.Errorf("could not remove existing groups: %w", err)
	}

	for _, group := range params.Groups {
		gid, err := uuid.Parse(group)
		if err != nil {
			return UserView{}, fmt.Errorf("group ID should be a UUID: %w", err)
		}

		if err := q.AddGroupToUserByUUID(ctx, AddGroupToUserByUUIDParams{
			UserUuid:  params.UserUUID,
			GroupUuid: gid,
		}); err != nil {
			return UserView{}, fmt.Errorf("could not add group %s to user: %w", group, err)
		}
	}

	userWithGroups, err := q.GetUserByUUIDWithGroups(ctx, params.UserUUID)
	if err != nil {
		return UserView{}, fmt.Errorf("could not get updated user with groups: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return UserView{}, fmt.Errorf("could not commit transaction: %w", err)
	}

	return userWithGroups, nil
}

func (p *PostgresStore) ProcessApprovalDecisionTx(ctx context.Context, params ApprovalDecisionTxParams) (ApprovalDecisionResult, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return ApprovalDecisionResult{}, fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	q := Queries{db: tx}

	var approval ApprovalDecisionResult

	// Process approval or rejection
	if params.Status == ApprovalStatusApproved {
		a, err := q.ApproveRequestByUUID(ctx, ApproveRequestByUUIDParams{
			Uuid:      params.ApprovalUUID,
			DecidedBy: sql.NullInt32{Int32: params.DecidedByUserID, Valid: true},
			Uuid_2:    params.NamespaceUUID,
		})
		if err != nil {
			return ApprovalDecisionResult{}, fmt.Errorf("could not approve request: %w", err)
		}

		approval = ApprovalDecisionResult{
			Uuid:        a.Uuid,
			Status:      a.Status,
			ActionID:    a.ActionID,
			RequestedBy: a.RequestedBy,
			ExecLogID:   a.ExecLogID,
		}
	} else if params.Status == ApprovalStatusRejected {
		a, err := q.RejectRequestByUUID(ctx, RejectRequestByUUIDParams{
			Uuid:      params.ApprovalUUID,
			DecidedBy: sql.NullInt32{Int32: params.DecidedByUserID, Valid: true},
			Uuid_2:    params.NamespaceUUID,
		})
		if err != nil {
			return ApprovalDecisionResult{}, fmt.Errorf("could not reject request: %w", err)
		}

		approval = ApprovalDecisionResult{
			Uuid:        a.Uuid,
			Status:      a.Status,
			ActionID:    a.ActionID,
			RequestedBy: a.RequestedBy,
			ExecLogID:   a.ExecLogID,
		}

		// If rejected, update execution status to cancelled
		if params.CancellationNote != "" {
			exec, err := q.GetExecutionByID(ctx, GetExecutionByIDParams{
				ID:   a.ExecLogID,
				Uuid: params.NamespaceUUID,
			})
			if err != nil {
				return ApprovalDecisionResult{}, fmt.Errorf("could not get execution: %w", err)
			}

			_, err = q.UpdateExecutionStatus(ctx, UpdateExecutionStatusParams{
				Status: ExecutionStatusCancelled,
				Error:  sql.NullString{String: params.CancellationNote, Valid: true},
				ExecID: exec.ExecID,
				Uuid:   params.NamespaceUUID,
			})
			if err != nil {
				return ApprovalDecisionResult{}, fmt.Errorf("could not update execution status: %w", err)
			}
		}
	} else {
		return ApprovalDecisionResult{}, fmt.Errorf("invalid approval status: %s", params.Status)
	}

	// Get execution info to include in result
	exec, err := q.GetExecutionByID(ctx, GetExecutionByIDParams{
		ID:   approval.ExecLogID,
		Uuid: params.NamespaceUUID,
	})
	if err != nil {
		return ApprovalDecisionResult{}, fmt.Errorf("could not get execution info: %w", err)
	}
	approval.ExecID = exec.ExecID

	if err := tx.Commit(); err != nil {
		return ApprovalDecisionResult{}, fmt.Errorf("could not commit transaction: %w", err)
	}

	return approval, nil
}
