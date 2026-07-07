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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cvhariharan/flowctl/internal/metrics"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/cvhariharan/flowctl/internal/storage"
	"github.com/cvhariharan/flowctl/internal/streamlogger"
	"github.com/cvhariharan/flowctl/sdk/executor"
	"github.com/expr-lang/expr"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"gopkg.in/yaml.v3"
)

const PayloadTypeFlowExecution PayloadType = "flow_execution"

// FlowExecutionHandler handles flow execution jobs
type FlowExecutionHandler struct {
	store            repo.Store
	secretsProvider  SecretsProviderFn
	logmanager       streamlogger.LogManager
	logger           *slog.Logger
	executionTimeout time.Duration
	metrics          *metrics.Manager
	taskQueuer       TaskQueuer
	executorKeys     map[string]string // executor_name → API token
	apiBaseURL       string
	storage          storage.Storage
}

type flowRunContext struct {
	execID       string
	workflowMeta Metadata
	variableMeta flowVariableMetadata
	input        map[string]any
	streamLogger streamlogger.Logger
	artifactDir  string
	secrets      map[string]string
	outputs      map[string]any
	namespaceID  string
	userUUID     string
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
	ExecutorKeys         map[string]string // executor_name → API token
	APIBaseURL           string
	Storage              storage.Storage
}

// NewFlowExecutionHandler creates a new flow execution handler
func NewFlowExecutionHandler(cfg FlowHandlerConfig) *FlowExecutionHandler {
	if cfg.FlowExecutionTimeout == 0 {
		cfg.FlowExecutionTimeout = time.Hour
	}

	return &FlowExecutionHandler{
		store:            cfg.Store,
		secretsProvider:  cfg.SecretsProvider,
		logmanager:       cfg.LogManager,
		logger:           cfg.Logger,
		metrics:          cfg.Metrics,
		executionTimeout: cfg.FlowExecutionTimeout,
		executorKeys:     cfg.ExecutorKeys,
		apiBaseURL:       cfg.APIBaseURL,
		storage:          cfg.Storage,
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

	if job.Attempt > 0 {
		payload.Resumed = true
	}

	// Create execution log for scheduled executions or for retried jobs
	if job.Attempt > 0 || (payload.TriggerType == TriggerTypeScheduled && job.ScheduledAt.IsZero()) {
		if err := h.createExecutionLog(ctx, job.ExecID, payload); err != nil {
			return fmt.Errorf("failed to create execution log: %w", err)
		}
	}

	// Set status to Running
	if err := h.setStatus(ctx, job.ExecID, repo.ExecutionStatusRunning, payload.NamespaceID, nil); err != nil {
		return fmt.Errorf("could not update execution_log status: %w", err)
	}

	// Set started_at timestamp
	if err := h.setStartedAt(ctx, job.ExecID, payload.NamespaceID); err != nil {
		h.logger.Warn("failed to set started_at", "execID", job.ExecID, "error", err)
	}

	if h.metrics != nil {
		h.metrics.IncExecutionsRunning(payload.NamespaceID, payload.Workflow.Meta.ID)
	}

	// Execute the flow
	if err := h.executeFlow(ctx, job.ExecID, payload); err != nil {
		h.logger.Error("error executing flow", "flow", payload.Workflow.Meta.ID, "error", err, "attempt", job.Attempt, "maxRetries", job.MaxRetries)
		if errors.Is(err, ErrPendingApproval) {
			return h.setStatusWithMetrics(ctx, job.ExecID, repo.ExecutionStatusPendingApproval, payload, nil)
		}
		if errors.Is(err, ErrExecutionCancelled) {
			// If execution is cancelled, the context will also be cancelled, so use background context
			return h.setStatusWithMetrics(context.Background(), job.ExecID, repo.ExecutionStatusCancelled, payload, nil)
		}

		if err := h.setStatusWithMetrics(ctx, job.ExecID, repo.ExecutionStatusErrored, payload, err); err != nil {
			return err
		}

		return err
	}

	if h.metrics != nil {
		h.metrics.DecExecutionsRunning(payload.NamespaceID, payload.Workflow.Meta.ID)
	}

	return h.setStatusWithMetrics(ctx, job.ExecID, repo.ExecutionStatusCompleted, payload, nil)
}

// executeFlow executes a flow
func (h *FlowExecutionHandler) executeFlow(ctx context.Context, execID string, payload FlowExecutionPayload) error {
	if payload.StartingActionIdx < 0 {
		payload.StartingActionIdx = 0
	}
	if payload.StartingActionIdx > len(payload.Workflow.Actions) {
		payload.StartingActionIdx = len(payload.Workflow.Actions)
	}

	// Apply default input values for any inputs not provided by the caller
	if payload.Input == nil {
		payload.Input = make(map[string]any)
	}
	applyDefaultInputs(payload.Workflow.Inputs, payload.Input)

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

	// Persist uploaded files if storage is configured
	if h.storage != nil {
		uploadsDir := filepath.Join(artifactDir, "uploads")
		entries, err := os.ReadDir(uploadsDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					path := filepath.Join(uploadsDir, entry.Name())
					file, err := os.Open(path)
					if err == nil {
						// path format: <flow_slug>/<exec_id>/uploads/<filename>
						storageKey := fmt.Sprintf("%s/%s/uploads/%s", payload.Workflow.Meta.ID, execID, entry.Name())
						h.logger.Debug("persisting uploaded input file", "key", storageKey)
						if err := h.storage.Put(ctx, storageKey, file); err != nil {
							h.logger.Warn("failed to persist uploaded input file", "key", storageKey, "error", err)
						}
						file.Close()
					}
				}
			}
		}
	}

	streamID := execID

	streamLogger, err := h.logmanager.NewLogger(streamID)
	if err != nil {
		return err
	}
	defer streamLogger.Close()

	// Initialize action_retries for all actions in the flow for new executions only
	if !payload.Resumed {
		if err := h.initializeActionRetries(ctx, execID, payload.Workflow.Actions, payload.NamespaceID); err != nil {
			h.logger.Warn("failed to initialize action retries", "error", err)
		}
	}

	// Get flow-specific secrets
	flowSecrets := h.getFlowSecrets(ctx, payload.Workflow.Meta.ID, payload.NamespaceID, execID)

	// Initialize outputs map to accumulate results from all previous actions
	runCtx := flowRunContext{
		execID:       execID,
		workflowMeta: payload.Workflow.Meta,
		variableMeta: newFlowVariableMetadata(payload.Workflow.Meta),
		input:        payload.Input,
		streamLogger: streamLogger,
		artifactDir:  artifactDir,
		secrets:      flowSecrets,
		outputs:      make(map[string]any),
		namespaceID:  payload.NamespaceID,
		userUUID:     payload.UserUUID,
	}

	for i := payload.StartingActionIdx; i < len(payload.Workflow.Actions); i++ {
		action := payload.Workflow.Actions[i]

		res, err := h.executeSingleAction(ctx, action, runCtx)
		if err != nil {
			return err
		}

		h.logger.Debug("Action results", "results", res)
		processActionResults(res, runCtx.outputs)
		h.logger.Debug("outputs", "results", runCtx.outputs)

		// Persist artifacts for this action
		if h.storage != nil {
			if err := h.persistActionArtifacts(ctx, payload.Workflow.Meta.ID, execID, action.ID, artifactDir); err != nil {
				h.logger.Warn("failed to persist action artifacts", "actionID", action.ID, "error", err)
			}
		}
	}

	// Only remove the artifact store when all actions have been executed
	// This is to account for approval actions that could be run later
	os.RemoveAll(artifactDir)
	return nil
}

// initializeActionRetries initializes the action_retries map with all actions set to 0
func (h *FlowExecutionHandler) initializeActionRetries(ctx context.Context, execID string, actions []Action, namespaceID string) error {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	// Build map of all actions initialized to 0
	actionRetries := make(map[string]int32)
	for _, action := range actions {
		actionRetries[action.ID] = 0
	}

	retriesJSON, err := json.Marshal(actionRetries)
	if err != nil {
		return fmt.Errorf("failed to marshal action retries: %w", err)
	}

	if err := h.store.UpdateExecutionActionRetries(ctx, repo.UpdateExecutionActionRetriesParams{
		ExecID: execID,
		ActionRetries: pqtype.NullRawMessage{
			RawMessage: retriesJSON,
			Valid:      true,
		},
		Uuid: namespaceUUID,
	}); err != nil {
		return fmt.Errorf("failed to initialize action retries: %w", err)
	}

	h.logger.Debug("initialized action retries", "execID", execID, "actions", len(actions))
	return nil
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

// executeSingleAction executes a single action within a flow, handling approval and error checkpointing
func (h *FlowExecutionHandler) executeSingleAction(ctx context.Context, action Action, runCtx flowRunContext) (map[string]string, error) {
	// Check for context cancellation
	if ctx.Err() != nil {
		if err := runCtx.streamLogger.Checkpoint("", "", "execution cancelled", streamlogger.CancelledMessageType); err != nil {
			h.logger.Error("failed to send cancellation message", "error", err)
		}
		return nil, ErrExecutionCancelled
	}

	// Check for approval requests
	if err := h.checkApproval(ctx, runCtx.execID, action, runCtx.namespaceID); err != nil {
		return nil, err
	}

	// Increment retry count for this action
	namespaceUUID, err := uuid.Parse(runCtx.namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	row, err := h.store.IncrementActionRetry(ctx, repo.IncrementActionRetryParams{
		ExecID:  runCtx.execID,
		Column2: action.ID,
		Uuid:    namespaceUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to increment retry count for action %s: %w", action.ID, err)
	}

	runCtx.streamLogger.SetRetry(row.RetryCount)
	h.logger.Debug("action retry count", "action", action.ID, "retry", row.RetryCount)

	// Run the action
	res, err := h.runAction(ctx, action, runCtx)
	if err != nil {
		// Check if the error is due to context cancellation
		if errors.Is(err, context.Canceled) {
			if streamErr := runCtx.streamLogger.Checkpoint(action.ID, "", "execution cancelled", streamlogger.CancelledMessageType); streamErr != nil {
				h.logger.Error("failed to send cancelled message", "execID", runCtx.execID, "actionID", action.ID, "error", streamErr)
			}
			return nil, ErrExecutionCancelled
		}
		runCtx.streamLogger.Checkpoint(action.ID, "", err.Error(), streamlogger.ErrMessageType)
		return nil, err
	}

	// Checkpoint successful result
	if err := runCtx.streamLogger.Checkpoint(action.ID, "", res, streamlogger.ResultMessageType); err != nil {
		return nil, err
	}

	return res, nil
}

// processActionResults processes action results and updates the outputs map
func processActionResults(results map[string]string, outputs map[string]any) {
	for k, v := range results {
		parts := strings.SplitN(k, "@", 2)
		// node suffixed output
		if len(parts) == 2 {
			keyName := parts[0]
			nodeName := parts[1]

			if _, exists := outputs[nodeName]; !exists {
				outputs[nodeName] = make(map[string]any)
			}
			outputs[nodeName].(map[string]any)[keyName] = v
		} else {
			outputs[k] = v
		}
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

	nodeLogger := streamlogger.NewNodeContextLogger(runCtx.streamLogger, action.ID, node.Name)

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
	exec, err = ef(nodeExecutorID, execNode, runCtx.execID)
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
	if err := h.pushArtifactsWithDriver(ctx, artifactDriver, runCtx.artifactDir, runCtx.execID); err != nil {
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

	// Pull all artifacts from this node after execution
	if err == nil {
		if pullErr := h.pullArtifactsWithDriver(ctx, artifactDriver, runCtx.artifactDir, runCtx.execID, node.Name); pullErr != nil {
			err = fmt.Errorf("execution succeeded but failed to pull artifacts: %w", pullErr)
		}
	}

	// Add node.Name suffix to result keys
	prefixedRes := prefixResultKeys(res, node.Name)

	return ExecResults{
		result: prefixedRes,
		err:    err,
	}
}

// prefixResultKeys adds node name suffix to result keys for node-specific outputs
func prefixResultKeys(results map[string]string, nodeName string) map[string]string {
	prefixedRes := make(map[string]string)
	for key, value := range results {
		// Format key as valid environment variable (replace special chars with _)
		prefixedKey := regexp.MustCompile(`[^a-zA-Z0-9_]+`).ReplaceAllString(key, "_")
		if nodeName != "" {
			// example key@hostname
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
func (h *FlowExecutionHandler) runAction(ctx context.Context, action Action, runCtx flowRunContext) (map[string]string, error) {
	runCtx.streamLogger.SetActionID(action.ID)

	jobCtx, cancel := context.WithTimeout(ctx, h.executionTimeout)
	defer cancel()

	// Interpolate variables
	inputVars, err := h.interpolateVariables(action, runCtx)
	if err != nil {
		return nil, err
	}

	withConfig, err := yaml.Marshal(action.With)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal 'with' config: %w", err)
	}

	if len(action.On) == 0 {
		action.On = append(action.On, Node{})
	}

	// Executors with NodeDispatch capability handle node fan-out themselves
	// so run them once locally and pass the selected nodes through ExecutionContext
	dispatchNodes := action.On
	if caps, err := executor.GetCapabilities(action.Executor); err == nil && caps&executor.NodeDispatch != 0 {
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
		maps.Copy(mergedResults, res.result)
	}

	return mergedResults, nil
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

// pushArtifactsWithDriver pushes files from the local artifact directory to the remote artifacts directory
// Only pushes direct child files of top-level directories (one level deep)
func (h *FlowExecutionHandler) pushArtifactsWithDriver(ctx context.Context, driver executor.NodeDriver, artifactDir string, execID string) error {
	remoteArtifactsDir := driver.Join(driver.TempDir(), fmt.Sprintf("artifacts-%s", execID))
	h.logger.Debug("remote artifacts directory", "pushdir", remoteArtifactsDir)

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
func (h *FlowExecutionHandler) pullArtifactsWithDriver(ctx context.Context, driver executor.NodeDriver, artifactDir string, execID string, nodeName string) error {
	remoteArtifactsDir := driver.Join(driver.TempDir(), fmt.Sprintf("artifacts-%s", execID))
	h.logger.Debug("remote artifacts directory", "pulldir", remoteArtifactsDir)
	files, err := driver.ListFiles(ctx, remoteArtifactsDir)
	if err != nil {
		// If the directory doesn't exist, there are no artifacts to pull
		h.logger.Debug("no artifacts to pull", "remoteDir", remoteArtifactsDir, "error", err)
		return nil
	}

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

	// Set the current action ID
	h.logger.Debug("current action", "actionID", action.ID)
	if _, err := h.store.UpdateExecutionActionID(ctx, repo.UpdateExecutionActionIDParams{
		CurrentActionID: sql.NullString{String: action.ID, Valid: action.ID != ""},
		ExecID:          execID,
		Uuid:            namespaceUUID,
	}); err != nil {
		return fmt.Errorf("could not update current action ID in exec %s: %w", execID, err)
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
		return fmt.Errorf("request for running action %q is rejected", action.Name)
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

// setStatus updates the execution status in the execution_log table
func (h *FlowExecutionHandler) setStatus(ctx context.Context, execID string, status repo.ExecutionStatus, namespaceID string, err error) error {
	var errMsg sql.NullString
	if err != nil {
		errMsg = sql.NullString{String: err.Error(), Valid: true}
	}
	namespaceUUID, parseErr := uuid.Parse(namespaceID)
	if parseErr != nil {
		return fmt.Errorf("invalid namespace ID: %w", parseErr)
	}
	_, updateErr := h.store.UpdateExecutionStatus(ctx, repo.UpdateExecutionStatusParams{
		Status: status,
		Error:  errMsg,
		ExecID: execID,
		Uuid:   namespaceUUID,
	})
	if updateErr != nil {
		return fmt.Errorf("could not update error execution status: %w", updateErr)
	}

	return nil
}

// setStartedAt sets the started_at timestamp when execution begins running
func (h *FlowExecutionHandler) setStartedAt(ctx context.Context, execID string, namespaceID string) error {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace ID: %w", err)
	}
	return h.store.UpdateExecutionStartedAt(ctx, repo.UpdateExecutionStartedAtParams{
		ExecID: execID,
		Uuid:   namespaceUUID,
	})
}

// createExecutionLog creates an execution log entry for cron jobs
// scheduled (run later) and manual jobs have the execution logs created in core
func (h *FlowExecutionHandler) createExecutionLog(ctx context.Context, execID string, payload FlowExecutionPayload) error {
	namespaceUUID, err := uuid.Parse(payload.NamespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	userUUID, err := uuid.Parse(payload.UserUUID)
	if err != nil {
		return fmt.Errorf("invalid user UUID: %w", err)
	}

	inputJSON, err := json.Marshal(payload.Input)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}

	triggerType := repo.TriggerTypeManual
	if payload.TriggerType == TriggerTypeScheduled {
		triggerType = repo.TriggerTypeScheduled
	}

	_, err = h.store.AddExecutionLog(ctx, repo.AddExecutionLogParams{
		ExecID:      execID,
		FlowID:      payload.Workflow.Meta.DBID,
		Input:       inputJSON,
		TriggerType: triggerType,
		Uuid:        userUUID,
		Uuid_2:      namespaceUUID,
	})
	if err != nil {
		return fmt.Errorf("failed to add execution log: %w", err)
	}

	return nil
}

// setStatusWithMetrics updates the execution status and tracks metrics
func (h *FlowExecutionHandler) setStatusWithMetrics(ctx context.Context, execID string, status repo.ExecutionStatus, payload FlowExecutionPayload, execErr error) error {
	if err := h.setStatus(ctx, execID, status, payload.NamespaceID, execErr); err != nil {
		return err
	}

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

	return nil
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

func (h *FlowExecutionHandler) persistActionArtifacts(ctx context.Context, flowSlug string, execID string, actionID string, artifactDir string) error {
	if h.storage == nil {
		return nil
	}

	entries, err := os.ReadDir(artifactDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "uploads" {
			continue
		}

		nodeDir := filepath.Join(artifactDir, entry.Name())
		err := filepath.WalkDir(nodeDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				relPath, err := filepath.Rel(nodeDir, path)
				if err != nil {
					return err
				}

				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()

				// Path structure: <flow_slug>/<exec_id>/<action_id>/<filename>
				storageKey := fmt.Sprintf("%s/%s/%s/%s", flowSlug, execID, actionID, relPath)
				h.logger.Debug("persisting action artifact file", "key", storageKey, "localPath", path)
				if err := h.storage.Put(ctx, storageKey, file); err != nil {
					return fmt.Errorf("failed to persist artifact %s: %w", storageKey, err)
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

