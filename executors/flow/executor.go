package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	apiclient "github.com/cvhariharan/flowctl/pkg/client"
	"github.com/cvhariharan/flowctl/sdk/executor"
	"github.com/google/uuid"
	"github.com/invopop/jsonschema"
	"gopkg.in/yaml.v3"
)

type FlowWithConfig struct {
	FlowID string `yaml:"flow_id" json:"flow_id" jsonschema:"title=Flow ID,description=ID of the flow to execute,required" jsonschema_extras:"placeholder=my-flow-id"`
	Params string `yaml:"params,omitempty" json:"params,omitempty" jsonschema:"title=Params,description=JSON parameters to pass to the flow" jsonschema_extras:"widget=codeeditor"`
	Wait   bool   `yaml:"wait,omitempty" json:"wait,omitempty" jsonschema:"title=Wait for completion,description=Wait for the flow execution to complete" jsonschema_extras:"type=checkbox"`
}

const (
	statusCompleted = "completed"
	statusErrored   = "errored"
	statusCancelled = "cancelled"
)

type FlowExecutor struct {
	name   string
	execID string
}

func GetSchema() interface{} {
	return jsonschema.Reflect(&FlowWithConfig{})
}

func NewFlowExecutor(name string, node executor.Node, execID string) (executor.Executor, error) {
	return &FlowExecutor{name: name, execID: execID}, nil
}

func (j *FlowExecutor) GetArtifactsDir() string {
	return ""
}

func (j *FlowExecutor) Close() error {
	return nil
}

func GetCapabilities() executor.Capability {
	return executor.StreamingOutput
}

func (j *FlowExecutor) Execute(ctx context.Context, execCtx executor.ExecutionContext) (executor.ExecutionResult, error) {
	if execCtx.APIKey == "" || execCtx.APIBaseURL == "" {
		return executor.ExecutionResult{}, fmt.Errorf("flow executor %s requires API credentials (APIKey and APIBaseURL)", j.name)
	}

	var config FlowWithConfig
	if err := yaml.Unmarshal(execCtx.WithConfig, &config); err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("could not read config for Flow executor %s: %w", j.name, err)
	}

	if config.FlowID == "" {
		return executor.ExecutionResult{}, fmt.Errorf("flow_id is required for Flow executor %s", j.name)
	}

	params := make(map[string]interface{})
	if config.Params != "" {
		if err := json.Unmarshal([]byte(config.Params), &params); err != nil {
			return executor.ExecutionResult{}, fmt.Errorf("failed to parse params JSON: %w", err)
		}
	}

	client, err := newFlowAPIClient(execCtx.APIBaseURL, execCtx.APIKey, execCtx.UserUUID)
	if err != nil {
		return executor.ExecutionResult{}, err
	}

	triggerResp, err := triggerFlow(ctx, client, execCtx.NamespaceName, config.FlowID, params)
	if err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("failed to trigger flow %q: %w", config.FlowID, err)
	}

	execID := triggerResp.ExecID.String()
	fmt.Fprintf(execCtx.Stdout, "queued flow %s with exec_id %s\n", config.FlowID, execID)

	outputs := map[string]string{
		"exec_id": execID,
	}

	if config.Wait {
		status, err := j.waitForCompletion(ctx, client, execCtx.NamespaceName, execID)
		if err != nil {
			return executor.ExecutionResult{Outputs: outputs}, err
		}
		outputs["status"] = status

		if status == statusErrored {
			return executor.ExecutionResult{Outputs: outputs}, fmt.Errorf("child flow execution %s errored", triggerResp.ExecID)
		}
		if status == statusCancelled {
			return executor.ExecutionResult{Outputs: outputs}, fmt.Errorf("child flow execution %s was cancelled", triggerResp.ExecID)
		}

		fmt.Fprintf(execCtx.Stdout, "flow %s completed with status %s\n", config.FlowID, status)
	}

	return executor.ExecutionResult{Outputs: outputs}, nil
}

func newFlowAPIClient(baseURL, apiKey, userUUID string) (*apiclient.ClientWithResponses, error) {
	client, err := apiclient.NewClientWithResponses(strings.TrimRight(baseURL, "/"), apiclient.WithHTTPClient(&http.Client{
		Timeout: 30 * time.Second,
	}), apiclient.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		if userUUID != "" {
			req.Header.Set("X-User-UUID", userUUID)
		}
		return nil
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}
	return client, nil
}

func triggerFlow(ctx context.Context, client *apiclient.ClientWithResponses, namespace, flowID string, params map[string]any) (apiclient.FlowTriggerResp, error) {
	form := url.Values{}
	for k, v := range params {
		form.Set(k, fmt.Sprintf("%v", v))
	}

	resp, err := client.TriggerFlowWithBodyWithResponse(ctx, apiclient.NamespacePath(namespace), flowID, nil, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return apiclient.FlowTriggerResp{}, fmt.Errorf("trigger request failed: %w", err)
	}
	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		return apiclient.FlowTriggerResp{}, fmt.Errorf("trigger request failed (status %d): %s", resp.StatusCode(), string(resp.Body))
	}
	if resp.JSON200.ExecID == nil {
		return apiclient.FlowTriggerResp{}, fmt.Errorf("trigger response missing exec_id")
	}
	return *resp.JSON200, nil
}

func (j *FlowExecutor) waitForCompletion(ctx context.Context, client *apiclient.ClientWithResponses, namespace, execID string) (string, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	execUUID, err := uuid.Parse(execID)
	if err != nil {
		return "", fmt.Errorf("invalid execution ID %s: %w", execID, err)
	}

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled while waiting for execution %s: %w", execID, ctx.Err())
		case <-ticker.C:
			resp, err := client.GetExecutionWithResponse(ctx, apiclient.NamespacePath(namespace), execUUID)
			if err != nil {
				return "", fmt.Errorf("failed to get execution status for %s: %w", execID, err)
			}
			if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
				return "", fmt.Errorf("failed to get execution status for %s (status %d): %s", execID, resp.StatusCode(), string(resp.Body))
			}
			if resp.JSON200.Status == nil {
				return "", fmt.Errorf("execution status response missing status for %s", execID)
			}

			status := string(*resp.JSON200.Status)
			switch status {
			case statusCompleted, statusErrored, statusCancelled:
				return status, nil
			}
		}
	}
}

// FlowExecutorPlugin implements executor.ExecutorPlugin for the flow executor.
type FlowExecutorPlugin struct{}

func (p *FlowExecutorPlugin) GetName() string {
	return "flow"
}

func (p *FlowExecutorPlugin) GetSchema() interface{} {
	return GetSchema()
}

func (p *FlowExecutorPlugin) GetCapabilities() executor.Capability {
	return GetCapabilities()
}

func (p *FlowExecutorPlugin) New(name string, node executor.Node, execID string) (executor.Executor, error) {
	return NewFlowExecutor(name, node, execID)
}
