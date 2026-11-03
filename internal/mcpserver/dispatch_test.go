package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/labstack/echo/v4"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const testToken = "Bearer fctl_pat_test"

type headerTransport struct {
	header http.Header
}

func (t headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	for k, v := range t.header {
		r.Header[k] = v
	}
	return http.DefaultTransport.RoundTrip(r)
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type harness struct {
	cs   *mcp.ClientSession
	logs *syncBuffer
}

func newHarness(t *testing.T, routes func(e *echo.Echo)) *harness {
	t.Helper()

	e := echo.New()
	routes(e)

	logs := &syncBuffer{}
	e.Any("/mcp", NewHandler(e, Options{
		RootURL:     "https://flowctl.example.com/",
		Version:     "test",
		Logger:      slog.New(slog.NewTextHandler(logs, nil)),
		MaxBodySize: 1 << 20,
	}), func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("user", models.UserInfo{ID: "user-1"})
			return next(c)
		}
	})

	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)

	transport := headerTransport{header: http.Header{
		"Authorization": {testToken},
		"Cookie":        {"flowctl_session=abc"},
		"X-User-Uuid":   {"someone-else"},
		"X-Csrf-Token":  {"csrf"},
	}}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	cs, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:             srv.URL + "/mcp",
		HTTPClient:           &http.Client{Transport: transport},
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })

	return &harness{cs: cs, logs: logs}
}

func (h *harness) call(t *testing.T, name string, args map[string]any) (*mcp.CallToolResult, string) {
	t.Helper()
	res, err := h.cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	return res, text.String()
}

func decode(t *testing.T, s string) map[string]any {
	t.Helper()
	out := make(map[string]any)
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}
	return out
}

func TestDispatchForwardsOnlyAuthorization(t *testing.T) {
	var got *http.Request
	h := newHarness(t, func(e *echo.Echo) {
		e.GET("/api/v1/namespaces", func(c echo.Context) error {
			got = c.Request()
			return c.JSON(http.StatusOK, map[string]any{"namespaces": []string{"a"}})
		})
	})

	res, text := h.call(t, "list_namespaces", nil)
	if res.IsError {
		t.Fatalf("unexpected error: %s", text)
	}
	if got == nil {
		t.Fatal("route was not called")
	}
	if v := got.Header.Get("Authorization"); v != testToken {
		t.Errorf("Authorization = %q, want %q", v, testToken)
	}
	for _, name := range []string{"Cookie", "X-User-UUID", "X-CSRF-Token"} {
		if v := got.Header.Get(name); v != "" {
			t.Errorf("%s leaked to inner request: %q", name, v)
		}
	}
	if got.RemoteAddr == "" {
		t.Error("RemoteAddr was not copied from the outer request")
	}

	logs := h.logs.String()
	for _, want := range []string{`msg="mcp tool call"`, "tool=list_namespaces", "user=user-1", "status=200"} {
		if !strings.Contains(logs, want) {
			t.Errorf("audit log missing %q: %s", want, logs)
		}
	}
}

func TestDispatchErrorBecomesToolError(t *testing.T) {
	h := newHarness(t, func(e *echo.Echo) {
		e.GET("/api/v1/:namespace/flows", func(c echo.Context) error {
			return c.JSON(http.StatusForbidden, map[string]any{"error": "insufficient permissions", "code": "FORBIDDEN"})
		})
		e.POST("/api/v1/:namespace/flows", func(c echo.Context) error {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error":   "",
				"code":    "VALIDATION_FAILED",
				"details": map[string]string{"field": "env", "error": "env is required"},
			})
		})
	})

	res, text := h.call(t, "list_flows", map[string]any{"namespace": "a"})
	if !res.IsError {
		t.Fatal("expected IsError")
	}
	if text != "insufficient permissions" {
		t.Errorf("text = %q", text)
	}

	res, text = h.call(t, "create_flow", map[string]any{
		"namespace":  "a",
		"definition": map[string]any{"metadata": map[string]any{"name": "x"}, "inputs": []any{}, "actions": []any{}},
	})
	if !res.IsError {
		t.Fatal("expected IsError")
	}
	if !strings.Contains(text, "VALIDATION_FAILED") || !strings.Contains(text, `"field":"env"`) {
		t.Errorf("text = %q", text)
	}
}

func TestDispatchEscapesPathSegments(t *testing.T) {
	var usersHit bool
	var flowID string
	h := newHarness(t, func(e *echo.Echo) {
		e.GET("/api/v1/users", func(c echo.Context) error {
			usersHit = true
			return c.JSON(http.StatusOK, map[string]any{})
		})
		e.GET("/api/v1/:namespace/flows/:flowID/config", func(c echo.Context) error {
			flowID = c.Param("flowID")
			return c.JSON(http.StatusOK, map[string]any{})
		})
	})

	h.call(t, "get_flow_definition", map[string]any{"namespace": "a", "flow_id": "../../users"})
	if flowID != "..%2F..%2Fusers" && flowID != "../../users" {
		t.Errorf("flowID = %q, want the escaped literal", flowID)
	}
	h.call(t, "get_flow_definition", map[string]any{"namespace": "../users", "flow_id": "x"})
	if usersHit {
		t.Fatal("path traversal reached /api/v1/users")
	}
}

func TestCreateFlowSchemaFollowsValidateTags(t *testing.T) {
	h := newHarness(t, func(e *echo.Echo) {})

	tools, err := h.cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	for _, tool := range tools.Tools {
		if tool.Name == "create_flow" {
			b, _ := json.Marshal(tool.InputSchema)
			schema = decode(t, string(b))
		}
	}
	if schema == nil {
		t.Fatal("create_flow not listed")
	}

	def := schema["properties"].(map[string]any)["definition"].(map[string]any)
	meta := def["properties"].(map[string]any)["metadata"].(map[string]any)
	if got, _ := json.Marshal(meta["required"]); string(got) != `["name"]` {
		t.Errorf("metadata.required = %s", got)
	}
	mode := meta["properties"].(map[string]any)["execution_mode"].(map[string]any)
	if got, _ := json.Marshal(mode["enum"]); string(got) != `["","sequential","dag"]` {
		t.Errorf("execution_mode.enum = %s", got)
	}
}
