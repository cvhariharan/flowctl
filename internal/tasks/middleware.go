package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/hibiken/asynq"
)

type StatusTrackerDB struct {
	store repo.Store
}

func NewStatusTracker(s repo.Store) *StatusTrackerDB {
	return &StatusTrackerDB{store: s}
}

func (s *StatusTrackerDB) SetStatus(ctx context.Context, execID string, status repo.ExecutionStatus, err error) error {
	var errMsg sql.NullString
	if err != nil {
		errMsg = sql.NullString{String: err.Error(), Valid: true}
	}
	_, err = s.store.UpdateExecutionStatus(ctx, repo.UpdateExecutionStatusParams{
		Status:    status,
		Error:     errMsg,
		ExecID:    execID,
		UpdatedAt: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("could not update error execution status: %w", err)
	}

	return nil
}

func (s *StatusTrackerDB) TrackerMiddleware(next func(context.Context, *asynq.Task) error) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload FlowExecutionPayload

		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("payload could not be deserialized: %w", err)
		}

		if err := s.SetStatus(ctx, payload.ExecID, repo.ExecutionStatusRunning, nil); err != nil {
			return err
		}

		if err := next(ctx, t); err != nil {
			if errors.Is(err, ErrPendingApproval) {
				return s.SetStatus(ctx, payload.ExecID, repo.ExecutionStatusPendingApproval, nil)
			}
			return s.SetStatus(ctx, payload.ExecID, repo.ExecutionStatusErrored, err)
		}

		return s.SetStatus(ctx, payload.ExecID, repo.ExecutionStatusCompleted, nil)
	}
}
