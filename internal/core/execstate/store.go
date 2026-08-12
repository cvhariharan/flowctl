package execstate

import (
	"context"
	"errors"

	"github.com/cvhariharan/flowctl/internal/repo"
)

type repository struct{ repo.Store }

func NewRecorder(ctx context.Context, store repo.Store, execID string) (*Recorder, error) {
	return newRecorder(ctx, repository{store}, execID)
}

func LoadState(ctx context.Context, store repo.Store, execID string) (ExecutionState, error) {
	events, err := repository{store}.LoadEvents(ctx, execID)
	if err != nil {
		return ExecutionState{}, err
	}
	return Fold(events), nil
}

func (s repository) AppendEvent(ctx context.Context, event Event) error {
	err := s.Store.AppendEvent(ctx, repo.Event{
		ExecID: event.ExecID, Attempt: event.Attempt, ActionID: event.ActionID,
		Type: repo.ExecutionEventType(event.Type), Error: event.Error,
		Outputs: event.Outputs, CreatedAt: event.CreatedAt,
	})
	if errors.Is(err, repo.ErrSuperseded) {
		return ErrSuperseded
	}
	return err
}

func (s repository) LoadEvents(ctx context.Context, execID string) ([]Event, error) {
	rows, err := s.Store.LoadEvents(ctx, execID)
	if err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		events = append(events, Event{
			ExecID: row.ExecID, Attempt: row.Attempt, ActionID: row.ActionID,
			Type: EventType(row.Type), Error: row.Error,
			Outputs: row.Outputs, CreatedAt: row.CreatedAt,
		})
	}
	return events, nil
}

func (s repository) BeginAttempt(ctx context.Context, execID string) (int32, error) {
	attempt, err := s.Store.BeginExecutionAttempt(ctx, execID)
	if errors.Is(err, repo.ErrStaleJob) {
		return 0, ErrStaleJob
	}
	return attempt, err
}
