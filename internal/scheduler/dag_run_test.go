package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
)

func pendingStates(actions []Action) map[string]ActionState {
	return seedActionStates(actions, nil)
}

func statuses(states map[string]ActionState) map[string]ActionStatus {
	out := make(map[string]ActionStatus, len(states))
	for id, s := range states {
		out[id] = s.Status
	}
	return out
}

// drain runs a dag to completion, resolving each action with results[id]. Actions are released in
// the order they were dispatched, which is enough to exercise the scheduling rules.
func drain(t *testing.T, actions []Action, maxParallel int, results map[string]error) (*dagRun, []string, int) {
	t.Helper()

	graph := mustGraph(t, actions)
	run := newDAGRun(graph, pendingStates(actions), maxParallel)

	var dispatched []string
	var queue []Action
	maxInflight := 0

	for {
		for {
			action, ok := run.next()
			if !ok {
				break
			}
			dispatched = append(dispatched, action.ID)
			queue = append(queue, action)
		}

		if run.inflight > maxInflight {
			maxInflight = run.inflight
		}

		if !run.active() {
			break
		}

		next := queue[0]
		queue = queue[1:]
		run.record(next, results[next.ID])
	}

	return run, dispatched, maxInflight
}

func TestDAGRun_DiamondOrder(t *testing.T) {
	actions := acts(
		[]string{"build"},
		[]string{"test_unit", "build"},
		[]string{"test_integration", "build"},
		[]string{"deploy", "test_unit", "test_integration"},
	)

	run, dispatched, maxInflight := drain(t, actions, 0, nil)

	if err := run.finish(); err != nil {
		t.Fatalf("finish() = %v, want nil", err)
	}

	want := []string{"build", "test_unit", "test_integration", "deploy"}
	if fmt.Sprint(dispatched) != fmt.Sprint(want) {
		t.Errorf("dispatch order = %v, want %v", dispatched, want)
	}
	if maxInflight != 2 {
		t.Errorf("max concurrent actions = %d, want 2 (the two tests)", maxInflight)
	}
	for id, status := range statuses(run.states) {
		if status != ActionStatusCompleted {
			t.Errorf("action %s = %s, want completed", id, status)
		}
	}
}

func TestDAGRun_MaxParallel(t *testing.T) {
	actions := acts([]string{"a"}, []string{"b"}, []string{"c"}, []string{"d"}, []string{"e"})

	for _, limit := range []int{1, 2, 3} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			run, dispatched, maxInflight := drain(t, actions, limit, nil)
			if err := run.finish(); err != nil {
				t.Fatalf("finish() = %v, want nil", err)
			}
			if maxInflight > limit {
				t.Errorf("max concurrent actions = %d, want at most %d", maxInflight, limit)
			}
			if len(dispatched) != len(actions) {
				t.Errorf("dispatched %d actions, want %d", len(dispatched), len(actions))
			}
		})
	}
}

func TestDAGRun_MaxParallelUnboundedByDefault(t *testing.T) {
	actions := acts([]string{"a"}, []string{"b"}, []string{"c"})
	_, _, maxInflight := drain(t, actions, 0, nil)
	if maxInflight != 3 {
		t.Errorf("max concurrent actions = %d, want 3", maxInflight)
	}
}

func TestDAGRun_FailureSkipsDescendantsAndKeepsSiblings(t *testing.T) {
	actions := acts(
		[]string{"build"},
		[]string{"flaky", "build"},
		[]string{"slow", "build"},
		[]string{"deploy", "flaky"},
		[]string{"notify", "slow"},
	)

	boom := errors.New("boom")
	run, _, _ := drain(t, actions, 0, map[string]error{"flaky": boom})

	if err := run.finish(); !errors.Is(err, boom) {
		t.Fatalf("finish() = %v, want %v", err, boom)
	}

	got := statuses(run.states)
	want := map[string]ActionStatus{
		"build":  ActionStatusCompleted,
		"flaky":  ActionStatusFailed,
		"slow":   ActionStatusCompleted, // already dispatched when flaky failed
		"deploy": ActionStatusPending,
		"notify": ActionStatusPending, // the recorder emits skipped events for these
	}
	for id, wantStatus := range want {
		if got[id] != wantStatus {
			t.Errorf("action %s = %s, want %s", id, got[id], wantStatus)
		}
	}

	if msg := run.states["flaky"].Error; msg != "boom" {
		t.Errorf("failed action error = %q, want %q", msg, "boom")
	}
}

func TestDAGRun_ApprovalBlocksOnlyItsBranch(t *testing.T) {
	actions := acts(
		[]string{"build"},
		[]string{"gate", "build"},
		[]string{"deploy", "gate"},
		[]string{"docs", "build"},
	)

	run, _, _ := drain(t, actions, 0, map[string]error{"gate": ErrPendingApproval})

	if err := run.finish(); !errors.Is(err, ErrPendingApproval) {
		t.Fatalf("finish() = %v, want ErrPendingApproval", err)
	}

	got := statuses(run.states)
	want := map[string]ActionStatus{
		"build":  ActionStatusCompleted,
		"gate":   ActionStatusBlocked,
		"deploy": ActionStatusPending, // waiting, not skipped
		"docs":   ActionStatusCompleted,
	}
	for id, wantStatus := range want {
		if got[id] != wantStatus {
			t.Errorf("action %s = %s, want %s", id, got[id], wantStatus)
		}
	}

	if run.states["gate"].StartedAt == nil {
		t.Error("blocked action lost its start time")
	}
	if run.states["gate"].FinishedAt != nil {
		t.Error("blocked action was marked finished")
	}
}

// A failure has to outrank a pending approval, otherwise approving would resume an execution that
// immediately fails again.
func TestDAGRun_FailureOutranksPendingApproval(t *testing.T) {
	actions := acts([]string{"gate"}, []string{"broken"})

	boom := errors.New("boom")
	run, _, _ := drain(t, actions, 0, map[string]error{
		"gate":   ErrPendingApproval,
		"broken": boom,
	})

	if err := run.finish(); !errors.Is(err, boom) {
		t.Fatalf("finish() = %v, want %v", err, boom)
	}
}

func TestDAGRun_Cancellation(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"execution cancelled", ErrExecutionCancelled},
		{"context canceled", context.Canceled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := acts([]string{"a"}, []string{"b", "a"})
			run, _, _ := drain(t, actions, 1, map[string]error{"a": tt.err})

			if err := run.finish(); !errors.Is(err, ErrExecutionCancelled) {
				t.Fatalf("finish() = %v, want ErrExecutionCancelled", err)
			}
			if got := run.states["a"].Status; got != ActionStatusCancelled {
				t.Errorf("action a = %s, want cancelled", got)
			}
			if got := run.states["b"].Status; got != ActionStatusPending {
				t.Errorf("action b = %s, want pending", got)
			}
		})
	}
}

func TestDAGRun_NoDeadlockWhenBlockedBranchHasDescendants(t *testing.T) {
	actions := acts(
		[]string{"gate"},
		[]string{"a", "gate"},
		[]string{"b", "a"},
	)

	run, dispatched, _ := drain(t, actions, 0, map[string]error{"gate": ErrPendingApproval})

	if len(dispatched) != 1 {
		t.Errorf("dispatched %v, want only the gate", dispatched)
	}
	if err := run.finish(); !errors.Is(err, ErrPendingApproval) {
		t.Errorf("finish() = %v, want ErrPendingApproval", err)
	}
}

func TestSeedActionStates(t *testing.T) {
	actions := acts(
		[]string{"done"},
		[]string{"failed"},
		[]string{"blocked"},
		[]string{"skipped"},
		[]string{"interrupted"},
		[]string{"fresh"},
	)

	prev := map[string]ActionState{
		"done":        {Status: ActionStatusCompleted},
		"failed":      {Status: ActionStatusFailed, Attempt: 2, Error: "boom"},
		"blocked":     {Status: ActionStatusBlocked},
		"skipped":     {Status: ActionStatusSkipped},
		"interrupted": {Status: ActionStatusRunning},
		"removed":     {Status: ActionStatusCompleted},
	}

	got := statuses(seedActionStates(actions, prev))
	want := map[string]ActionStatus{
		"done":        ActionStatusCompleted,
		"failed":      ActionStatusPending,
		"blocked":     ActionStatusPending,
		"skipped":     ActionStatusPending,
		"interrupted": ActionStatusPending,
		"fresh":       ActionStatusPending,
	}

	if len(got) != len(want) {
		keys := make([]string, 0, len(got))
		for k := range got {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		t.Fatalf("reset produced %v, want keys for the flow's actions only", keys)
	}
	for id, wantStatus := range want {
		if got[id] != wantStatus {
			t.Errorf("action %s = %s, want %s", id, got[id], wantStatus)
		}
	}
	if _, ok := got["removed"]; ok {
		t.Error("state for an action no longer in the flow was kept")
	}
	if got := seedActionStates(actions, prev)["failed"].Error; got != "" {
		t.Errorf("reset action kept its error %q", got)
	}
	if got := seedActionStates(actions, prev)["failed"].Attempt; got != 2 {
		t.Errorf("reset action attempt = %d, want 2", got)
	}
}

// A resumed run must not re-run completed actions, and must pick up from the frontier.
func TestDAGRun_ResumeSkipsCompleted(t *testing.T) {
	actions := acts(
		[]string{"build"},
		[]string{"gate", "build"},
		[]string{"deploy", "gate"},
	)

	states := seedActionStates(actions, map[string]ActionState{
		"build": {Status: ActionStatusCompleted},
		"gate":  {Status: ActionStatusBlocked},
	})

	run := newDAGRun(mustGraph(t, actions), states, 0)

	var dispatched []string
	for {
		action, ok := run.next()
		if !ok {
			break
		}
		dispatched = append(dispatched, action.ID)
		run.record(action, nil)
	}

	want := []string{"gate", "deploy"}
	if fmt.Sprint(dispatched) != fmt.Sprint(want) {
		t.Errorf("dispatched %v, want %v", dispatched, want)
	}
	if err := run.finish(); err != nil {
		t.Errorf("finish() = %v, want nil", err)
	}
}
