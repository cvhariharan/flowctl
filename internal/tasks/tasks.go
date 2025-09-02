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

	"github.com/cvhariharan/flowctl/internal/streamlogger"
	"github.com/cvhariharan/flowctl/sdk/executor"
	"github.com/expr-lang/expr"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

const (
	TypeFlowExecution     = "flow_execution"
	TypeActionExecution   = "action_execution"
	MaxRetries            = 0
	CancellationSignalKey = "cancelled_executions"
)

var (
	ErrPendingApproval    = errors.New("pending approval")
	ErrExecutionCancelled = errors.New("execution cancelled")
)

type TriggerType string

const (
	TriggerTypeManual    TriggerType = "manual"
	TriggerTypeScheduled TriggerType = "scheduled"
)

type FlowExecutionPayload struct {
	Workflow          Flow
	Input             map[string]interface{}
	StartingActionIdx int
	ExecID            string
	NamespaceID       string
	TriggerType       TriggerType
	UserUUID          string
}

type HookFn func(ctx context.Context, execID string, action Action, namespaceID string) error
type SecretsProviderFn func(ctx context.Context, flowID string, namespaceID string) (map[string]string, error)

func NewFlowExecution(f Flow, input map[string]interface{}, startingActionIdx int, ExecID, namespaceID string, triggerType TriggerType, userUUID string) (*asynq.Task, error) {
	// Apply default values for empty or missing input parameters
	processedInput := applyDefaultValues(f.Inputs, input)

	payload, err := json.Marshal(FlowExecutionPayload{Workflow: f, Input: processedInput, StartingActionIdx: startingActionIdx, ExecID: ExecID, NamespaceID: namespaceID, TriggerType: triggerType, UserUUID: userUUID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeFlowExecution, payload, asynq.MaxRetry(MaxRetries)), nil
}

// applyDefaultValues applies default values for empty or missing input parameters
func applyDefaultValues(inputs []Input, userInput map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range userInput {
		result[k] = v
	}

	for _, inputDef := range inputs {
		value, exists := result[inputDef.Name]

		if !exists || (value == "") {
			if inputDef.Default != "" {
				result[inputDef.Name] = inputDef.Default
			}
		}
	}

	return result
}

type FlowRunner struct {
	onBeforeActionFn HookFn
	onAfterActionFn  HookFn
	secretsProvider  SecretsProviderFn
	debugLogger      *slog.Logger
	redisClient      redis.UniversalClient
}

func NewFlowRunner(redisClient redis.UniversalClient, onBeforeActionFn, onAfterActionFn HookFn, secretsProvider SecretsProviderFn, debugLogger *slog.Logger) *FlowRunner {
	return &FlowRunner{redisClient: redisClient, onBeforeActionFn: onBeforeActionFn, onAfterActionFn: onAfterActionFn, secretsProvider: secretsProvider, debugLogger: debugLogger.With("component", "flow_runner")}
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

	// Create cancellable context for this execution
	execCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start cancellation monitoring goroutine
	cancelChan := make(chan struct{})
	go r.monitorCancellation(payload.ExecID, cancel, cancelChan)
	defer close(cancelChan)

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

	// Get flow-specific secrets
	var flowSecrets map[string]string
	if r.secretsProvider != nil {
		var err error
		flowSecrets, err = r.secretsProvider(ctx, payload.Workflow.Meta.ID, payload.NamespaceID)
		if err != nil {
			r.debugLogger.Error("failed to get flow secrets", "error", err)
			flowSecrets = make(map[string]string)
		}
	} else {
		flowSecrets = make(map[string]string)
	}

	for i := payload.StartingActionIdx; i < len(payload.Workflow.Actions); i++ {
		if execCtx.Err() != nil {
			// Send a cancelled message before returning
			if err := streamLogger.Checkpoint("", "execution cancelled", streamlogger.CancelledMessageType); err != nil {
				r.debugLogger.Debug("failed to send cancelled message", "error", err)
			}
			return ErrExecutionCancelled
		}
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
			res, err = r.runAction(execCtx, action, payload.Workflow.Meta.SrcDir, payload.Input, streamLogger, artifactDir, flowSecrets)
			if err != nil {
				// Check if the error is due to context cancellation
				r.debugLogger.Debug("context cancelled action", "err", err, "is", errors.Is(err, context.Canceled))
				if errors.Is(err, context.Canceled) {
					// Send a cancelled message before returning
					if streamErr := streamLogger.Checkpoint(action.ID, "execution cancelled", streamlogger.CancelledMessageType); streamErr != nil {
						r.debugLogger.Debug("failed to send cancelled message", "error", streamErr)
					}
					return ErrExecutionCancelled
				}
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

func (r *FlowRunner) runAction(ctx context.Context, action Action, srcdir string, input map[string]interface{}, streamlogger *streamlogger.StreamLogger, artifactDir string, secrets map[string]string) (map[string]string, error) {
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
				"secrets": secrets,
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
				Hostname:       node.Hostname,
				Port:           node.Port,
				Username:       node.Username,
				ConnectionType: node.ConnectionType,
				Auth: executor.NodeAuth{
					Method: string(node.Auth.Method),
					Key:    node.Auth.Key,
				},
			}
			ef, err := executor.GetNewExecutorFunc(action.Executor)
			if err != nil {
				resChan <- ExecResults{
					result: nil,
					err:    fmt.Errorf("failed to get executor for %s: %w", action.ID, err),
				}
				return
			}
			exec, err = ef(nodeExecutorID, execNode)
			if err != nil {
				resChan <- ExecResults{
					result: nil,
					err:    fmt.Errorf("failed to create executor for %s: %w", action.ID, err),
				}
				return
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
			// Check if any executor returned a context cancellation error
			if errors.Is(res.err, context.Canceled) {
				return nil, context.Canceled
			}
			return nil, res.err
		}
		for key, value := range res.result {
			mergedResults[key] = value
		}
	}

	return mergedResults, nil
}

// isCancelled checks if the execution has been cancelled by looking for a cancellation signal in Redis
func (r *FlowRunner) isCancelled(execID string) bool {
	key := fmt.Sprintf("%s:%s", CancellationSignalKey, execID)
	exists, err := r.redisClient.Exists(context.Background(), key).Result()
	if err != nil {
		r.debugLogger.Debug("failed to check cancellation status", "execID", execID, "error", err)
		return false
	}
	return exists > 0
}

// monitorCancellation continuously monitors for cancellation signals and calls cancel when detected
func (r *FlowRunner) monitorCancellation(execID string, cancel context.CancelFunc, done <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Second) // Check every second for quick response
	defer ticker.Stop()

	for {
		select {
		case <-done:
			// Execution finished, stop monitoring
			return
		case <-ticker.C:
			if r.isCancelled(execID) {
				r.debugLogger.Debug("cancellation signal detected, cancelling execution", "execID", execID)
				cancel()
				return
			}
		}
	}
}
