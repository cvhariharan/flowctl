package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/cvhariharan/autopilot/internal/runner"
	"github.com/docker/docker/api/types/mount"
	"github.com/expr-lang/expr"
	"github.com/hashicorp/go-envparse"
	"github.com/hibiken/asynq"
	"github.com/spf13/viper"
)

const (
	TypeFlowExecution   = "flow_execution"
	TypeActionExecution = "action_execution"
	MaxRetries          = 0
)

type FlowExecutionPayload struct {
	Workflow          models.Flow
	Input             map[string]interface{}
	StartingActionIdx int
	ExecID            string
	ParentExecID      string
}

type HookFn func(ctx context.Context, execID, parentExecID string, action models.Action) error

func NewFlowExecution(f models.Flow, input map[string]interface{}, startingActionIdx int, ExecID, parentExecID string) (*asynq.Task, error) {
	payload, err := json.Marshal(FlowExecutionPayload{Workflow: f, Input: input, StartingActionIdx: startingActionIdx, ExecID: ExecID, ParentExecID: parentExecID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeFlowExecution, payload, asynq.MaxRetry(MaxRetries)), nil
}

type FlowRunner struct {
	logger           *runner.StreamLogger
	artifactManager  runner.ArtifactManager
	onBeforeActionFn HookFn
	onAfterActionFn  HookFn
}

func NewFlowRunner(logger *runner.StreamLogger, artifactManager runner.ArtifactManager, onBeforeActionFn, onAfterActionFn HookFn) *FlowRunner {
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

	streamLogger := r.logger.WithID(payload.ExecID)
	defer streamLogger.Close()

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
				return err
			}
		}
		if err := streamLogger.Checkpoint(action.ID, res, models.ResultMessageType); err != nil {
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

func (r *FlowRunner) runAction(ctx context.Context, action models.Action, srcdir string, input map[string]interface{}, streamlogger *runner.StreamLogger) (map[string]string, error) {
	// Create temp file for outputs
	outfile, err := os.CreateTemp("", fmt.Sprintf("output-action-%s-*", action.ID))
	if err != nil {
		return nil, fmt.Errorf("could not create tmp file for storing action %s outputs: %w", action.ID, err)
	}
	defer func() {
		outfile.Close()
		os.Remove(outfile.Name())
	}()

	// pattern to extract interpolated variables
	pattern := `{{\s*([^}]+)\s*}}`
	re := regexp.MustCompile(pattern)

	jobCtx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()

	// Iterate over all the flow variables execute variable interpolation if required
	for i, variable := range action.Variables {
		matches := re.FindAllStringSubmatch(variable.Value(), -1)
		if len(matches) > 0 {
			inputExpr := matches[0][1]
			env := map[string]interface{}{
				"input": input,
			}

			program, err := expr.Compile(inputExpr, expr.Env(env))
			if err != nil {
				return nil, fmt.Errorf("failed to compile expression: %w", err)
			}

			output, err := expr.Run(program, env)
			if err != nil {
				return nil, fmt.Errorf("failed to run expression: %w", err)
			}

			action.Variables[i][action.Variables[i].Name()] = output
			log.Println(action.Variables[i])
		}
	}

	// Add output env variable
	action.Variables = append(action.Variables, map[string]interface{}{"OUTPUT": "/tmp/flow/output"})
	log.Println(filepath.Join(viper.GetString("app.flows_directory"), srcdir))
	err = runner.NewDockerRunner(action.ID, r.artifactManager, runner.DockerRunnerOptions{
		ShowImagePull: true,
		Stdout:        streamlogger,
		Stderr:        streamlogger,
	}).CreatesArtifacts(action.Artifacts).
		WithImage(action.Image).
		WithCmd(action.Script).
		WithEnv(action.Variables).
		WithEntrypoint(action.Entrypoint).
		// copy the files from flow directory
		WithSrc(filepath.Join(viper.GetString("app.flows_directory"), srcdir)).
		// Output file
		WithMount(mount.Mount{
			Type:   mount.TypeBind,
			Source: outfile.Name(),
			Target: "/tmp/flow/output",
		}).
		Run(jobCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to run docker runner: %w", err)
	}

	// Parse output file env
	outputTempFile, err := os.Open(outfile.Name())
	if err != nil {
		return nil, fmt.Errorf("error opening output file for reading: %w", err)
	}

	outputEnv, err := envparse.Parse(outputTempFile)
	if err != nil {
		return nil, fmt.Errorf("could not load output env: %w", err)
	}

	return outputEnv, nil
}
