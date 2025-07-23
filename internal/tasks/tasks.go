package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"path/filepath"
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
	NamespaceID       string
}

type HookFn func(ctx context.Context, execID string, action Action, namespaceID string) error

func NewFlowExecution(f Flow, input map[string]interface{}, startingActionIdx int, ExecID, namespaceID string) (*asynq.Task, error) {
	payload, err := json.Marshal(FlowExecutionPayload{Workflow: f, Input: input, StartingActionIdx: startingActionIdx, ExecID: ExecID, NamespaceID: namespaceID})
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

	// Create temporary directory for artifacts shared across all actions in this flow
	artifactDir, err := os.MkdirTemp("", fmt.Sprintf("artifacts-%s-", payload.ExecID))
	if err != nil {
		return fmt.Errorf("failed to create artifact directory: %w", err)
	}
	log.Println("artifact dir: ", artifactDir)
	defer os.RemoveAll(artifactDir)

	streamID := payload.ExecID

	streamLogger := streamlogger.NewStreamLogger(r.redisClient).WithID(streamID)
	defer streamLogger.Close(payload.ExecID)

	for i := payload.StartingActionIdx; i < len(payload.Workflow.Actions); i++ {
		action := payload.Workflow.Actions[i]

		if r.onBeforeActionFn != nil {
			if err := r.onBeforeActionFn(ctx, payload.ExecID, action, payload.NamespaceID); err != nil {
				r.debugLogger.Debug("could not run before action func", "error", err)
				return err
			}
		}

		// Only run action if it has not already run, if it has run, use the existing results
		res, err := streamLogger.Results(action.ID)
		if err != nil {
			res, err = r.runAction(ctx, action, payload.Workflow.Meta.SrcDir, payload.Input, streamLogger, artifactDir)
			if err != nil {
				streamLogger.Checkpoint(action.ID, err.Error(), streamlogger.ErrMessageType)
				return err
			}
		}
		if err := streamLogger.Checkpoint(action.ID, res, streamlogger.ResultMessageType); err != nil {
			return err
		}

		if r.onAfterActionFn != nil {
			if err := r.onAfterActionFn(ctx, payload.ExecID, action, payload.NamespaceID); err != nil {
				r.debugLogger.Debug("could not run after action func", "error", err)
				return err
			}
		}
	}

	return nil
}

// pushArtifacts pushes existing artifact files from the artifact directory to the executor
func (r *FlowRunner) pushArtifacts(ctx context.Context, exec executor.Executor, artifactDir string) error {
	return filepath.WalkDir(artifactDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rPath, err := filepath.Rel(artifactDir, path)
			if err != nil {
				return err
			}
			if err := exec.PushFile(ctx, path, rPath); err != nil {
				return fmt.Errorf("failed to push artifact %s: %w", path, err)
			}
		}
		return nil
	})
}

// pullArtifacts pulls specified artifact files from the executor to the artifact directory
// If nodeName is provided, artifacts are placed in a subdirectory named after the node
func (r *FlowRunner) pullArtifacts(ctx context.Context, exec executor.Executor, artifactDir string, artifacts []string, nodeName string) error {
	for _, artifact := range artifacts {
		var localPath string
		if nodeName != "" {
			localPath = filepath.Join(artifactDir, nodeName, artifact)
		} else {
			localPath = filepath.Join(artifactDir, artifact)
		}

		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for artifact %s: %w", artifact, err)
		}

		if err := exec.PullFile(ctx, artifact, localPath); err != nil {
			return fmt.Errorf("failed to pull artifact %s from node %s: %w", artifact, nodeName, err)
		}
	}
	return nil
}

func (r *FlowRunner) runAction(ctx context.Context, action Action, srcdir string, input map[string]interface{}, streamlogger *streamlogger.StreamLogger, artifactDir string) (map[string]string, error) {
	streamlogger = streamlogger.WithActionID(action.ID)

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

			// Create a separate executor instance for each node
			var exec executor.Executor
			nodeExecutorID := fmt.Sprintf("%s-%s", action.ID, node.Name)
			if node.Name == "" {
				nodeExecutorID = action.ID
			}
			// Convert task node to executor node
			execNode := executor.Node{
				Hostname: node.Hostname,
				Port:     node.Port,
				Username: node.Username,
				Auth: executor.NodeAuth{
					Method: string(node.Auth.Method),
					Key:    node.Auth.Key,
				},
			}
			switch action.Executor {
			case "docker":
				var err error
				exec, err = executor.NewDockerExecutor(nodeExecutorID, executor.DockerRunnerOptions{}, execNode)
				if err != nil {
					resChan <- ExecResults{
						result: nil,
						err:    fmt.Errorf("failed to create docker executor for action %s node %s: %w", action.ID, node.Name, err),
					}
					return
				}
			case "script":
				var err error
				exec, err = executor.NewScriptExecutor(nodeExecutorID, execNode)
				if err != nil {
					resChan <- ExecResults{
						result: nil,
						err:    fmt.Errorf("failed to create docker executor for action %s node %s: %w", action.ID, node.Name, err),
					}
					return
				}
			}
			defer exec.Close()

			// Push existing artifacts to this node's executor before execution
			if err := r.pushArtifacts(jobCtx, exec, artifactDir); err != nil {
				resChan <- ExecResults{
					result: nil,
					err:    fmt.Errorf("failed to push artifacts to node %s: %w", node.Name, err),
				}
				return
			}

			res, err := exec.Execute(jobCtx, executor.ExecutionContext{
				Inputs:     inputVars,
				WithConfig: withConfig,
				Artifacts:  action.Artifacts,
				Stdout:     streamlogger,
				Stderr:     streamlogger,
			})

			// Pull artifacts from this node after successful execution
			if err == nil && len(action.Artifacts) > 0 {
				if pullErr := r.pullArtifacts(jobCtx, exec, artifactDir, action.Artifacts, node.Name); pullErr != nil {
					err = fmt.Errorf("execution succeeded but failed to pull artifacts: %w", pullErr)
				}
			}

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
