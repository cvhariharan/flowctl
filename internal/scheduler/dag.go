package scheduler

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

func actionStarted(state ActionState) ActionState {
	now := time.Now()
	state.Status = ActionStatusRunning
	state.StartedAt = &now
	state.Error = ""
	return state
}

func actionFinished(state ActionState, status ActionStatus, err error) ActionState {
	now := time.Now()
	state.Status = status
	state.FinishedAt = &now
	if err != nil {
		state.Error = err.Error()
	}
	return state
}

func actionOutcome(state ActionState, err error) ActionState {
	switch {
	case err == nil:
		return actionFinished(state, ActionStatusCompleted, nil)
	case errors.Is(err, ErrPendingApproval):
		state.Status = ActionStatusBlocked
		return state
	case errors.Is(err, ErrExecutionCancelled), errors.Is(err, context.Canceled):
		return actionFinished(state, ActionStatusCancelled, nil)
	default:
		return actionFinished(state, ActionStatusFailed, err)
	}
}

// Graph is the dependency graph of a flow's actions. Actions keep their definition order so that
// dispatch is deterministic when several are eligible at once.
type Graph struct {
	actions  []Action
	needs    map[string][]string
	children map[string][]string
}

// BuildGraph validates the dependencies between actions and builds the graph.
func BuildGraph(actions []Action) (*Graph, error) {
	g := &Graph{
		actions:  actions,
		needs:    make(map[string][]string, len(actions)),
		children: make(map[string][]string, len(actions)),
	}

	known := make(map[string]bool, len(actions))
	for _, a := range actions {
		known[a.ID] = true
	}

	for _, a := range actions {
		var deps []string
		for _, dep := range a.Needs {
			if dep == a.ID {
				return nil, fmt.Errorf("action %q cannot depend on itself", a.ID)
			}
			if !known[dep] {
				return nil, fmt.Errorf("action %q needs unknown action %q", a.ID, dep)
			}
			if slices.Contains(deps, dep) {
				return nil, fmt.Errorf("action %q lists %q in needs more than once", a.ID, dep)
			}
			deps = append(deps, dep)
			g.children[dep] = append(g.children[dep], a.ID)
		}
		g.needs[a.ID] = deps
	}

	if err := g.checkCycles(); err != nil {
		return nil, err
	}

	return g, nil
}

// dagRun tracks the progress of a dag execution. It owns the action states and decides what may be
// dispatched next, so the scheduling rules can be exercised without a store or an executor.
type dagRun struct {
	graph  *Graph
	states map[string]ActionState
	limit  int

	inflight        int
	halted          bool
	cancelled       bool
	pendingApproval bool
	firstErr        error
}

func newDAGRun(graph *Graph, states map[string]ActionState, maxParallel int) *dagRun {
	limit := maxParallel
	if limit <= 0 || limit > len(graph.actions) {
		limit = len(graph.actions)
	}

	return &dagRun{graph: graph, states: states, limit: limit}
}

// next returns an action to dispatch and marks it running. Dispatching stops once the run has
// halted or the parallelism limit is reached.
func (r *dagRun) next() (Action, bool) {
	if r.halted || r.inflight >= r.limit {
		return Action{}, false
	}

	action, ok := r.graph.NextEligible(r.states)
	if !ok {
		return Action{}, false
	}

	r.states[action.ID] = actionStarted(r.states[action.ID])
	r.inflight++
	return action, true
}

func (r *dagRun) active() bool {
	return r.inflight > 0
}

// record stores the result of a finished action. A failure stops further dispatch but lets actions
// that are already running finish.
func (r *dagRun) record(action Action, err error) {
	r.inflight--
	r.states[action.ID] = actionOutcome(r.states[action.ID], err)

	switch {
	case err == nil:
	case errors.Is(err, ErrPendingApproval):
		r.pendingApproval = true
	case errors.Is(err, ErrExecutionCancelled), errors.Is(err, context.Canceled):
		r.cancelled, r.halted = true, true
	default:
		r.halted = true
		if r.firstErr == nil {
			r.firstErr = err
		}
	}
}

func (r *dagRun) undispatch(action Action) {
	r.inflight--
	state := r.states[action.ID]
	state.Status = ActionStatusPending
	state.StartedAt = nil
	r.states[action.ID] = state
}

// halt stops further dispatch without touching action state. Actions already running are left to
// finish.
func (r *dagRun) halt(err error) {
	r.halted = true
	if r.firstErr == nil {
		r.firstErr = err
	}
}

// finish reports how the execution ended. A failure outranks a pending approval, so approving an
// execution that also has a failed branch does not resume into an immediate error.
func (r *dagRun) finish() error {
	switch {
	case r.cancelled:
		return ErrExecutionCancelled
	case r.halted:
		return r.firstErr
	case r.pendingApproval:
		return ErrPendingApproval
	default:
		return nil
	}
}

// NextEligible returns the next pending action whose dependencies have all completed. Actions are
// checked in definition order so dispatch is deterministic.
func (g *Graph) NextEligible(states map[string]ActionState) (Action, bool) {
	for _, a := range g.actions {
		if states[a.ID].Status != ActionStatusPending {
			continue
		}
		ready := true
		for _, dep := range g.needs[a.ID] {
			if states[dep].Status != ActionStatusCompleted {
				ready = false
				break
			}
		}
		if ready {
			return a, true
		}
	}
	return Action{}, false
}

func (g *Graph) Pending(states map[string]ActionState) []string {
	var pending []string
	for _, action := range g.actions {
		if states[action.ID].Status == ActionStatusPending {
			pending = append(pending, action.ID)
		}
	}
	return pending
}

// Roots returns the IDs of actions that have no dependencies.
func (g *Graph) Roots() []string {
	var roots []string
	for _, a := range g.actions {
		if len(g.needs[a.ID]) == 0 {
			roots = append(roots, a.ID)
		}
	}
	return roots
}

// Levels groups actions into topological layers. Every action in a layer can run concurrently.
func (g *Graph) Levels() [][]string {
	depth := make(map[string]int, len(g.actions))
	for _, id := range g.topoOrder() {
		for _, dep := range g.needs[id] {
			if depth[dep]+1 > depth[id] {
				depth[id] = depth[dep] + 1
			}
		}
	}

	var levels [][]string
	for _, a := range g.actions {
		d := depth[a.ID]
		for len(levels) <= d {
			levels = append(levels, nil)
		}
		levels[d] = append(levels[d], a.ID)
	}
	return levels
}

// Descendants returns every action reachable from id.
func (g *Graph) Descendants(id string) []string {
	seen := make(map[string]bool)
	queue := slices.Clone(g.children[id])
	var out []string
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		if seen[next] {
			continue
		}
		seen[next] = true
		out = append(out, next)
		queue = append(queue, g.children[next]...)
	}
	return out
}

// topoOrder returns action IDs in dependency order. BuildGraph has already rejected cycles.
func (g *Graph) topoOrder() []string {
	remaining := make(map[string]int, len(g.actions))
	for _, a := range g.actions {
		remaining[a.ID] = len(g.needs[a.ID])
	}

	order := make([]string, 0, len(g.actions))
	for len(order) < len(g.actions) {
		progressed := false
		for _, a := range g.actions {
			if count, ok := remaining[a.ID]; !ok || count != 0 {
				continue
			}
			delete(remaining, a.ID)
			order = append(order, a.ID)
			progressed = true
			for _, child := range g.children[a.ID] {
				remaining[child]--
			}
		}
		if !progressed {
			break
		}
	}
	return order
}

func (g *Graph) checkCycles() error {
	const (
		unvisited = 0
		active    = 1
		done      = 2
	)

	state := make(map[string]int, len(g.actions))
	var path []string

	var visit func(id string) error
	visit = func(id string) error {
		switch state[id] {
		case done:
			return nil
		case active:
			cycle := append(slices.Clone(path[slices.Index(path, id):]), id)
			return fmt.Errorf("dependency cycle: %s", strings.Join(cycle, " -> "))
		}

		state[id] = active
		path = append(path, id)
		for _, dep := range g.needs[id] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		path = path[:len(path)-1]
		state[id] = done
		return nil
	}

	for _, a := range g.actions {
		if err := visit(a.ID); err != nil {
			return err
		}
	}
	return nil
}
