package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/sqlc-dev/pqtype"
)

var (
	ErrSuperseded = errors.New("execution attempt superseded")
	ErrStaleJob   = errors.New("execution is not dispatchable")
)

type Event struct {
	ExecID    string
	Attempt   int32
	ActionID  string
	Type      ExecutionEventType
	Error     string
	Outputs   map[string]any
	CreatedAt time.Time
}

type ExecutionJob struct {
	PayloadType string
	Payload     json.RawMessage
	CreatedAt   time.Time
	// ScheduledAt is the zero time for jobs that run immediately. The job queue reader scans this
	// column into a time.Time, so it must never be NULL.
	ScheduledAt time.Time
	MaxRetries  int32
	Attempt     int32
}

type RequestApprovalParam struct {
	ID string
}

type CreateUserTxParams struct {
	Name      string
	Username  string
	LoginType UserLoginType
	Role      UserRoleType
	Groups    []string
}

type UpdateUserTxParams struct {
	UserUUID uuid.UUID
	Name     string
	Username string
	Groups   []string
}

type ApprovalDecisionTxParams struct {
	ApprovalUUID    uuid.UUID
	NamespaceUUID   uuid.UUID
	DecidedByUserID int32
	Status          ApprovalStatus
}

type ApprovalDecisionResult struct {
	Uuid        uuid.UUID
	Status      ApprovalStatus
	ActionID    string
	RequestedBy string
	ExecID      string
}

type CreateFlowTxParams struct {
	Slug        string
	Name        string
	Description string
	Checksum    string
	FilePath    string
	Namespace   string
	PrefixID    sql.NullInt32
	Schedules   []struct {
		Name     string
		Cron     string
		Timezone string
	}
}

type UpdateFlowTxParams struct {
	Slug            string
	Name            string
	Description     string
	Checksum        string
	FilePath        string
	Namespace       string
	PrefixID        sql.NullInt32
	UserSchedulable bool
	Schedules       []struct {
		Name     string
		Cron     string
		Timezone string
	}
}

type Store interface {
	Querier
	AppendEvent(ctx context.Context, event Event) error
	LoadEvents(ctx context.Context, execID string) ([]Event, error)
	BeginExecutionAttempt(ctx context.Context, execID string) (int32, error)
	AddExecutionTx(ctx context.Context, params AddExecutionParams, outputs map[string]any) (Execution, error)
	QueueExecutionTx(ctx context.Context, params AddExecutionParams, outputs map[string]any, job ExecutionJob) (Execution, error)
	CancelExecutionTx(ctx context.Context, params CancelExecutionParams, note string) (Execution, error)
	RequeueExecutionTx(ctx context.Context, params RequeueExecutionParams) (int32, error)
	RequeueExecutionAndJobTx(ctx context.Context, params RequeueExecutionParams, job ExecutionJob) (int32, error)
	ResetActionsAndRequeueTx(ctx context.Context, params RequeueExecutionParams, actionIDs []string, job ExecutionJob) (int32, error)
	DeleteExpiredExecutionsTx(ctx context.Context, cutoff time.Time, batchSize int) (int, error)
	RequestApprovalTx(ctx context.Context, execID string, namespaceUUID uuid.UUID, action RequestApprovalParam) (AddApprovalRequestRow, error)
	CreateUserTx(ctx context.Context, params CreateUserTxParams) (UserView, error)
	UpdateUserTx(ctx context.Context, params UpdateUserTxParams) (UserView, error)
	ProcessApprovalDecisionTx(ctx context.Context, params ApprovalDecisionTxParams) (ApprovalDecisionResult, error)
	CreateFlowTx(ctx context.Context, params CreateFlowTxParams) (Flow, error)
	UpdateFlowTx(ctx context.Context, params UpdateFlowTxParams) (Flow, error)
}

func (p *PostgresStore) AddExecutionTx(ctx context.Context, params AddExecutionParams, outputs map[string]any) (Execution, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Execution{}, err
	}
	defer tx.Rollback()
	q := &Queries{db: tx}
	exec, err := q.AddExecution(ctx, params)
	if err != nil {
		return Execution{}, err
	}
	if err := appendEvent(ctx, q, Event{
		ExecID: exec.ExecID, Attempt: exec.Attempt, Type: ExecutionEventTypeQueued, Outputs: outputs,
	}); err != nil {
		return Execution{}, err
	}
	return exec, tx.Commit()
}

func (p *PostgresStore) QueueExecutionTx(ctx context.Context, params AddExecutionParams, outputs map[string]any, job ExecutionJob) (Execution, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Execution{}, err
	}
	defer tx.Rollback()
	q := &Queries{db: tx}
	exec, err := q.AddExecution(ctx, params)
	if err != nil {
		return Execution{}, err
	}
	if err := appendEvent(ctx, q, Event{
		ExecID: exec.ExecID, Attempt: exec.Attempt, Type: ExecutionEventTypeQueued, Outputs: outputs,
	}); err != nil {
		return Execution{}, err
	}
	if err := insertExecutionJob(ctx, tx, exec.ExecID, job); err != nil {
		return Execution{}, err
	}
	return exec, tx.Commit()
}

func insertExecutionJob(ctx context.Context, tx *sql.Tx, execID string, job ExecutionJob) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO job_queue (exec_id, payload_type, payload, created_at, scheduled_at, max_retries, attempt)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, execID, job.PayloadType, job.Payload, job.CreatedAt, job.ScheduledAt, job.MaxRetries, job.Attempt)
	return err
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

func (p *PostgresStore) AppendEvent(ctx context.Context, event Event) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := appendEvent(ctx, &Queries{db: tx}, event); err != nil {
		return err
	}
	return tx.Commit()
}

func appendEvent(ctx context.Context, q *Queries, event Event) error {
	var outputs pqtype.NullRawMessage
	if event.Outputs != nil {
		raw, err := json.Marshal(event.Outputs)
		if err != nil {
			return err
		}
		outputs = pqtype.NullRawMessage{RawMessage: raw, Valid: true}
	}
	createdAt := event.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	params := InsertExecutionEventParams{
		ExecID:    event.ExecID,
		Attempt:   event.Attempt,
		ActionID:  sql.NullString{String: event.ActionID, Valid: event.ActionID != ""},
		EventType: event.Type,
		Error:     sql.NullString{String: event.Error, Valid: event.Error != ""},
		Outputs:   outputs,
		CreatedAt: sql.NullTime{Time: createdAt, Valid: true},
	}
	n, err := q.InsertExecutionEvent(ctx, params)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrSuperseded
	}
	result, err := q.ProjectExecutionEvent(ctx, ProjectExecutionEventParams{
		EventType: params.EventType,
		Error:     params.Error,
		Outputs:   params.Outputs,
		CreatedAt: params.CreatedAt,
		ExecID:    params.ExecID,
		Attempt:   params.Attempt,
	})
	if err != nil {
		return err
	}
	n, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrSuperseded
	}
	return nil
}

func (p *PostgresStore) LoadEvents(ctx context.Context, execID string) ([]Event, error) {
	rows, err := p.LoadExecutionEvents(ctx, execID)
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		event := Event{
			ExecID: row.ExecID, Attempt: row.Attempt, ActionID: row.ActionID.String,
			Type: row.Type, Error: row.Error.String, CreatedAt: row.CreatedAt,
		}
		if row.Outputs.Valid {
			if err := json.Unmarshal(row.Outputs.RawMessage, &event.Outputs); err != nil {
				return nil, err
			}
		}
		events = append(events, event)
	}
	return events, nil
}

func (p *PostgresStore) BeginExecutionAttempt(ctx context.Context, execID string) (int32, error) {
	attempt, err := p.BeginAttempt(ctx, execID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrStaleJob
	}
	return attempt, err
}

func (p *PostgresStore) CancelExecutionTx(ctx context.Context, params CancelExecutionParams, note string) (Execution, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return Execution{}, err
	}
	defer tx.Rollback()
	q := &Queries{db: tx}
	exec, err := q.CancelExecution(ctx, params)
	if err != nil {
		return Execution{}, err
	}
	if err := appendEvent(ctx, q, Event{ExecID: exec.ExecID, Attempt: exec.Attempt, Type: ExecutionEventTypeCancelled, Error: note}); err != nil {
		return Execution{}, err
	}
	return exec, tx.Commit()
}

func (p *PostgresStore) RequeueExecutionTx(ctx context.Context, params RequeueExecutionParams) (int32, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	q := &Queries{db: tx}
	attempt, err := q.RequeueExecution(ctx, params)
	if err != nil {
		return 0, err
	}
	if err := appendEvent(ctx, q, Event{ExecID: params.ExecID, Attempt: attempt, Type: ExecutionEventTypeQueued}); err != nil {
		return 0, err
	}
	return attempt, tx.Commit()
}

func (p *PostgresStore) RequeueExecutionAndJobTx(ctx context.Context, params RequeueExecutionParams, job ExecutionJob) (int32, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	q := &Queries{db: tx}
	attempt, err := q.RequeueExecution(ctx, params)
	if err != nil {
		return 0, err
	}
	if err := appendEvent(ctx, q, Event{ExecID: params.ExecID, Attempt: attempt, Type: ExecutionEventTypeQueued}); err != nil {
		return 0, err
	}
	if err := insertExecutionJob(ctx, tx, params.ExecID, job); err != nil {
		return 0, err
	}
	return attempt, tx.Commit()
}

// ResetActionsAndRequeueTx resets the given actions and requeues the execution in one transaction.
func (p *PostgresStore) ResetActionsAndRequeueTx(ctx context.Context, params RequeueExecutionParams, actionIDs []string, job ExecutionJob) (int32, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	q := &Queries{db: tx}
	attempt, err := q.RequeueExecutionForReset(ctx, RequeueExecutionForResetParams{
		ExecID: params.ExecID,
		Uuid:   params.Uuid,
	})
	if err != nil {
		return 0, err
	}
	for _, actionID := range actionIDs {
		if err := appendEvent(ctx, q, Event{
			ExecID: params.ExecID, Attempt: attempt, ActionID: actionID, Type: ExecutionEventTypeActionReset,
		}); err != nil {
			return 0, err
		}
	}
	if err := appendEvent(ctx, q, Event{ExecID: params.ExecID, Attempt: attempt, Type: ExecutionEventTypeQueued}); err != nil {
		return 0, err
	}
	if err := q.DeleteApprovalsForExecActions(ctx, DeleteApprovalsForExecActionsParams{
		ExecID: params.ExecID, ActionIds: actionIDs,
	}); err != nil {
		return 0, err
	}
	if err := insertExecutionJob(ctx, tx, params.ExecID, job); err != nil {
		return 0, err
	}
	return attempt, tx.Commit()
}

func (p *PostgresStore) DeleteExpiredExecutionsTx(ctx context.Context, cutoff time.Time, batchSize int) (int, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	q := &Queries{db: tx}
	ids, err := q.DeleteExpiredExecutions(ctx, DeleteExpiredExecutionsParams{
		Cutoff: sql.NullTime{Time: cutoff, Valid: true}, BatchSize: int32(batchSize),
	})
	if err != nil {
		return 0, err
	}
	return len(ids), tx.Commit()
}

func (p *PostgresStore) RequestApprovalTx(ctx context.Context, execID string, namespaceUUID uuid.UUID, action RequestApprovalParam) (AddApprovalRequestRow, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return AddApprovalRequestRow{}, fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	q := Queries{db: tx}

	a, err := q.AddApprovalRequest(ctx, AddApprovalRequestParams{
		ExecID:   execID,
		ActionID: action.ID,
		Uuid:     namespaceUUID,
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
			ExecID:      a.ExecID,
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
			ExecID:      a.ExecID,
		}
	} else {
		return ApprovalDecisionResult{}, fmt.Errorf("invalid approval status: %s", params.Status)
	}

	if err := tx.Commit(); err != nil {
		return ApprovalDecisionResult{}, fmt.Errorf("could not commit transaction: %w", err)
	}

	return approval, nil
}

func (p *PostgresStore) CreateFlowTx(ctx context.Context, params CreateFlowTxParams) (Flow, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return Flow{}, fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	q := Queries{db: tx}

	// Create the flow
	flow, err := q.CreateFlow(ctx, CreateFlowParams{
		Slug:        params.Slug,
		Name:        params.Name,
		Description: sql.NullString{String: params.Description, Valid: true},
		Checksum:    params.Checksum,
		FilePath:    params.FilePath,
		Name_2:      params.Namespace,
		PrefixID:    params.PrefixID,
	})
	if err != nil {
		return Flow{}, fmt.Errorf("could not create flow: %w", err)
	}

	// Create cron schedules
	for _, sched := range params.Schedules {
		_, err = q.CreateCronSchedule(ctx, CreateCronScheduleParams{
			FlowID:   flow.ID,
			Name:     sched.Name,
			Cron:     sched.Cron,
			Timezone: sched.Timezone,
		})
		if err != nil {
			return Flow{}, fmt.Errorf("could not create schedule: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Flow{}, fmt.Errorf("could not commit transaction: %w", err)
	}

	return flow, nil
}

func (p *PostgresStore) UpdateFlowTx(ctx context.Context, params UpdateFlowTxParams) (Flow, error) {
	tx, err := p.db.Begin()
	if err != nil {
		return Flow{}, fmt.Errorf("could not start transaction: %w", err)
	}
	defer tx.Rollback()

	q := Queries{db: tx}

	// Update the flow
	flow, err := q.UpdateFlow(ctx, UpdateFlowParams{
		Name:        params.Name,
		Description: sql.NullString{String: params.Description, Valid: true},
		Checksum:    params.Checksum,
		FilePath:    params.FilePath,
		PrefixID:    params.PrefixID,
		Slug:        params.Slug,
		Name_2:      params.Namespace,
	})
	if err != nil {
		return Flow{}, fmt.Errorf("could not update flow: %w", err)
	}

	// Disable user-created schedules if flow is not schedulable or not user-schedulable
	if !params.UserSchedulable {
		err = q.DisableUserSchedulesForFlow(ctx, flow.ID)
		if err != nil {
			return Flow{}, fmt.Errorf("could not disable user schedules: %w", err)
		}
	}

	// Delete existing system schedules only
	err = q.DeleteSystemCronsByFlowID(ctx, flow.ID)
	if err != nil {
		return Flow{}, fmt.Errorf("could not delete old system schedules: %w", err)
	}

	// Create new system schedules from flow definition
	for _, sched := range params.Schedules {
		_, err = q.CreateCronSchedule(ctx, CreateCronScheduleParams{
			FlowID:   flow.ID,
			Name:     sched.Name,
			Cron:     sched.Cron,
			Timezone: sched.Timezone,
		})
		if err != nil {
			return Flow{}, fmt.Errorf("could not create schedule: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Flow{}, fmt.Errorf("could not commit transaction: %w", err)
	}

	return flow, nil
}
