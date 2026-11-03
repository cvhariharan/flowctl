package mcpserver

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cvhariharan/flowctl/internal/handlers"
	"github.com/labstack/echo/v4"
)

func TestEncodeInputs(t *testing.T) {
	defs := []handlers.FlowInput{
		{Name: "name", Type: "string"},
		{Name: "count", Type: "number"},
		{Name: "force", Type: "checkbox"},
		{Name: "hosts", Type: "node"},
		{Name: "env", Type: "select"},
		{Name: "upload", Type: "file"},
	}

	tests := []struct {
		name    string
		inputs  map[string]any
		want    string
		wantErr string
	}{
		{name: "string", inputs: map[string]any{"name": "web"}, want: "name=web"},
		{name: "integer number", inputs: map[string]any{"count": float64(3)}, want: "count=3"},
		{name: "decimal number", inputs: map[string]any{"count": 2.5}, want: "count=2.5"},
		{name: "checkbox true", inputs: map[string]any{"force": true}, want: "force=true"},
		{name: "checkbox false", inputs: map[string]any{"force": false}, want: ""},
		{name: "checkbox string", inputs: map[string]any{"force": "false"}, want: ""},
		{name: "node array", inputs: map[string]any{"hosts": []any{"a", "b"}}, want: "hosts=a&hosts=b"},
		{name: "node single", inputs: map[string]any{"hosts": "a"}, want: "hosts=a"},
		{name: "select", inputs: map[string]any{"env": "prod"}, want: "env=prod"},
		{name: "file rejected", inputs: map[string]any{"upload": "x"}, wantErr: "file inputs are not supported over MCP"},
		{name: "unknown input", inputs: map[string]any{"nope": "x"}, wantErr: "flow has no input named nope"},
		{name: "object rejected", inputs: map[string]any{"name": map[string]any{}}, wantErr: "input name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form, err := encodeInputs(defs, tt.inputs)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := form.Encode(); got != tt.want {
				t.Errorf("form = %q, want %q", got, tt.want)
			}
		})
	}

	if _, err := encodeInputs([]handlers.FlowInput{{Name: "upload", Type: "file", Required: true}}, nil); err == nil {
		t.Error("expected a required file input to be rejected")
	}
}

func TestTriggerFlow(t *testing.T) {
	var form map[string][]string
	var optionsID string
	h := newHarness(t, func(e *echo.Echo) {
		e.GET("/api/v1/:namespace/flows/:flowID/inputs", func(c echo.Context) error {
			return c.JSON(http.StatusOK, handlers.FlowInputsResp{
				Inputs:           []handlers.FlowInput{{Name: "hosts", Type: "node"}, {Name: "force", Type: "checkbox"}},
				OptionsRequestID: "opts-1",
			})
		})
		e.POST("/api/v1/:namespace/trigger/:flow", func(c echo.Context) error {
			params, _ := c.FormParams()
			form = params
			optionsID = c.Request().Header.Get("X-Options-Request-ID")
			return c.JSON(http.StatusOK, handlers.FlowTriggerResp{ExecID: "e1"})
		})
	})

	res, text := h.call(t, "trigger_flow", map[string]any{
		"namespace": "ops",
		"flow_id":   "deploy",
		"inputs":    map[string]any{"hosts": []any{"a", "b"}, "force": true},
	})
	if res.IsError {
		t.Fatal(text)
	}
	if !reflect.DeepEqual(form, map[string][]string{"hosts": {"a", "b"}, "force": {"true"}}) {
		t.Errorf("form = %v", form)
	}
	if optionsID != "opts-1" {
		t.Errorf("options request id = %q", optionsID)
	}
	out := decode(t, text)
	if out["exec_id"] != "e1" || out["execution_url"] != "https://flowctl.example.com/view/ops/results/deploy/e1" {
		t.Errorf("result = %v", out)
	}
}

func summaryRoute(e *echo.Echo, status func() string) {
	e.GET("/api/v1/:namespace/flows/executions/:execID", func(c echo.Context) error {
		return c.JSON(http.StatusOK, handlers.ExecutionSummary{
			ID:     c.Param("execID"),
			FlowID: "deploy",
			Status: handlers.ExecutionStatus(status()),
		})
	})
}

func TestWaitForExecution(t *testing.T) {
	old := pollInterval
	pollInterval = 10 * time.Millisecond
	t.Cleanup(func() { pollInterval = old })

	t.Run("completes", func(t *testing.T) {
		var calls atomic.Int32
		steps := []string{"pending", "pending", "completed"}
		h := newHarness(t, func(e *echo.Echo) {
			summaryRoute(e, func() string {
				n := int(calls.Add(1)) - 1
				return steps[min(n, len(steps)-1)]
			})
		})

		_, text := h.call(t, "wait_for_execution", map[string]any{"namespace": "ops", "exec_id": "e1"})
		out := decode(t, text)
		if out["done"] != true || out["status"] != "completed" {
			t.Errorf("result = %v", out)
		}
		if calls.Load() != 3 {
			t.Errorf("polled %d times, want 3", calls.Load())
		}
	})

	t.Run("times out", func(t *testing.T) {
		h := newHarness(t, func(e *echo.Echo) {
			summaryRoute(e, func() string { return "pending" })
		})

		start := time.Now()
		_, text := h.call(t, "wait_for_execution", map[string]any{"namespace": "ops", "exec_id": "e1", "timeout_seconds": 1})
		out := decode(t, text)
		if out["done"] != false || out["status"] != "pending" {
			t.Errorf("result = %v", out)
		}
		if elapsed := time.Since(start); elapsed < time.Second || elapsed > 5*time.Second {
			t.Errorf("waited %s", elapsed)
		}
	})

	t.Run("stops at approval", func(t *testing.T) {
		h := newHarness(t, func(e *echo.Echo) {
			summaryRoute(e, func() string { return "pending_approval" })
		})

		_, text := h.call(t, "wait_for_execution", map[string]any{"namespace": "ops", "exec_id": "e1"})
		out := decode(t, text)
		if out["done"] != false || out["message"] != approvalMessage {
			t.Errorf("result = %v", out)
		}
		if out["execution_url"] != "https://flowctl.example.com/view/ops/results/deploy/e1" {
			t.Errorf("execution_url = %v", out["execution_url"])
		}
	})
}

func TestExecutionLogs(t *testing.T) {
	logRoute := func(lines int) func(e *echo.Echo) {
		return func(e *echo.Echo) {
			summaryRoute(e, func() string { return "completed" })
			e.GET("/api/v1/:namespace/logs/:logID/download", func(c echo.Context) error {
				c.Response().WriteHeader(http.StatusOK)
				for i := range lines {
					fmt.Fprintf(c.Response(), "line %05d %s\n", i, strings.Repeat("x", 80))
				}
				return nil
			})
		}
	}

	t.Run("tail", func(t *testing.T) {
		h := newHarness(t, logRoute(50))
		_, text := h.call(t, "get_execution_logs", map[string]any{"namespace": "ops", "exec_id": "e1", "tail_lines": 3})
		out := decode(t, text)
		lines := strings.Split(out["logs"].(string), "\n")
		if len(lines) != 3 || !strings.HasPrefix(lines[0], "line 00047") || !strings.HasPrefix(lines[2], "line 00049") {
			t.Errorf("logs = %q", out["logs"])
		}
		if out["truncated"] != true {
			t.Error("expected truncated")
		}
	})

	t.Run("whole log", func(t *testing.T) {
		h := newHarness(t, logRoute(5))
		_, text := h.call(t, "get_execution_logs", map[string]any{"namespace": "ops", "exec_id": "e1"})
		out := decode(t, text)
		if n := len(strings.Split(out["logs"].(string), "\n")); n != 5 || out["truncated"] != false {
			t.Errorf("lines = %d, truncated = %v", n, out["truncated"])
		}
	})

	t.Run("byte cap", func(t *testing.T) {
		h := newHarness(t, logRoute(20000))
		_, text := h.call(t, "get_execution_logs", map[string]any{"namespace": "ops", "exec_id": "e1", "tail_lines": 2000})
		out := decode(t, text)
		logs := out["logs"].(string)
		if len(logs) > maxLogBytes {
			t.Errorf("logs are %d bytes, cap is %d", len(logs), maxLogBytes)
		}
		if !strings.HasPrefix(logs, "line ") || !strings.HasSuffix(logs, "line 19999 "+strings.Repeat("x", 80)) {
			t.Errorf("logs do not end at the last line or start mid-line")
		}
		if out["truncated"] != true {
			t.Error("expected truncated")
		}
	})

	t.Run("running", func(t *testing.T) {
		h := newHarness(t, func(e *echo.Echo) {
			summaryRoute(e, func() string { return "pending" })
		})
		res, text := h.call(t, "get_execution_logs", map[string]any{"namespace": "ops", "exec_id": "e1"})
		if !res.IsError || text != "execution is still running; call wait_for_execution first" {
			t.Errorf("IsError = %v, text = %q", res.IsError, text)
		}
	})
}
