package execstate

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestFold(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	events := []Event{
		{Type: EventQueued, CreatedAt: t0},
		{Type: EventStarted, CreatedAt: t0.Add(time.Second)},
		{ActionID: "build", Type: EventActionStarted, CreatedAt: t0.Add(2 * time.Second)},
		{ActionID: "build", Type: EventActionCompleted, Outputs: map[string]any{"IMAGE": "one"}, CreatedAt: t0.Add(3 * time.Second)},
		{ActionID: "deploy", Type: EventActionStarted, CreatedAt: t0.Add(4 * time.Second)},
		{ActionID: "deploy", Type: EventActionFailed, Error: "failed", CreatedAt: t0.Add(5 * time.Second)},
		{Type: EventErrored, Error: "failed", CreatedAt: t0.Add(6 * time.Second)},
		{Type: EventQueued, CreatedAt: t0.Add(7 * time.Second)},
		{Type: EventStarted, CreatedAt: t0.Add(8 * time.Second)},
		{ActionID: "deploy", Type: EventActionStarted, CreatedAt: t0.Add(9 * time.Second)},
		{ActionID: "deploy", Type: EventActionCompleted, Outputs: map[string]any{"IMAGE": "two", "URL": "example"}, CreatedAt: t0.Add(10 * time.Second)},
		{Type: EventCompleted, CreatedAt: t0.Add(11 * time.Second)},
	}

	state := Fold(events)
	if state.Status != "completed" || state.Error != "" || state.CurrentActionID != "" {
		t.Fatalf("unexpected execution state: %+v", state)
	}
	if !state.StartedAt.Equal(t0.Add(time.Second)) || !state.CompletedAt.Equal(t0.Add(11*time.Second)) {
		t.Fatalf("unexpected execution timestamps: %+v", state)
	}
	if !reflect.DeepEqual(state.Outputs, map[string]any{"IMAGE": "two", "URL": "example"}) {
		t.Fatalf("outputs = %#v", state.Outputs)
	}
	if state.Actions["build"].Attempt != 1 || state.Actions["deploy"].Attempt != 2 {
		t.Fatalf("attempts = %#v", state.Actions)
	}
}

func TestRecorderCrashRecovery(t *testing.T) {
	store := &fakeEventStore{
		attempt: 1,
		events: []Event{
			{Type: EventQueued},
			{Type: EventStarted},
			{ActionID: "build", Type: EventActionStarted},
			{ActionID: "build", Type: EventActionCompleted},
			{ActionID: "test_unit", Type: EventActionStarted},
			{ActionID: "test_integration", Type: EventActionStarted},
			{ActionID: "test_integration", Type: EventActionCompleted},
		},
	}
	recorder, err := newRecorder(context.Background(), store, "exec")
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]bool{
		"build": false, "test_integration": false, "test_unit": true, "deploy": true,
	} {
		if got := recorder.Runnable(id); got != want {
			t.Errorf("Runnable(%q) = %v, want %v", id, got, want)
		}
	}
	if recorder.attempt != 2 {
		t.Fatalf("attempt = %d, want 2", recorder.attempt)
	}
}

func TestRecorderApprovalDoesNotConsumeAttempt(t *testing.T) {
	store := &fakeEventStore{}
	recorder, err := newRecorder(context.Background(), store, "exec")
	if err != nil {
		t.Fatal(err)
	}
	if err := recorder.BlockAction(context.Background(), "deploy", ErrPendingApproval); err != nil {
		t.Fatal(err)
	}
	if got := recorder.Attempt("deploy"); got != 0 {
		t.Fatalf("attempt = %d, want 0", got)
	}
	if len(store.events) != 1 || store.events[0].Type != EventActionBlocked {
		t.Fatalf("events = %#v", store.events)
	}
}

type fakeEventStore struct {
	events     []Event
	attempt    int32
	superseded bool
}

func (s *fakeEventStore) AppendEvent(_ context.Context, event Event) error {
	if s.superseded || event.Attempt != s.attempt {
		return ErrSuperseded
	}
	s.events = append(s.events, event)
	return nil
}

func (s *fakeEventStore) LoadEvents(_ context.Context, _ string) ([]Event, error) {
	return append([]Event(nil), s.events...), nil
}

func (s *fakeEventStore) BeginAttempt(_ context.Context, _ string) (int32, error) {
	if s.superseded {
		return 0, ErrStaleJob
	}
	s.attempt++
	return s.attempt, nil
}

func TestRecorderSuperseded(t *testing.T) {
	store := &fakeEventStore{}
	recorder, err := newRecorder(context.Background(), store, "exec")
	if err != nil {
		t.Fatal(err)
	}
	store.superseded = true
	if err := recorder.Start(context.Background()); !errors.Is(err, ErrSuperseded) {
		t.Fatalf("error = %v, want ErrSuperseded", err)
	}
	if len(store.events) != 0 {
		t.Fatalf("stale append moved log: %#v", store.events)
	}
}
