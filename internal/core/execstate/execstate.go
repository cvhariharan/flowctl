package execstate

import (
	"context"
	"errors"
	"sync"
	"time"
)

type EventType string

const (
	EventQueued          EventType = "queued"
	EventStarted         EventType = "started"
	EventWaitingApproval EventType = "waiting_approval"
	EventCompleted       EventType = "completed"
	EventErrored         EventType = "errored"
	EventCancelled       EventType = "cancelled"
	EventActionStarted   EventType = "action_started"
	EventActionCompleted EventType = "action_completed"
	EventActionFailed    EventType = "action_failed"
	EventActionBlocked   EventType = "action_blocked"
	EventActionSkipped   EventType = "action_skipped"
	EventActionCancelled EventType = "action_cancelled"
)

type ActionStatus string

const (
	ActionStatusPending   ActionStatus = "pending"
	ActionStatusRunning   ActionStatus = "running"
	ActionStatusCompleted ActionStatus = "completed"
	ActionStatusFailed    ActionStatus = "failed"
	ActionStatusSkipped   ActionStatus = "skipped"
	ActionStatusBlocked   ActionStatus = "blocked"
	ActionStatusCancelled ActionStatus = "cancelled"
)

type ActionState struct {
	Status     ActionStatus `json:"status"`
	Attempt    int32        `json:"attempt,omitempty"`
	StartedAt  *time.Time   `json:"started_at,omitempty"`
	FinishedAt *time.Time   `json:"finished_at,omitempty"`
	Error      string       `json:"error,omitempty"`
}

var (
	ErrSuperseded         = errors.New("execution attempt superseded")
	ErrStaleJob           = errors.New("execution is not dispatchable")
	ErrPendingApproval    = errors.New("pending approval")
	ErrExecutionCancelled = errors.New("execution cancelled")
)

type Event struct {
	ExecID    string
	Attempt   int32
	ActionID  string
	Type      EventType
	Error     string
	Outputs   map[string]any
	CreatedAt time.Time
}

type eventStore interface {
	AppendEvent(context.Context, Event) error
	LoadEvents(context.Context, string) ([]Event, error)
	BeginAttempt(context.Context, string) (int32, error)
}

type ExecutionState struct {
	Status          string
	Error           string
	StartedAt       *time.Time
	CompletedAt     *time.Time
	Outputs         map[string]any
	Actions         map[string]ActionState
	CurrentActionID string
}

func Fold(events []Event) ExecutionState {
	state := ExecutionState{
		Status:  string(ActionStatusPending),
		Outputs: make(map[string]any),
		Actions: make(map[string]ActionState),
	}
	activeSeq := make(map[string]int)

	for seq, event := range events {
		for key, value := range event.Outputs {
			state.Outputs[key] = value
		}

		if event.ActionID != "" {
			action := state.Actions[event.ActionID]
			action.Error = event.Error
			switch event.Type {
			case EventActionStarted:
				action.Status = ActionStatusRunning
				action.Attempt++
				t := event.CreatedAt
				action.StartedAt = &t
			case EventActionCompleted:
				action.Status = ActionStatusCompleted
				t := event.CreatedAt
				action.FinishedAt = &t
			case EventActionFailed:
				action.Status = ActionStatusFailed
				t := event.CreatedAt
				action.FinishedAt = &t
			case EventActionBlocked:
				action.Status = ActionStatusBlocked
			case EventActionSkipped:
				action.Status = ActionStatusSkipped
				t := event.CreatedAt
				action.FinishedAt = &t
			case EventActionCancelled:
				action.Status = ActionStatusCancelled
				t := event.CreatedAt
				action.FinishedAt = &t
			}
			state.Actions[event.ActionID] = action
			activeSeq[event.ActionID] = seq
			continue
		}

		state.Error = event.Error
		switch event.Type {
		case EventQueued:
			state.Status = string(ActionStatusPending)
			state.CompletedAt = nil
		case EventStarted:
			state.Status = string(ActionStatusRunning)
			state.CompletedAt = nil
			if state.StartedAt == nil {
				t := event.CreatedAt
				state.StartedAt = &t
			}
		case EventWaitingApproval:
			state.Status = "pending_approval"
			state.CompletedAt = nil
		case EventCompleted:
			state.Status = string(ActionStatusCompleted)
			t := event.CreatedAt
			state.CompletedAt = &t
		case EventErrored:
			state.Status = "errored"
			t := event.CreatedAt
			state.CompletedAt = &t
		case EventCancelled:
			state.Status = string(ActionStatusCancelled)
			t := event.CreatedAt
			state.CompletedAt = &t
		}
	}

	last := -1
	for id, seq := range activeSeq {
		switch state.Actions[id].Status {
		case ActionStatusRunning, ActionStatusBlocked, ActionStatusFailed:
			if seq > last {
				last = seq
				state.CurrentActionID = id
			}
		}
	}
	return state
}

type Recorder struct {
	mu      sync.RWMutex
	store   eventStore
	execID  string
	attempt int32
	events  []Event
	state   ExecutionState
}

func newRecorder(ctx context.Context, store eventStore, execID string) (*Recorder, error) {
	events, err := store.LoadEvents(ctx, execID)
	if err != nil {
		return nil, err
	}
	attempt, err := store.BeginAttempt(ctx, execID)
	if err != nil {
		return nil, err
	}
	return &Recorder{store: store, execID: execID, attempt: attempt, events: events, state: Fold(events)}, nil
}

func (r *Recorder) append(ctx context.Context, event Event) error {
	event.ExecID = r.execID
	event.Attempt = r.attempt
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	if err := r.store.AppendEvent(ctx, event); err != nil {
		return err
	}
	r.mu.Lock()
	r.events = append(r.events, event)
	r.state = Fold(r.events)
	r.mu.Unlock()
	return nil
}

func (r *Recorder) Start(ctx context.Context) error {
	return r.append(ctx, Event{Type: EventStarted})
}

func (r *Recorder) StartAction(ctx context.Context, id string) error {
	return r.append(ctx, Event{ActionID: id, Type: EventActionStarted})
}

func (r *Recorder) FinishAction(ctx context.Context, id string, outputs map[string]any, err error) error {
	event := Event{ActionID: id}
	switch {
	case err == nil:
		event.Type = EventActionCompleted
		event.Outputs = outputs
	case errors.Is(err, ErrPendingApproval):
		event.Type = EventActionBlocked
	case errors.Is(err, ErrExecutionCancelled), errors.Is(err, context.Canceled):
		event.Type = EventActionCancelled
	default:
		event.Type = EventActionFailed
		event.Error = err.Error()
	}
	return r.append(ctx, event)
}

func (r *Recorder) BlockAction(ctx context.Context, id string, err error) error {
	type_ := EventActionBlocked
	if err != nil && !errors.Is(err, ErrPendingApproval) {
		type_ = EventActionFailed
	}
	event := Event{ActionID: id, Type: type_}
	if err != nil && !errors.Is(err, ErrPendingApproval) {
		event.Error = err.Error()
	}
	return r.append(ctx, event)
}

func (r *Recorder) SkipPending(ctx context.Context, ids []string) error {
	for _, id := range ids {
		if err := r.append(ctx, Event{ActionID: id, Type: EventActionSkipped}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Recorder) Finish(ctx context.Context, err error) error {
	event := Event{}
	switch {
	case err == nil:
		event.Type = EventCompleted
	case errors.Is(err, ErrPendingApproval):
		event.Type = EventWaitingApproval
	case errors.Is(err, ErrExecutionCancelled), errors.Is(err, context.Canceled):
		event.Type = EventCancelled
	default:
		event.Type = EventErrored
		event.Error = err.Error()
	}
	return r.append(ctx, event)
}

func (r *Recorder) State() ExecutionState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneExecutionState(r.state)
}

func (r *Recorder) Runnable(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state.Actions[id].Status != ActionStatusCompleted
}

func (r *Recorder) Attempt(id string) int32 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state.Actions[id].Attempt
}

func cloneExecutionState(state ExecutionState) ExecutionState {
	state.Outputs = cloneOutputs(state.Outputs)
	actions := state.Actions
	state.Actions = map[string]ActionState{}
	for id, action := range actions {
		state.Actions[id] = action
	}
	return state
}

func cloneOutputs(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for key, value := range src {
		if inner, ok := value.(map[string]any); ok {
			dst[key] = cloneOutputs(inner)
			continue
		}
		dst[key] = value
	}
	return dst
}
