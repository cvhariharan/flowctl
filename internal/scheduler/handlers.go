package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/cvhariharan/flowctl/internal/streamlogger"
	"github.com/cvhariharan/flowctl/sdk/executor"
	"github.com/expr-lang/expr"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

var (
	ErrExecutionCancelled = errors.New("execution cancelled")
	RemoteArtifactsPath   = os.TempDir()
)

// executeFlow executes a flow - adapted from FlowRunner.HandleFlowExecution
func (s *Scheduler) executeFlow(ctx context.Context, payload FlowExecutionPayload) error {
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
	s.logger.Debug("artifact directory creation", "dir", artifactDir)
	defer os.RemoveAll(artifactDir)

	streamID := payload.ExecID

	streamLogger, err := s.logmanager.NewLogger(streamID)
	if err != nil {
		return err
	}
	defer streamLogger.Close()

	// Get flow-specific secrets
	var flowSecrets map[string]string
	if s.secretsProvider != nil {
		var err error
		flowSecrets, err = s.secretsProvider(ctx, payload.Workflow.Meta.ID, payload.NamespaceID)
		if err != nil {
			s.logger.Error("failed to get flow secrets", "execID", payload.ExecID, "error", err)
			flowSecrets = make(map[string]string)
		}
	} else {
		flowSecrets = make(map[string]string)
	}

	// Initialize outputs map to accumulate results from all previous actions
	outputs := make(map[string]interface{})

	for i := payload.StartingActionIdx; i < len(payload.Workflow.Actions); i++ {
		if ctx.Err() != nil {
			// Send a cancelled message before returning
			if err := streamLogger.Checkpoint("", "", "execution cancelled", streamlogger.CancelledMessageType); err != nil {
				s.logger.Error("failed to send cancellation message", "error", err)
			}
			return ErrExecutionCancelled
		}
		action := payload.Workflow.Actions[i]

		// Check for approval requests
		if err := s.checkApproval(ctx, payload.ExecID, action, payload.NamespaceID); err != nil {
			return err
		}

		res, err := s.runAction(ctx, action, payload.Workflow.Meta.SrcDir, payload.Input, streamLogger, artifactDir, flowSecrets, outputs)
		if err != nil {
			// Check if the error is due to context cancellation
			if errors.Is(err, context.Canceled) {
				// Send a cancelled message before returning
				if streamErr := streamLogger.Checkpoint(action.ID, "", "execution cancelled", streamlogger.CancelledMessageType); streamErr != nil {
					s.logger.Error("failed to send cancelled message", "execID", payload.ExecID, "actionID", action.ID, "error", streamErr)
				}
				return ErrExecutionCancelled
			}
			streamLogger.Checkpoint(action.ID, "", err.Error(), streamlogger.ErrMessageType)
			return err
		}
		if err := streamLogger.Checkpoint(action.ID, "", res, streamlogger.ResultMessageType); err != nil {
			return err
		}
		s.logger.Debug("Action results", "results", res)

		for k, v := range res {
			parts := strings.SplitN(k, "@", 2)
			// node suffixed output
			if len(parts) == 2 {
				keyName := parts[0]
				nodeName := parts[1]

				if _, exists := outputs[nodeName]; !exists {
					outputs[nodeName] = make(map[string]interface{})
				}
				outputs[nodeName].(map[string]interface{})[keyName] = v
			} else {
				outputs[k] = v
			}
		}
		s.logger.Debug("outputs", "results", outputs)
	}

	return nil
}

// runAction executes a single action - adapted from FlowRunner.runAction
func (s *Scheduler) runAction(ctx context.Context, action Action, srcdir string, input map[string]interface{}, streamLogger streamlogger.Logger, artifactDir string, secrets map[string]string, outputs map[string]interface{}) (map[string]string, error) {
	streamLogger.SetActionID(action.ID)

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
				"inputs":  input,
				"secrets": secrets,
				"outputs": outputs,
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
			nodeLogger := streamlogger.NewNodeContextLogger(streamLogger, action.ID, node.Name)
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

			// Check if node is accessible
			// Ignore local node
			if node.Name != "" {
				if err := execNode.CheckConnectivity(); err != nil {
					resChan <- ExecResults{
						result: nil,
						err:    fmt.Errorf("failed to connect to node %s", node.Name),
					}
					return
				}
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
			if err := s.pushArtifacts(jobCtx, exec, artifactDir); err != nil {
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
				Stdout:     nodeLogger,
				Stderr:     nodeLogger,
			})

			// Pull artifacts from this node after successful execution
			if err == nil && len(action.Artifacts) > 0 {
				if pullErr := s.pullArtifacts(jobCtx, exec, artifactDir, action.Artifacts, node.Name); pullErr != nil {
					err = fmt.Errorf("execution succeeded but failed to pull artifacts: %w", pullErr)
				}
			}

			// Add node.Name suffix to result keys
			prefixedRes := make(map[string]string)
			for key, value := range res {
				// Format key as valid environment variable (replace special chars with _)
				prefixedKey := regexp.MustCompile(`[^a-zA-Z0-9_]+`).ReplaceAllString(key, "_")
				if node.Name != "" {
					// example key@hostname
					prefixedKey = prefixedKey + "@" + node.Name
				}
				prefixedRes[prefixedKey] = value
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

// pushArtifacts pushes existing artifact files from the artifact directory to the executor
func (s *Scheduler) pushArtifacts(ctx context.Context, exec executor.Executor, artifactDir string) error {
	return filepath.WalkDir(artifactDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			rPath, err := filepath.Rel(artifactDir, path)
			if err != nil {
				return err
			}
			remotePath := filepath.Join(RemoteArtifactsPath, rPath)
			if err := exec.PushFile(ctx, path, remotePath); err != nil {
				return fmt.Errorf("failed to push artifact %s: %w", path, err)
			}
		}
		return nil
	})
}

// pullArtifacts pulls specified artifact files from the executor to the artifact directory
// If nodeName is provided, artifacts are placed in a subdirectory named after the node
func (s *Scheduler) pullArtifacts(ctx context.Context, exec executor.Executor, artifactDir string, artifacts []string, nodeName string) error {
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

func (s *Scheduler) checkApproval(ctx context.Context, execID string, action Action, namespaceID string) error {
	// use parent exec ID if available for approval requests
	eID := execID

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	// Set the current action ID
	log.Println("current action ID: ", action.ID)
	if _, err := s.store.UpdateExecutionActionID(ctx, repo.UpdateExecutionActionIDParams{
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
	a, err := s.store.GetApprovalRequestForActionAndExec(ctx, repo.GetApprovalRequestForActionAndExecParams{
		ExecID:   eID,
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
		_, err = s.store.RequestApprovalTx(ctx, eID, namespaceUUID, repo.RequestApprovalParam{
			ID: action.ID,
		})
		if err != nil {
			return err
		}
	}

	return ErrPendingApproval
}
