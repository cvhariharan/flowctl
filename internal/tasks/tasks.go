package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/cvhariharan/autopilot/internal/executor"
	"github.com/cvhariharan/autopilot/internal/runner"
	"github.com/cvhariharan/autopilot/internal/streamlogger"
	"github.com/expr-lang/expr"
	"github.com/hibiken/asynq"
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
}

type HookFn func(ctx context.Context, execID, parentExecID string, action Action) error

func NewFlowExecution(f Flow, input map[string]interface{}, startingActionIdx int, ExecID, parentExecID string) (*asynq.Task, error) {
	payload, err := json.Marshal(FlowExecutionPayload{Workflow: f, Input: input, StartingActionIdx: startingActionIdx, ExecID: ExecID, ParentExecID: parentExecID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeFlowExecution, payload, asynq.MaxRetry(MaxRetries)), nil
}

type FlowRunner struct {
	logger           *streamlogger.StreamLogger
	artifactManager  runner.ArtifactManager
	onBeforeActionFn HookFn
	onAfterActionFn  HookFn
}

func NewFlowRunner(logger *streamlogger.StreamLogger, artifactManager runner.ArtifactManager, onBeforeActionFn, onAfterActionFn HookFn) *FlowRunner {
	return &FlowRunner{logger: logger, artifactManager: artifactManager, onBeforeActionFn: onBeforeActionFn, onAfterActionFn: onAfterActionFn}
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

	streamLogger := r.logger.WithID(streamID)
	defer streamLogger.Close(payload.ExecID)

	for i := payload.StartingActionIdx; i < len(payload.Workflow.Actions); i++ {
		action := payload.Workflow.Actions[i]

		if r.onBeforeActionFn != nil {
			if err := r.onBeforeActionFn(ctx, payload.ExecID, payload.ParentExecID, action); err != nil {
				return err
			}
		}

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
			if err := r.onAfterActionFn(ctx, payload.ExecID, payload.ParentExecID, action); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *FlowRunner) runAction(ctx context.Context, action Action, srcdir string, input map[string]interface{}, streamlogger *streamlogger.StreamLogger) (map[string]string, error) {
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
		return exec.Execute(jobCtx, executor.ExecutionContext{
			Inputs:     inputVars,
			WithConfig: withConfig,
			Artifacts:  action.Artifacts,
			Stdout:     streamlogger,
			Stderr:     streamlogger,
		})
	}

	for _, node := range action.On {
		return exec.Execute(jobCtx, executor.ExecutionContext{
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
	}

	return nil, fmt.Errorf("could not run action %s on any nodes", action.ID)
}
