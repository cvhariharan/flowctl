package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/cvhariharan/autopilot/internal/executor"
	"github.com/cvhariharan/autopilot/internal/runner"
	"github.com/cvhariharan/autopilot/internal/streamlogger"
	"github.com/expr-lang/expr"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	TypeFlowExecution   = "flow_execution"
	TypeActionExecution = "action_execution"
	MaxRetries          = 0
)

var (
	ErrPendingApproval = errors.New("pending approval")
)

type FlowExecutionPayload struct {
	Workflow          Flow
	Input             map[string]interface{}
	StartingActionIdx int
	ExecID            string
	ParentExecID      string
	NamespaceID       string
}

type HookFn func(ctx context.Context, execID, parentExecID string, action Action, namespaceID string) error

func NewFlowExecution(f Flow, input map[string]interface{}, startingActionIdx int, ExecID, parentExecID, namespaceID string) (*asynq.Task, error) {
	payload, err := json.Marshal(FlowExecutionPayload{Workflow: f, Input: input, StartingActionIdx: startingActionIdx, ExecID: ExecID, ParentExecID: parentExecID, NamespaceID: namespaceID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeFlowExecution, payload, asynq.MaxRetry(MaxRetries)), nil
}

type FlowRunner struct {
	artifactManager  runner.ArtifactManager
	onBeforeActionFn HookFn
	onAfterActionFn  HookFn
	debugLogger      *slog.Logger
	redisClient      redis.UniversalClient
}

func NewFlowRunner(redisClient redis.UniversalClient, artifactManager runner.ArtifactManager, onBeforeActionFn, onAfterActionFn HookFn, debugLogger *slog.Logger) *FlowRunner {
	return &FlowRunner{redisClient: redisClient, artifactManager: artifactManager, onBeforeActionFn: onBeforeActionFn, onAfterActionFn: onAfterActionFn, debugLogger: debugLogger.With("component", "flow_runner")}
}

func (r *FlowRunner) HandleFlowExecution(ctx context.Context, t *asynq.Task) error {
	var payload FlowExecutionPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	if payload.StartingActionIdx < 0 {
		payload.StartingActionIdx = 0
	}
	if payload.StartingActionIdx > len(payload.Workflow.Actions) {
		payload.StartingActionIdx = len(payload.Workflow.Actions)
	}

	streamID := payload.ExecID
	if payload.ParentExecID != "" {
		streamID = payload.ParentExecID
	}

	streamLogger := streamlogger.NewStreamLogger(r.redisClient).WithID(streamID)
	defer streamLogger.Close(payload.ExecID)

	for i := payload.StartingActionIdx; i < len(payload.Workflow.Actions); i++ {
		action := payload.Workflow.Actions[i]

		if r.onBeforeActionFn != nil {
			if err := r.onBeforeActionFn(ctx, payload.ExecID, payload.ParentExecID, action, payload.NamespaceID); err != nil {
				r.debugLogger.Debug("could not run before action func", "error", err)
				return err
			}
		}

		// Only run action if it has not already run, if it has run, use the existing results
		res, err := streamLogger.Results(action.ID)
		if err != nil {
			res, err = r.runAction(ctx, action, payload.Workflow.Meta.SrcDir, payload.Input, streamLogger)
			if err != nil {
				streamLogger.Checkpoint(action.ID, err.Error(), streamlogger.ErrMessageType)
				return err
			}
		}
		if err := streamLogger.Checkpoint(action.ID, res, streamlogger.ResultMessageType); err != nil {
			return err
		}

		if r.onAfterActionFn != nil {
			if err := r.onAfterActionFn(ctx, payload.ExecID, payload.ParentExecID, action, payload.NamespaceID); err != nil {
				r.debugLogger.Debug("could not run after action func", "error", err)
				return err
			}
		}
	}

	return nil
}

func (r *FlowRunner) runAction(ctx context.Context, action Action, srcdir string, input map[string]interface{}, streamlogger *streamlogger.StreamLogger) (map[string]string, error) {
	streamlogger = streamlogger.WithActionID(action.ID)
	var exec executor.Executor
	switch action.Executor {
	case "docker":
		var err error
		exec, err = executor.NewDockerExecutor(action.ID, executor.DockerRunnerOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create docker executor for action %s: %w", action.ID, err)
		}
	}
	defer exec.Close()

	// pattern to extract interpolated variables
	pattern := `{{\s*([^}]+)\s*}}`
	re := regexp.MustCompile(pattern)

	jobCtx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()

	// Iterate over all the flow variables execute variable interpolation if required
	inputVars := make(map[string]interface{})
	for _, variable := range action.Variables {
		matches := re.FindAllStringSubmatch(variable.Value(), -1)
		if len(matches) > 0 {
			inputExpr := matches[0][1]
			env := map[string]interface{}{
				"input":   input,
				"secrets": viper.GetStringMapString("secrets"),
			}

			program, err := expr.Compile(inputExpr, expr.Env(env))
			if err != nil {
				return nil, fmt.Errorf("failed to compile expression: %w", err)
			}

			output, err := expr.Run(program, env)
			if err != nil {
				return nil, fmt.Errorf("failed to run expression: %w", err)
			}

			inputVars[variable.Name()] = output
		}
	}

	withConfig, err := yaml.Marshal(action.With)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal 'with' config: %w", err)
	}

	if len(action.On) == 0 {
		action.On = append(action.On, Node{})
	}

	type ExecResults struct {
		result map[string]string
		err    error
	}
	var wg sync.WaitGroup
	resChan := make(chan ExecResults, len(action.On))

	for _, node := range action.On {
		wg.Add(1)
		go func(node Node, resChan chan ExecResults) {
			defer wg.Done()
			res, err := exec.Execute(jobCtx, executor.ExecutionContext{
				Inputs:     inputVars,
				WithConfig: withConfig,
				Artifacts:  action.Artifacts,
				Stdout:     streamlogger,
				Stderr:     streamlogger,
				Node: executor.Node{
					Hostname: node.Hostname,
					Port:     node.Port,
					Username: node.Username,
					Auth: executor.NodeAuth{
						Method: string(node.Auth.Method),
						Key:    node.Auth.Key,
					},
				},
			})

			// Add node.Name prefix to result keys
			prefixedRes := make(map[string]string)
			if res != nil {
				for key, value := range res {
					prefixedKey := key
					if node.Name != "" {
						prefixedKey = node.Name + "." + key
					}
					prefixedRes[prefixedKey] = value
				}
			}

			resChan <- ExecResults{
				result: prefixedRes,
				err:    err,
			}
		}(node, resChan)
	}

	wg.Wait()
	close(resChan)

	// Merge all results into a single map
	mergedResults := make(map[string]string)
	for res := range resChan {
		if res.err != nil {
			return nil, res.err
		}
		for key, value := range res.result {
			mergedResults[key] = value
		}
	}

	return mergedResults, nil
}
