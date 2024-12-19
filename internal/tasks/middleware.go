package tasks

import (
	"context"
	"database/sql"
	"encoding/json"
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

func (s *StatusTrackerDB) SetStatus(ctx context.Context, execID string, err error) error {
	if err != nil {
		_, err = s.store.UpdateExecutionStatus(ctx, repo.UpdateExecutionStatusParams{
			Status:    repo.ExecutionStatusErrored,
			Error:     sql.NullString{String: err.Error(), Valid: true},
			ExecID:    execID,
			UpdatedAt: time.Now(),
		})
		if err != nil {
			return fmt.Errorf("could not update error execution status: %w", err)
		}
	} else {
		_, err = s.store.UpdateExecutionStatus(ctx, repo.UpdateExecutionStatusParams{
			Status:    repo.ExecutionStatusCompleted,
			ExecID:    execID,
			UpdatedAt: time.Now(),
		})
		if err != nil {
			return fmt.Errorf("could not update completed execution status: %w", err)
		}
	}

	return nil
}

func (s *StatusTrackerDB) TrackerMiddleware(next func(context.Context, *asynq.Task) error) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload FlowExecutionPayload

		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("payload could not be deserialized: %w", err)
		}

		return s.SetStatus(ctx, payload.LogID, next(ctx, t))
	}
}
