package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/handlers"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultWaitSeconds = 30
	maxWaitSeconds     = 120
	defaultTailLines   = 200
	maxTailLines       = 2000
	maxLogBytes        = 64 << 10
	approvalMessage    = "waiting for a human approval in flowctl; approvals cannot be decided over MCP"
)

var pollInterval = 2 * time.Second

type namespaceArgs struct {
	Namespace string `json:"namespace" jsonschema:"namespace name"`
}

type listFlowsArgs struct {
	Namespace string `json:"namespace" jsonschema:"namespace name"`
	Query     string `json:"query,omitempty" jsonschema:"filter flows by name"`
	Page      int    `json:"page,omitempty" jsonschema:"1-based page number"`
}

type flowArgs struct {
	Namespace string `json:"namespace" jsonschema:"namespace name"`
	FlowID    string `json:"flow_id" jsonschema:"flow slug"`
}

type triggerArgs struct {
	Namespace   string         `json:"namespace" jsonschema:"namespace name"`
	FlowID      string         `json:"flow_id" jsonschema:"flow slug"`
	Inputs      map[string]any `json:"inputs,omitempty" jsonschema:"input values keyed by input name; node inputs take an array of node names"`
	ScheduledAt string         `json:"scheduled_at,omitempty" jsonschema:"RFC3339 time in the future to run the flow at instead of now"`
}

type execArgs struct {
	Namespace string `json:"namespace" jsonschema:"namespace name"`
	ExecID    string `json:"exec_id" jsonschema:"execution ID"`
}

type listExecutionsArgs struct {
	Namespace string `json:"namespace" jsonschema:"namespace name"`
	FlowID    string `json:"flow_id,omitempty" jsonschema:"flow slug; omit to list executions across the namespace"`
	Page      int    `json:"page,omitempty" jsonschema:"1-based page number"`
}

type waitArgs struct {
	Namespace      string `json:"namespace" jsonschema:"namespace name"`
	ExecID         string `json:"exec_id" jsonschema:"execution ID"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" jsonschema:"how long to wait, default 30, max 120"`
}

type logsArgs struct {
	Namespace string `json:"namespace" jsonschema:"namespace name"`
	ExecID    string `json:"exec_id" jsonschema:"execution ID"`
	TailLines int    `json:"tail_lines,omitempty" jsonschema:"number of trailing log lines to return, default 200, max 2000"`
}

type retryArgs struct {
	Namespace  string `json:"namespace" jsonschema:"namespace name"`
	ExecID     string `json:"exec_id" jsonschema:"execution ID"`
	FromAction string `json:"from_action,omitempty" jsonschema:"action ID to rerun from; omit to retry from the failed action"`
}

func (s *server) registerRunTools() {
	addTool(s, &mcp.Tool{
		Name:        "list_namespaces",
		Description: "List the flowctl namespaces you are a member of. Every other tool takes a namespace name from this list.",
		Annotations: readOnly(),
	}, func(ctx context.Context, a *api, _ struct{}) (any, error) {
		body, err := a.get(ctx, "/api/v1/namespaces", nil)
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "list_flows",
		Description: "List the flows in a namespace that you can see. The `slug` field is the flow_id used by other tools.",
		Annotations: readOnly(),
	}, func(ctx context.Context, a *api, in listFlowsArgs) (any, error) {
		body, err := a.get(ctx, apiPath(in.Namespace, "flows"), pageQuery(in.Page, in.Query))
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "get_flow",
		Description: "Get a flow's metadata, actions and inputs. Call this before trigger_flow: `inputs` lists each input's name, type, whether it is required, and its options.",
		Annotations: readOnly(),
	}, func(ctx context.Context, a *api, in flowArgs) (any, error) {
		meta, err := a.get(ctx, apiPath(in.Namespace, "flows", in.FlowID, "meta"), nil)
		if err != nil {
			return nil, err
		}
		inputs, err := s.flowInputs(ctx, a, in.Namespace, in.FlowID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"meta": json.RawMessage(meta), "inputs": inputs.Inputs}, nil
	})

	addTool(s, &mcp.Tool{
		Name:        "trigger_flow",
		Description: "Run a flow. Call get_flow first to learn its inputs. Returns exec_id and execution_url; follow up with wait_for_execution. File inputs are not supported.",
	}, s.triggerFlow)

	addTool(s, &mcp.Tool{
		Name:        "get_execution",
		Description: "Get the current status and per-action state of an execution.",
		Annotations: readOnly(),
	}, func(ctx context.Context, a *api, in execArgs) (any, error) {
		body, sum, err := s.execution(ctx, a, in.Namespace, in.ExecID)
		if err != nil {
			return nil, err
		}
		return withFields(body, map[string]any{"execution_url": s.executionURL(in.Namespace, sum.FlowID, in.ExecID)})
	})

	addTool(s, &mcp.Tool{
		Name:        "list_executions",
		Description: "List recent executions of a flow, or of the whole namespace when flow_id is omitted.",
		Annotations: readOnly(),
	}, func(ctx context.Context, a *api, in listExecutionsArgs) (any, error) {
		path := apiPath(in.Namespace, "flows", "executions")
		if in.FlowID != "" {
			path = apiPath(in.Namespace, "flows", in.FlowID, "executions")
		}
		body, err := a.get(ctx, path, pageQuery(in.Page, ""))
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "wait_for_execution",
		Description: "Wait until an execution finishes, reaches an approval gate, or the timeout passes. If `done` is false, call it again. Approval gates must be decided by a human in the flowctl UI.",
		Annotations: readOnly(),
	}, s.waitForExecution)

	addTool(s, &mcp.Tool{
		Name:        "get_execution_logs",
		Description: "Get the last lines of a finished execution's logs. Only works once the execution is completed, errored or cancelled.",
		Annotations: readOnly(),
	}, s.executionLogs)

	addTool(s, &mcp.Tool{
		Name:        "cancel_execution",
		Description: "Cancel a pending or running execution.",
		Annotations: destructive(false),
	}, func(ctx context.Context, a *api, in execArgs) (any, error) {
		body, err := a.send(ctx, http.MethodPost, apiPath(in.Namespace, "flows", "executions", in.ExecID, "cancel"), nil)
		return result(body), err
	})

	addTool(s, &mcp.Tool{
		Name:        "retry_execution",
		Description: "Retry a failed or cancelled execution, optionally rerunning from a specific action.",
	}, func(ctx context.Context, a *api, in retryArgs) (any, error) {
		var req any
		if in.FromAction != "" {
			req = handlers.RetryExecutionReq{FromAction: in.FromAction}
		}
		body, err := a.send(ctx, http.MethodPost, apiPath(in.Namespace, "flows", "executions", in.ExecID, "retry"), req)
		return result(body), err
	})
}

func (s *server) flowInputs(ctx context.Context, a *api, namespace, flowID string) (handlers.FlowInputsResp, error) {
	var resp handlers.FlowInputsResp
	body, err := a.get(ctx, apiPath(namespace, "flows", flowID, "inputs"), nil)
	if err != nil {
		return resp, err
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return resp, fmt.Errorf("could not decode flow inputs: %w", err)
	}
	return resp, nil
}

func (s *server) execution(ctx context.Context, a *api, namespace, execID string) ([]byte, handlers.ExecutionSummary, error) {
	var sum handlers.ExecutionSummary
	body, err := a.get(ctx, apiPath(namespace, "flows", "executions", execID), nil)
	if err != nil {
		return nil, sum, err
	}
	if err := json.Unmarshal(body, &sum); err != nil {
		return nil, sum, fmt.Errorf("could not decode execution: %w", err)
	}
	return body, sum, nil
}

func (s *server) triggerFlow(ctx context.Context, a *api, in triggerArgs) (any, error) {
	defs, err := s.flowInputs(ctx, a, in.Namespace, in.FlowID)
	if err != nil {
		return nil, err
	}

	form, err := encodeInputs(defs.Inputs, in.Inputs)
	if err != nil {
		return nil, err
	}

	c := call{
		method:      http.MethodPost,
		path:        apiPath(in.Namespace, "trigger", in.FlowID),
		body:        strings.NewReader(form.Encode()),
		contentType: "application/x-www-form-urlencoded",
	}
	if in.ScheduledAt != "" {
		c.query = url.Values{"scheduled_at": {in.ScheduledAt}}
	}
	if defs.OptionsRequestID != "" {
		c.header = http.Header{"X-Options-Request-Id": {defs.OptionsRequestID}}
	}

	body, err := a.do(ctx, c)
	if err != nil {
		return nil, err
	}

	var resp handlers.FlowTriggerResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("could not decode trigger response: %w", err)
	}
	return withFields(body, map[string]any{"execution_url": s.executionURL(in.Namespace, in.FlowID, resp.ExecID)})
}

func encodeInputs(defs []handlers.FlowInput, inputs map[string]any) (url.Values, error) {
	known := make(map[string]bool, len(defs))
	form := url.Values{}
	for _, def := range defs {
		known[def.Name] = true
		v, ok := inputs[def.Name]

		if models.InputType(def.Type) == models.INPUT_TYPE_FILE {
			if ok || def.Required {
				return nil, errors.New("file inputs are not supported over MCP")
			}
			continue
		}
		if !ok || v == nil {
			continue
		}

		switch models.InputType(def.Type) {
		case models.INPUT_TYPE_CHECKBOX:
			on, err := truthy(v)
			if err != nil {
				return nil, fmt.Errorf("input %s: %w", def.Name, err)
			}
			if on {
				form.Set(def.Name, "true")
			}
		case models.INPUT_TYPE_NODE:
			values, ok := v.([]any)
			if !ok {
				values = []any{v}
			}
			for _, n := range values {
				s, err := scalar(n)
				if err != nil {
					return nil, fmt.Errorf("input %s: %w", def.Name, err)
				}
				form.Add(def.Name, s)
			}
		default:
			s, err := scalar(v)
			if err != nil {
				return nil, fmt.Errorf("input %s: %w", def.Name, err)
			}
			form.Set(def.Name, s)
		}
	}

	for name := range inputs {
		if !known[name] {
			return nil, fmt.Errorf("flow has no input named %s", name)
		}
	}
	return form, nil
}

func scalar(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(t), nil
	case json.Number:
		return t.String(), nil
	default:
		return "", fmt.Errorf("expected a string, number or boolean, got %T", v)
	}
}

func truthy(v any) (bool, error) {
	switch t := v.(type) {
	case bool:
		return t, nil
	case string:
		if t == "" {
			return false, nil
		}
		return strconv.ParseBool(t)
	case float64:
		return t != 0, nil
	default:
		return false, fmt.Errorf("expected a boolean, got %T", v)
	}
}

func finished(status handlers.ExecutionStatus) bool {
	switch models.ExecutionStatus(status) {
	case models.ExecutionStatusCompleted, models.ExecutionStatusErrored, models.ExecutionStatusCancelled:
		return true
	}
	return false
}

func (s *server) waitForExecution(ctx context.Context, a *api, in waitArgs) (any, error) {
	timeout := in.TimeoutSeconds
	if timeout <= 0 {
		timeout = defaultWaitSeconds
	}
	timeout = min(timeout, maxWaitSeconds)
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)

	for {
		body, sum, err := s.execution(ctx, a, in.Namespace, in.ExecID)
		if err != nil {
			return nil, err
		}

		fields := map[string]any{"done": false, "execution_url": s.executionURL(in.Namespace, sum.FlowID, in.ExecID)}
		switch {
		case finished(sum.Status):
			fields["done"] = true
			return withFields(body, fields)
		case models.ExecutionStatus(sum.Status) == models.ExecutionStatusPendingApproval:
			fields["message"] = approvalMessage
			return withFields(body, fields)
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return withFields(body, fields)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(min(pollInterval, remaining)):
		}
	}
}

func (s *server) executionLogs(ctx context.Context, a *api, in logsArgs) (any, error) {
	lines := in.TailLines
	if lines <= 0 {
		lines = defaultTailLines
	}
	lines = min(lines, maxTailLines)

	_, sum, err := s.execution(ctx, a, in.Namespace, in.ExecID)
	if err != nil {
		return nil, err
	}
	if !finished(sum.Status) {
		return nil, errors.New("execution is still running; call wait_for_execution first")
	}

	tb := &tailBuffer{max: maxLogBytes}
	code, err := a.stream(ctx, call{method: http.MethodGet, path: apiPath(in.Namespace, "logs", in.ExecID, "download")}, tb)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, responseError(code, tb.bytes())
	}

	logs, truncated := tail(tb.bytes(), tb.dropped, lines)
	return map[string]any{
		"exec_id":   in.ExecID,
		"status":    sum.Status,
		"logs":      logs,
		"truncated": truncated,
	}, nil
}

type tailBuffer struct {
	max     int
	buf     []byte
	dropped bool
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > 2*t.max {
		t.buf = append([]byte(nil), t.buf[len(t.buf)-t.max:]...)
		t.dropped = true
	}
	return len(p), nil
}

func (t *tailBuffer) bytes() []byte {
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
		t.dropped = true
	}
	return t.buf
}

func tail(b []byte, cut bool, n int) (string, bool) {
	if cut {
		if i := bytes.IndexByte(b, '\n'); i >= 0 {
			b = b[i+1:]
		}
	}
	b = bytes.TrimRight(b, "\n")
	if len(b) == 0 {
		return "", cut
	}

	lines := bytes.Split(b, []byte("\n"))
	truncated := cut
	if len(lines) > n {
		lines = lines[len(lines)-n:]
		truncated = true
	}
	return string(bytes.Join(lines, []byte("\n"))), truncated
}
