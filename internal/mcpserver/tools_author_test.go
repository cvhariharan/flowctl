package mcpserver

import (
	"context"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestDeleteFlow(t *testing.T) {
	for _, tt := range []struct {
		name    string
		status  int
		message string
	}{
		{name: "deleted", status: http.StatusOK},
		{name: "forbidden", status: http.StatusForbidden, message: "insufficient permissions"},
		{name: "not found", status: http.StatusNotFound, message: "flow not found"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var called bool
			h := newHarness(t, func(e *echo.Echo) {
				e.DELETE("/api/v1/:namespace/flows/:flowID", func(c echo.Context) error {
					called = true
					if c.Param("namespace") != "ops" || c.Param("flowID") != "deploy" {
						t.Errorf("unexpected target: %s", c.Request().URL.Path)
					}
					if got := c.Request().Header.Get("Authorization"); got != testToken {
						t.Errorf("Authorization = %q, want %q", got, testToken)
					}
					if tt.message != "" {
						return c.JSON(tt.status, map[string]any{"error": tt.message})
					}
					return c.NoContent(tt.status)
				})
			})

			res, text := h.call(t, "delete_flow", map[string]any{"namespace": "ops", "flow_id": "deploy"})
			if !called {
				t.Fatal("delete route was not called")
			}
			if tt.message != "" {
				if !res.IsError || text != tt.message {
					t.Fatalf("IsError = %v, text = %q, want %q", res.IsError, text, tt.message)
				}
				return
			}
			if res.IsError {
				t.Fatal(text)
			}
			if out := decode(t, text); out["ok"] != true {
				t.Errorf("result = %v, want ok=true", out)
			}
		})
	}
}

func TestDeleteFlowAnnotations(t *testing.T) {
	h := newHarness(t, func(e *echo.Echo) {})
	tools, err := h.cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "delete_flow" {
			continue
		}
		a := tool.Annotations
		if a == nil || a.ReadOnlyHint || a.DestructiveHint == nil || !*a.DestructiveHint || !a.IdempotentHint {
			t.Errorf("delete_flow annotations = %+v, want destructive and idempotent", a)
		}
		return
	}
	t.Fatal("delete_flow not listed")
}
