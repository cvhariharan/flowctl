package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	coreexecstate "github.com/cvhariharan/flowctl/internal/core/execstate"
	"github.com/cvhariharan/flowctl/internal/metrics"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/cvhariharan/flowctl/internal/streamlogger"
	"github.com/cvhariharan/flowctl/sdk/executor"
	"github.com/expr-lang/expr"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const PayloadTypeFlowExecution PayloadType = "flow_execution"

// globalOutputKey is the reserved outputs bucket for FC_OUTPUT_GLOBAL values.
// Referenced from flows as outputs.global.<action_id>.<KEY>.
const globalOutputKey = "global"

// FlowExecutionHandler handles flow execution jobs
type FlowExecutionHandler struct {
	store            repo.Store
	secretsProvider  SecretsProviderFn
	logmanager       streamlogger.LogManager
	logger           *slog.Logger
	executionTimeout time.Duration
	maxParallel      int
	metrics          *metrics.Manager
	taskQueuer       TaskQueuer
	executorKeys     map[string]string // executor_name → API token
	apiBaseURL       string
}

type flowRunContext struct {
	execID        string
	workflowMeta  Metadata
	variableMeta  flowVariableMetadata
	input         map[string]any
	streamLogger  streamlogger.Logger
	artifactDir   string
	artifactMu    *sync.RWMutex
	secrets       map[string]string
	outputs       map[string]any
	namespaceID   string
	userUUID      string
	overrideNodes []Node
	// actionRetry is the attempt count of the action currently being run
	actionRetry int32
}

type flowVariableMetadata struct {
	ID          string
	Name        string
	Description string
	Namespace   string
}

func newFlowVariableMetadata(meta Metadata) flowVariableMetadata {
	return flowVariableMetadata{
		ID:          meta.ID,
		Name:        meta.Name,
		Description: meta.Description,
		Namespace:   meta.Namespace,
	}
}

// FlowHandlerConfig holds configuration for FlowExecutionHandler
type FlowHandlerConfig struct {
	Store                repo.Store
	SecretsProvider      SecretsProviderFn
	LogManager           streamlogger.LogManager
	Logger               *slog.Logger
	Metrics              *metrics.Manager
	FlowExecutionTimeout time.Duration
	MaxParallel          int
	ExecutorKeys         map[string]string // executor_name → API token
	APIBaseURL           string
}

// NewFlowExecutionHandler creates a new flow execution handler
func NewFlowExecutionHandler(cfg FlowHandlerConfig) *FlowExecutionHandler {
	if cfg.FlowExecutionTimeout == 0 {
		cfg.FlowExecutionTimeout = time.Hour
	}

	if cfg.MaxParallel <= 0 {
		cfg.MaxParallel = runtime.NumCPU()
	}

	return &FlowExecutionHandler{
		store:            cfg.Store,
		secretsProvider:  cfg.SecretsProvider,
		logmanager:       cfg.LogManager,
		logger:           cfg.Logger,
		metrics:          cfg.Metrics,
		executionTimeout: cfg.FlowExecutionTimeout,
		maxParallel:      cfg.MaxParallel,
		executorKeys:     cfg.ExecutorKeys,
		apiBaseURL:       cfg.APIBaseURL,
	}
}

// SetSecretsProvider allows updating secrets provider after creation
func (h *FlowExecutionHandler) SetSecretsProvider(sp SecretsProviderFn) {
	h.secretsProvider = sp
}

// SetTaskQueuer allows setting the task queuer after creation
func (h *FlowExecutionHandler) SetTaskQueuer(tq TaskQueuer) {
	h.taskQueuer = tq
}

// Type returns the payload type this handler processes
func (h *FlowExecutionHandler) Type() PayloadType {
	return PayloadTypeFlowExecution
}

// Handle processes a flow execution job
func (h *FlowExecutionHandler) Handle(ctx context.Context, job Job) error {
	var payload FlowExecutionPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal flow payload: %w", err)
	}

	if payload.Input == nil {
		payload.Input = make(map[string]any)
	}
	if payload.Outputs == nil {
		payload.Outputs = make(map[string]any)
	}

	// Apply default input values before the execution is written so the
	// stored context reflects the inputs the flow actually runs with. Cron
	// payloads carry only explicitly configured inputs.
	applyDefaultInputs(payload.Workflow.Inputs, payload.Input)

	// Cron executions are created at dispatch time; manual and one-off scheduled executions are
	// created by core before their jobs are queued.
	if payload.TriggerType == TriggerTypeScheduled && job.ScheduledAt.IsZero() {
		if _, err := h.store.GetExecutionProjection(ctx, job.ExecID); errors.Is(err, sql.ErrNoRows) {
			if err := h.createExecution(ctx, job.ExecID, payload); err != nil {
				return fmt.Errorf("failed to create execution: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to check scheduled execution: %w", err)
		}
	}

	recorder, err := coreexecstate.NewRecorder(ctx, h.store, job.ExecID)
	if errors.Is(err, coreexecstate.ErrStaleJob) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not begin execution attempt: %w", err)
	}
	if err := recorder.Start(ctx); err != nil {
		return fmt.Errorf("could not record execution start: %w", err)
	}

	if h.metrics != nil {
		h.metrics.IncExecutionsRunning(payload.NamespaceID, payload.Workflow.Meta.ID)
	}

	// Execute the flow
	execErr := h.executeFlow(ctx, job.ExecID, payload, recorder)
	finishCtx := ctx
	if errors.Is(execErr, ErrExecutionCancelled) || errors.Is(execErr, context.Canceled) {
		finishCtx = context.WithoutCancel(ctx)
	}
	if err := recorder.Finish(finishCtx, execErr); err != nil {
		if errors.Is(err, coreexecstate.ErrSuperseded) {
			return nil
		}
		return fmt.Errorf("could not record execution result: %w", err)
	}
	if execErr != nil {
		h.logger.Error("error executing flow", "flow", payload.Workflow.Meta.ID, "error", execErr, "attempt", job.Attempt, "maxRetries", job.MaxRetries)
		if errors.Is(execErr, ErrPendingApproval) {
			h.recordMetricsAndNotifications(ctx, job.ExecID, repo.ExecutionStatusPendingApproval, payload, nil)
			return h.requeueIfApprovalDecided(ctx, job, payload, recorder.State())
		}
		if errors.Is(execErr, ErrExecutionCancelled) {
			h.recordMetricsAndNotifications(context.Background(), job.ExecID, repo.ExecutionStatusCancelled, payload, nil)
			return nil
		}
		h.recordMetricsAndNotifications(ctx, job.ExecID, repo.ExecutionStatusErrored, payload, execErr)
		if errors.Is(execErr, ErrApprovalRejected) {
			return nil
		}
		return execErr
	}

	if h.metrics != nil {
		h.metrics.DecExecutionsRunning(payload.NamespaceID, payload.Workflow.Meta.ID)
	}

	h.recordMetricsAndNotifications(ctx, job.ExecID, repo.ExecutionStatusCompleted, payload, nil)
	return nil
}

// executeFlow executes a flow
func (h *FlowExecutionHandler) executeFlow(ctx context.Context, execID string, payload FlowExecutionPayload, recorder *coreexecstate.Recorder) error {
	// Create temporary directory for artifacts shared across all actions in this flow
	artifactDir := filepath.Join(os.TempDir(), fmt.Sprintf("artifacts-store-%s", execID))
	if err := os.MkdirAll(artifactDir, 0700); err != nil {
		return fmt.Errorf("failed to create artifact directory: %w", err)
	}
	h.logger.Debug("artifact directory creation", "dir", artifactDir)

	// Copy files from flow directory to artifacts if flow directory is specified
	if payload.FlowDirectory != "" {
		if err := h.copyFlowFilesToArtifacts(payload.FlowDirectory, artifactDir); err != nil {
			return fmt.Errorf("failed to copy flow files to artifacts: %w", err)
		}
	}

	streamID := execID

	streamLogger, err := h.logmanager.NewLogger(streamID)
	if err != nil {
		return err
	}
	defer streamLogger.Close()

	// Get flow-specific secrets
	flowSecrets := h.getFlowSecrets(ctx, payload.Workflow.Meta.ID, payload.NamespaceID, execID)

	runCtx := flowRunContext{
		execID:        execID,
		workflowMeta:  payload.Workflow.Meta,
		variableMeta:  newFlowVariableMetadata(payload.Workflow.Meta),
		input:         payload.Input,
		streamLogger:  streamLogger,
		artifactDir:   artifactDir,
		artifactMu:    &sync.RWMutex{},
		secrets:       flowSecrets,
		outputs:       recorder.State().Outputs,
		namespaceID:   payload.NamespaceID,
		userUUID:      payload.UserUUID,
		overrideNodes: payload.OverrideNodes,
	}

	states := seedActionStates(payload.Workflow.Actions, recorder.State().Actions)

	if payload.Workflow.Meta.ExecutionMode == ExecutionModeDAG {
		return h.executeDAG(ctx, execID, payload, runCtx, states, recorder)
	}

	return h.executeSequential(ctx, payload, runCtx, recorder)
}

func (h *FlowExecutionHandler) executeSequential(ctx context.Context, payload FlowExecutionPayload, runCtx flowRunContext, recorder *coreexecstate.Recorder) error {
	for _, action := range payload.Workflow.Actions {
		if !recorder.Runnable(action.ID) {
			continue
		}
		if err := h.checkApproval(ctx, runCtx.execID, action, runCtx.namespaceID); err != nil {
			if recordErr := recorder.BlockAction(ctx, action.ID, err); recordErr != nil {
				return recordErr
			}
			return err
		}
		if err := recorder.StartAction(ctx, action.ID); err != nil {
			return err
		}
		runCtx.actionRetry = recorder.Attempt(action.ID)
		res, globals, err := h.executeSingleAction(ctx, action, runCtx)
		if err != nil {
			if recordErr := recorder.FinishAction(context.WithoutCancel(ctx), action.ID, nil, err); recordErr != nil {
				return recordErr
			}
			return err
		}

		h.logger.Debug("Action results", "results", res, "globals", globals)
		processActionResults(res, runCtx.outputs)
		mergeGlobals(globals, action.ID, runCtx.outputs)
		h.logger.Debug("outputs", "results", runCtx.outputs)

		if err := recorder.FinishAction(ctx, action.ID, cloneOutputs(runCtx.outputs), nil); err != nil {
			return err
		}
	}

	// Only remove the artifact store when all actions have been executed
	// This is to account for approval actions that could be run later
	os.RemoveAll(runCtx.artifactDir)
	return nil
}

type dagActionResult struct {
	action  Action
	res     map[string]string
	globals map[string]string
	err     error
}

// executeDAG runs actions as their dependencies complete, up to the instance-wide max_parallel at a
// time. On failure it stops dispatching but lets running actions finish, since killing a partially
// applied action is usually worse than letting it complete.
func (h *FlowExecutionHandler) executeDAG(ctx context.Context, execID string, payload FlowExecutionPayload, runCtx flowRunContext, states map[string]ActionState, recorder *coreexecstate.Recorder) error {
	graph, err := BuildGraph(payload.Workflow.Actions)
	if err != nil {
		return fmt.Errorf("invalid action dependencies: %w", err)
	}

	run := newDAGRun(graph, states, h.maxParallel)
	done := make(chan dagActionResult, len(payload.Workflow.Actions))

	for {
		for {
			action, ok := run.next()
			if !ok {
				break
			}
			if approvalErr := h.checkApproval(ctx, execID, action, payload.NamespaceID); approvalErr != nil {
				if err := recorder.BlockAction(ctx, action.ID, approvalErr); err != nil {
					run.record(action, err)
					continue
				}
				run.record(action, approvalErr)
				continue
			}
			if err := recorder.StartAction(ctx, action.ID); err != nil {
				run.record(action, err)
				continue
			}

			actionCtx := runCtx
			actionCtx.outputs = cloneOutputs(runCtx.outputs)
			actionCtx.actionRetry = recorder.Attempt(action.ID)
			go func(a Action, c flowRunContext) {
				res, globals, err := h.executeSingleAction(ctx, a, c)
				done <- dagActionResult{action: a, res: res, globals: globals, err: err}
			}(action, actionCtx)
		}
		if !run.active() {
			break
		}

		r := <-done
		err := r.err
		if err == nil {
			h.logger.Debug("Action results", "results", r.res, "globals", r.globals)
			processActionResults(r.res, runCtx.outputs)
			mergeGlobals(r.globals, r.action.ID, runCtx.outputs)
		}

		recordErr := recorder.FinishAction(context.WithoutCancel(ctx), r.action.ID, cloneOutputs(runCtx.outputs), err)
		run.record(r.action, err)
		if recordErr != nil {
			run.halt(recordErr)
		}
	}

	if err := run.finish(); err != nil {
		if !errors.Is(err, ErrPendingApproval) && !errors.Is(err, ErrExecutionCancelled) {
			if recordErr := recorder.SkipPending(ctx, graph.Pending(states)); recordErr != nil {
				h.logger.Error("could not record skipped actions", "execID", execID, "error", recordErr)
			}
		}
		return err
	}

	os.RemoveAll(runCtx.artifactDir)
	return nil
}

// cloneOutputs copies the outputs tree so an action can read it while the scheduler keeps merging
// results from other actions. Node and global buckets are nested maps, so the copy has to recurse.
func cloneOutputs(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		if inner, ok := v.(map[string]any); ok {
			dst[k] = cloneOutputs(inner)
			continue
		}
		dst[k] = v
	}
	return dst
}

// getFlowSecrets retrieves flow-specific secrets or returns an empty map if unavailable
func (h *FlowExecutionHandler) getFlowSecrets(ctx context.Context, flowID string, namespaceID string, execID string) map[string]string {
	if h.secretsProvider == nil {
		return make(map[string]string)
	}

	secrets, err := h.secretsProvider(ctx, flowID, namespaceID)
	if err != nil {
		h.logger.Error("failed to get flow secrets", "execID", execID, "error", err)
		return make(map[string]string)
	}

	return secrets
}

// copyFlowFilesToArtifacts copies top-level files from the flow directory to the artifacts directory
func (h *FlowExecutionHandler) copyFlowFilesToArtifacts(flowDir string, artifactDir string) error {
	localArtifactDir := filepath.Join(artifactDir, "local")
	if err := os.MkdirAll(localArtifactDir, 0755); err != nil {
		return fmt.Errorf("failed to create local artifact directory: %w", err)
	}

	entries, err := os.ReadDir(flowDir)
	if err != nil {
		return fmt.Errorf("failed to read flow directory: %w", err)
	}

	for _, entry := range entries {
		// Skip directories, only copy top-level files
		if entry.IsDir() {
			continue
		}

		srcPath := filepath.Join(flowDir, entry.Name())
		destPath := filepath.Join(localArtifactDir, entry.Name())

		srcFile, err := os.Open(srcPath)
		if err != nil {
			return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
		}
		defer srcFile.Close()

		destFile, err := os.Create(destPath)
		if err != nil {
			return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
		}
		defer destFile.Close()

		if _, err := io.Copy(destFile, srcFile); err != nil {
			return fmt.Errorf("failed to copy file %s to %s: %w", srcPath, destPath, err)
		}

		h.logger.Debug("copied flow file to artifacts", "src", srcPath, "dest", destPath)
	}

	return nil
}

// executeSingleAction executes a single action within a flow.
func (h *FlowExecutionHandler) executeSingleAction(ctx context.Context, action Action, runCtx flowRunContext) (map[string]string, map[string]string, error) {
	if ctx.Err() != nil {
		if err := runCtx.streamLogger.Checkpoint("", "", "execution cancelled", streamlogger.CancelledMessageType, 0); err != nil {
			h.logger.Error("failed to send cancellation message", "error", err)
		}
		return nil, nil, ErrExecutionCancelled
	}

	h.logger.Debug("action retry count", "action", action.ID, "retry", runCtx.actionRetry)

	res, globals, err := h.runAction(ctx, action, runCtx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			if streamErr := runCtx.streamLogger.Checkpoint(action.ID, "", "execution cancelled", streamlogger.CancelledMessageType, runCtx.actionRetry); streamErr != nil {
				h.logger.Error("failed to send cancelled message", "execID", runCtx.execID, "actionID", action.ID, "error", streamErr)
			}
			return nil, nil, ErrExecutionCancelled
		}
		runCtx.streamLogger.Checkpoint(action.ID, "", err.Error(), streamlogger.ErrMessageType, runCtx.actionRetry)
		return nil, nil, err
	}

	if err := runCtx.streamLogger.Checkpoint(action.ID, "", mergeCheckpointResult(res, globals, action.ID), streamlogger.ResultMessageType, runCtx.actionRetry); err != nil {
		return nil, nil, err
	}

	return res, globals, nil
}

func mergeCheckpointResult(outputs, globals map[string]string, actionID string) map[string]string {
	merged := make(map[string]string, len(outputs)+len(globals))
	for k, v := range outputs {
		merged[k] = v
	}
	for k, v := range globals {
		merged[fmt.Sprintf("%s.%s.%s", globalOutputKey, actionID, k)] = v
	}
	return merged
}

// processActionResults processes action results and updates the outputs map
func processActionResults(results map[string]string, outputs map[string]any) {
	for k, v := range results {
		parts := strings.SplitN(k, "@", 2)
		// node suffixed output
		if len(parts) == 2 {
			keyName := parts[0]
			nodeName := parts[1]

			nodeBucket, ok := outputs[nodeName].(map[string]any)
			if !ok {
				nodeBucket = make(map[string]any)
				outputs[nodeName] = nodeBucket
			}
			nodeBucket[keyName] = v
		} else {
			outputs[k] = v
		}
	}
}

// mergeGlobals writes an action's globals into outputs.global.<action_id>.
func mergeGlobals(globals map[string]string, actionID string, outputs map[string]any) {
	if len(globals) == 0 {
		return
	}

	globalBucket, ok := outputs[globalOutputKey].(map[string]any)
	if !ok {
		globalBucket = make(map[string]any)
		outputs[globalOutputKey] = globalBucket
	}

	actionBucket, ok := globalBucket[actionID].(map[string]any)
	if !ok {
		actionBucket = make(map[string]any)
		globalBucket[actionID] = actionBucket
	}

	for k, v := range globals {
		actionBucket[k] = v
	}
}

// executeOnNode executes an action on a single node and returns the results
func (h *FlowExecutionHandler) executeOnNode(ctx context.Context, node Node, action Action, inputVars map[string]any, withConfig []byte, allNodes []Node, runCtx flowRunContext) ExecResults {
	// Create a separate executor instance for each node
	var exec executor.Executor
	nodeExecutorID := fmt.Sprintf("%s-%s", action.ID, node.Name)
	if node.Name == "" {
		nodeExecutorID = action.ID
	}

	// Reset to local execution if the executor doesn't support remote execution,
	// or if the executor dispatches to nodes itself
	if caps, err := executor.GetCapabilities(action.Executor); err == nil &&
		(caps&executor.RemoteExecution == 0 || caps&executor.NodeDispatch != 0) {
		node = Node{}
	}

	nodeLogger := streamlogger.NewNodeContextLogger(runCtx.streamLogger, action.ID, node.Name, runCtx.actionRetry)

	if node.Name != "" {
		if err := node.CheckConnectivity(); err != nil {
			h.logger.Debug("node connectivity", "error", err)
			return ExecResults{
				result: nil,
				err:    fmt.Errorf("failed to connect to node %s", node.Name),
			}
		}
	}

	// Convert task node to executor node
	execNode := executor.Node{
		Hostname:       node.Hostname,
		Port:           node.Port,
		Username:       node.Username,
		ConnectionType: node.ConnectionType,
		OSFamily:       node.OSFamily,
		Auth: executor.NodeAuth{
			Method: string(node.Auth.Method),
			Key:    node.Auth.Key,
		},
	}

	ef, err := executor.GetNewExecutorFunc(action.Executor)
	if err != nil {
		return ExecResults{
			result: nil,
			err:    fmt.Errorf("failed to get executor for %s: %w", action.ID, err),
		}
	}
	exec, err = ef(nodeExecutorID, execNode, artifactScope(runCtx.execID, action.ID))
	if err != nil {
		return ExecResults{
			result: nil,
			err:    fmt.Errorf("failed to create executor for %s: %w", action.ID, err),
		}
	}
	defer exec.Close()

	// Separate driver for artifact management
	artifactDriver, err := executor.NewNodeDriver(ctx, execNode)
	if err != nil {
		return ExecResults{
			result: nil,
			err:    fmt.Errorf("failed to create artifact driver: %w", err),
		}
	}
	defer artifactDriver.Close()

	// Push existing artifacts to this node's executor before execution
	if err := h.pushArtifactsWithDriver(ctx, artifactDriver, runCtx, action.ID); err != nil {
		return ExecResults{
			result: nil,
			err:    fmt.Errorf("failed to push artifacts to node %s: %w", node.Name, err),
		}
	}

	// Transform file paths for remote execution
	execInputVars := h.transformPaths(inputVars, runCtx.artifactDir, exec)

	var apiKey string
	if key, ok := h.executorKeys[action.Executor]; ok {
		apiKey = key
	}

	var execNodes []executor.Node
	if caps, err := executor.GetCapabilities(action.Executor); err == nil && caps&executor.NodeDispatch != 0 {
		execNodes = make([]executor.Node, len(allNodes))
		for i, n := range allNodes {
			execNodes[i] = executor.Node{
				Hostname:       n.Hostname,
				Port:           n.Port,
				Username:       n.Username,
				ConnectionType: n.ConnectionType,
				OSFamily:       n.OSFamily,
				Auth: executor.NodeAuth{
					Method: string(n.Auth.Method),
					Key:    n.Auth.Key,
				},
			}
		}
	}

	res, err := exec.Execute(ctx, executor.ExecutionContext{
		Inputs:        execInputVars,
		WithConfig:    withConfig,
		Stdout:        nodeLogger,
		Stderr:        nodeLogger,
		UserUUID:      runCtx.userUUID,
		NamespaceName: runCtx.workflowMeta.Namespace,
		APIKey:        apiKey,
		APIBaseURL:    h.apiBaseURL,
		ExecID:        runCtx.execID,
		FlowID:        runCtx.workflowMeta.ID,
		FlowName:      runCtx.workflowMeta.Name,
		ActionID:      action.ID,
		ActionName:    action.Name,
		Nodes:         execNodes,
	})

	if err == nil {
		if pullErr := h.pullArtifactsWithDriver(ctx, artifactDriver, runCtx, action.ID, node.Name); pullErr != nil {
			err = fmt.Errorf("execution succeeded but failed to pull artifacts: %w", pullErr)
		}
	}

	return ExecResults{
		result:  prefixResultKeys(res.Outputs, node.Name),
		globals: sanitizeKeys(res.Globals),
		err:     err,
	}
}

var keySanitizer = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

func sanitizeKeys(results map[string]string) map[string]string {
	out := make(map[string]string, len(results))
	for k, v := range results {
		out[keySanitizer.ReplaceAllString(k, "_")] = v
	}
	return out
}

func prefixResultKeys(results map[string]string, nodeName string) map[string]string {
	prefixedRes := make(map[string]string, len(results))
	for key, value := range results {
		prefixedKey := keySanitizer.ReplaceAllString(key, "_")
		if nodeName != "" {
			prefixedKey = prefixedKey + "@" + nodeName
		}
		prefixedRes[prefixedKey] = value
	}
	return prefixedRes
}

// interpolateVariables processes action variables and replaces templated values with evaluated expressions
func (h *FlowExecutionHandler) interpolateVariables(action Action, runCtx flowRunContext) (map[string]any, error) {
	// pattern to extract interpolated variables
	pattern := `{{\s*([^}]+)\s*}}`
	re := regexp.MustCompile(pattern)

	h.logger.Debug("scheduler variables", "input", runCtx.input)

	inputVars := make(map[string]any)
	for _, variable := range action.Variables {
		matches := re.FindAllStringSubmatch(variable.Value(), -1)
		if len(matches) > 0 {
			// Interpolated variable, needs evaluation
			inputExpr := matches[0][1]
			env := map[string]any{
				"inputs":  runCtx.input,
				"secrets": runCtx.secrets,
				"outputs": runCtx.outputs,
				"meta":    runCtx.variableMeta,
			}

			program, err := expr.Compile(inputExpr, expr.Env(env))
			if err != nil {
				return nil, fmt.Errorf("failed to compile expression: %w", err)
			}

			output, err := expr.Run(program, env)
			if err != nil {
				return nil, fmt.Errorf("failed to run expression: %w", err)
			}

			inputVars[variable.Name()] = ""
			if output != nil {
				inputVars[variable.Name()] = output
			}
		} else {
			// Normal variable, no evaluation
			inputVars[variable.Name()] = variable.Value()
		}
	}

	return inputVars, nil
}

// runAction executes a single action
func (h *FlowExecutionHandler) runAction(ctx context.Context, action Action, runCtx flowRunContext) (map[string]string, map[string]string, error) {
	jobCtx, cancel := context.WithTimeout(ctx, h.executionTimeout)
	defer cancel()

	inputVars, err := h.interpolateVariables(action, runCtx)
	if err != nil {
		return nil, nil, err
	}

	withConfig, err := yaml.Marshal(action.With)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal 'with' config: %w", err)
	}

	caps, _ := executor.GetCapabilities(action.Executor)

	if action.AllowNodeOverride && caps&executor.RemoteExecution != 0 && len(runCtx.overrideNodes) > 0 {
		action.On = runCtx.overrideNodes
	}

	if len(action.On) == 0 {
		action.On = append(action.On, Node{})
	}

	h.logger.Debug("final nodes", "nodes", action.On)

	// Executors with NodeDispatch capability handle node fan-out themselves
	// so run them once locally and pass the selected nodes through ExecutionContext
	dispatchNodes := action.On
	if caps&executor.NodeDispatch != 0 {
		dispatchNodes = []Node{{}}
	}

	var wg sync.WaitGroup
	resChan := make(chan ExecResults, len(dispatchNodes))

	for _, node := range dispatchNodes {
		wg.Add(1)
		go func(node Node) {
			defer wg.Done()
			result := h.executeOnNode(jobCtx, node, action, inputVars, withConfig, action.On, runCtx)
			resChan <- result
		}(node)
	}

	wg.Wait()
	close(resChan)

	mergedResults := make(map[string]string)
	mergedGlobals := make(map[string]string)
	for res := range resChan {
		if res.err != nil {
			if errors.Is(res.err, context.Canceled) {
				return nil, nil, context.Canceled
			}
			return nil, nil, res.err
		}
		maps.Copy(mergedResults, res.result)
		maps.Copy(mergedGlobals, res.globals)
	}

	return mergedResults, mergedGlobals, nil
}

// transformPaths replaces local artifact paths with executor artifact paths in input variables.
// File input paths that reference the local artifact directory are converted to use the executor's artifact directory as the base path.
func (h *FlowExecutionHandler) transformPaths(inputVars map[string]any, localArtifactDir string, exec executor.Executor) map[string]any {
	execArtifactDir := exec.GetArtifactsDir()
	transformed := make(map[string]any, len(inputVars))

	for k, v := range inputVars {
		transformed[k] = v
		if strVal, ok := v.(string); ok && strings.HasPrefix(strVal, localArtifactDir) {
			relPath, err := filepath.Rel(localArtifactDir, strVal)
			if err == nil {
				transformed[k] = filepath.Join(execArtifactDir, relPath)
			}
		}
	}

	return transformed
}

// artifactScope names the per-action artifact staging directory so actions running concurrently on
// the same node cannot overwrite each other's files. Executors derive the same directory from the
// ID they are constructed with, so both sides must build it from here.
func artifactScope(execID, actionID string) string {
	return fmt.Sprintf("%s-%s", execID, actionID)
}

func remoteArtifactsPath(driver executor.NodeDriver, execID, actionID string) string {
	return driver.Join(driver.TempDir(), fmt.Sprintf("artifacts-%s", artifactScope(execID, actionID)))
}

// pushArtifactsWithDriver pushes files from the local artifact directory to the remote artifacts directory
// Only pushes direct child files of top-level directories (one level deep)
func (h *FlowExecutionHandler) pushArtifactsWithDriver(ctx context.Context, driver executor.NodeDriver, runCtx flowRunContext, actionID string) error {
	artifactDir := runCtx.artifactDir
	remoteArtifactsDir := remoteArtifactsPath(driver, runCtx.execID, actionID)
	h.logger.Debug("remote artifacts directory", "pushdir", remoteArtifactsDir)

	runCtx.artifactMu.RLock()
	defer runCtx.artifactMu.RUnlock()

	// Read top-level entries in artifact directory
	entries, err := os.ReadDir(artifactDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(artifactDir, entry.Name())
			h.logger.Debug("processing top-level directory", "pushdirentry", dirPath)

			childEntries, err := os.ReadDir(dirPath)
			if err != nil {
				return err
			}

			for _, child := range childEntries {
				if !child.IsDir() {
					info, _ := child.Info()
					h.logger.Debug("file size", "filesize", info.Size())
					localPath := filepath.Join(dirPath, child.Name())
					remotePath := driver.Join(remoteArtifactsDir, entry.Name(), child.Name())
					h.logger.Debug("pushing artifact file", "localPath", localPath, "remotePath", remotePath)
					if err := driver.Upload(ctx, localPath, remotePath); err != nil {
						return fmt.Errorf("failed to push artifact %s: %w", localPath, err)
					}
				}
			}
		}
	}

	return nil
}

// pullArtifactsWithDriver downloads all files from the remote artifacts directory to the local artifact directory
func (h *FlowExecutionHandler) pullArtifactsWithDriver(ctx context.Context, driver executor.NodeDriver, runCtx flowRunContext, actionID string, nodeName string) error {
	artifactDir := runCtx.artifactDir
	remoteArtifactsDir := remoteArtifactsPath(driver, runCtx.execID, actionID)
	h.logger.Debug("remote artifacts directory", "pulldir", remoteArtifactsDir)
	files, err := driver.ListFiles(ctx, remoteArtifactsDir)
	if err != nil {
		// If the directory doesn't exist, there are no artifacts to pull
		h.logger.Debug("no artifacts to pull", "remoteDir", remoteArtifactsDir, "error", err)
		return nil
	}

	runCtx.artifactMu.Lock()
	defer runCtx.artifactMu.Unlock()

	for _, file := range files {
		remotePath := driver.Join(remoteArtifactsDir, file)

		var localPath string
		if driver.IsRemote() {
			// Remote execution then store in nodeName subdirectory
			localPath = filepath.Join(artifactDir, nodeName, file)
		} else {
			// Local execution then store in local subdirectory
			localPath = filepath.Join(artifactDir, "local", file)
		}

		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for artifact %s: %w", file, err)
		}

		if err := driver.Download(ctx, remotePath, localPath); err != nil {
			return fmt.Errorf("failed to pull artifact %s from node %s: %w", file, nodeName, err)
		}
	}
	return nil
}

func (h *FlowExecutionHandler) checkApproval(ctx context.Context, execID string, action Action, namespaceID string) error {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	if !action.Approval {
		return nil
	}

	// check if pending approval, exit if not approved
	a, err := h.store.GetApprovalRequestForActionAndExec(ctx, repo.GetApprovalRequestForActionAndExecParams{
		ExecID:   execID,
		ActionID: action.ID,
		Uuid:     namespaceUUID,
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	// continue execution if approved
	if a.Status == repo.ApprovalStatusApproved {
		return nil
	}

	if a.Status == repo.ApprovalStatusRejected {
		return fmt.Errorf("%w: action %q", ErrApprovalRejected, action.Name)
	}

	if a.Status == "" {
		_, err = h.store.RequestApprovalTx(ctx, execID, namespaceUUID, repo.RequestApprovalParam{
			ID: action.ID,
		})
		if err != nil {
			return err
		}
	}

	return ErrPendingApproval
}

// requeueIfApprovalDecided requeues the execution if any action it blocked on was decided while the
// run was still in flight. Core cannot requeue a running execution, so an approval granted before
// the last sibling action drained would otherwise be lost.
func (h *FlowExecutionHandler) requeueIfApprovalDecided(ctx context.Context, job Job, payload FlowExecutionPayload, state coreexecstate.ExecutionState) error {
	namespaceUUID, err := uuid.Parse(payload.NamespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	decided := false
	for id, action := range state.Actions {
		if action.Status != coreexecstate.ActionStatusBlocked {
			continue
		}
		a, err := h.store.GetApprovalRequestForActionAndExec(ctx, repo.GetApprovalRequestForActionAndExecParams{
			ExecID:   job.ExecID,
			ActionID: id,
			Uuid:     namespaceUUID,
		})
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if a.Status != repo.ApprovalStatusPending {
			decided = true
			break
		}
	}
	if !decided {
		return nil
	}

	_, err = h.store.RequeueExecutionAndJobTx(ctx, repo.RequeueExecutionParams{
		ExecID: job.ExecID,
		Uuid:   namespaceUUID,
	}, repo.ExecutionJob{
		PayloadType: string(PayloadTypeFlowExecution),
		Payload:     job.Payload,
		CreatedAt:   time.Now(),
		MaxRetries:  int32(job.MaxRetries),
	})
	// Zero rows means core won the race and has already queued the resume.
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

// createExecution creates an execution for cron jobs. Manual and one-off scheduled executions are
// created in core.
func (h *FlowExecutionHandler) createExecution(ctx context.Context, execID string, payload FlowExecutionPayload) error {
	namespaceUUID, err := uuid.Parse(payload.NamespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	userUUID, err := uuid.Parse(payload.UserUUID)
	if err != nil {
		return fmt.Errorf("invalid user UUID: %w", err)
	}

	inputsJSON, err := json.Marshal(payload.Input)
	if err != nil {
		return fmt.Errorf("failed to marshal execution context: %w", err)
	}

	triggerType := repo.TriggerTypeManual
	if payload.TriggerType == TriggerTypeScheduled {
		triggerType = repo.TriggerTypeScheduled
	}

	_, err = h.store.AddExecutionTx(ctx, repo.AddExecutionParams{
		ExecID:      execID,
		FlowID:      payload.Workflow.Meta.DBID,
		Inputs:      inputsJSON,
		TriggerType: triggerType,
		Uuid:        userUUID,
		Uuid_2:      namespaceUUID,
	}, payload.Outputs)
	if err != nil {
		return fmt.Errorf("failed to add execution: %w", err)
	}
	return nil
}

func (h *FlowExecutionHandler) recordMetricsAndNotifications(ctx context.Context, execID string, status repo.ExecutionStatus, payload FlowExecutionPayload, execErr error) {
	flowID := payload.Workflow.Meta.ID
	namespaceID := payload.NamespaceID

	if h.metrics != nil {
		switch status {
		case repo.ExecutionStatusCompleted:
			h.metrics.IncrementExecutionCount(namespaceID, flowID, "completed")
		case repo.ExecutionStatusErrored:
			h.metrics.IncrementExecutionCount(namespaceID, flowID, "errored")
		case repo.ExecutionStatusCancelled:
			h.metrics.IncrementExecutionCount(namespaceID, flowID, "cancelled")
		case repo.ExecutionStatusPendingApproval:
			h.metrics.IncExecutionsWaiting(namespaceID, flowID)
		}
	}

	// Enqueue notifications if configured
	h.logger.Debug("notification event", "status", status)
	h.enqueueNotifications(ctx, execID, status, payload, execErr)
}

// enqueueNotifications queues notification jobs for matching notify configurations
func (h *FlowExecutionHandler) enqueueNotifications(ctx context.Context, execID string, status repo.ExecutionStatus, payload FlowExecutionPayload, execErr error) {
	if h.taskQueuer == nil || len(payload.Workflow.Notify) == 0 {
		return
	}

	// Map execution status to notify event
	var event NotifyEvent
	switch status {
	case repo.ExecutionStatusCompleted:
		event = NotifyEventOnSuccess
	case repo.ExecutionStatusErrored:
		event = NotifyEventOnFailure
	case repo.ExecutionStatusCancelled:
		event = NotifyEventOnCancelled
	case repo.ExecutionStatusPendingApproval:
		event = NotifyEventOnWaiting
	default:
		return
	}

	h.logger.Debug("notification event", "event", event, "status", status)

	// Find matching notify configurations
	for _, notify := range payload.Workflow.Notify {
		if !slices.Contains(notify.Events, event) {
			continue
		}

		var errMsg string
		if execErr != nil {
			errMsg = execErr.Error()
		}

		notifyPayload := NotificationPayload{
			FlowID:      payload.Workflow.Meta.ID,
			FlowName:    payload.Workflow.Meta.Name,
			ExecID:      execID,
			Status:      string(status),
			Error:       errMsg,
			Config:      notify.Config,
			NamespaceID: payload.NamespaceID,
			Channel:     notify.Channel,
		}

		// Generate a unique exec ID for the notification job
		notifyExecID := fmt.Sprintf("notify-%s-%s", execID, notify.Channel)

		if _, err := h.taskQueuer.QueueTaskWithRetries(ctx, PayloadTypeNotification, notifyExecID, notifyPayload, 3); err != nil {
			h.logger.Error("failed to queue notification", "execID", execID, "channel", notify.Channel, "error", err)
		} else {
			h.logger.Debug("notification queued", "execID", execID, "channel", notify.Channel, "event", event)
		}
	}
}

// applyDefaultInputs sets default values from the flow's input definitions
// for any inputs that are missing or empty.
func applyDefaultInputs(definitions []Input, inputs map[string]any) {
	for _, inp := range definitions {
		if inp.Default == "" {
			continue
		}
		v, exists := inputs[inp.Name]
		if !exists || v == "" || v == nil {
			inputs[inp.Name] = inp.Default
		}
	}
}

func seedActionStates(actions []Action, folded map[string]ActionState) map[string]ActionState {
	states := make(map[string]ActionState, len(actions))
	for _, action := range actions {
		state := folded[action.ID]
		if state.Status == ActionStatusCompleted {
			states[action.ID] = state
		} else {
			states[action.ID] = ActionState{Status: ActionStatusPending, Attempt: state.Attempt}
		}
	}
	return states
}
