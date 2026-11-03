package mcpserver

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestVersionTools(t *testing.T) {
	var got string
	record := func(c echo.Context) error {
		got = c.Request().Method + " " + c.Path() + " flow=" + c.Param("flowID") + " version=" + c.Param("version") + " query=" + c.QueryString()
		return c.JSON(http.StatusOK, map[string]any{})
	}
	h := newHarness(t, func(e *echo.Echo) {
		e.GET("/api/v1/:namespace/flows/:flowID/versions", record)
		e.GET("/api/v1/:namespace/flows/:flowID/versions/diff", record)
		e.GET("/api/v1/:namespace/flows/:flowID/versions/:version", record)
		e.POST("/api/v1/:namespace/flows/:flowID/versions/:version/activate", record)
	})

	tests := []struct {
		tool string
		args map[string]any
		want string
	}{
		{
			tool: "list_flow_versions",
			args: map[string]any{"namespace": "ops", "flow_id": "deploy", "page": 2},
			want: "GET /api/v1/:namespace/flows/:flowID/versions flow=deploy version= query=page=2",
		},
		{
			tool: "get_flow_version",
			args: map[string]any{"namespace": "ops", "flow_id": "deploy", "version": 3},
			want: "GET /api/v1/:namespace/flows/:flowID/versions/:version flow=deploy version=3 query=",
		},
		{
			tool: "diff_flow_versions",
			args: map[string]any{"namespace": "ops", "flow_id": "deploy", "from": 3, "to": 5},
			want: "GET /api/v1/:namespace/flows/:flowID/versions/diff flow=deploy version= query=from=3&to=5",
		},
		{
			tool: "diff_flow_versions",
			args: map[string]any{"namespace": "ops", "flow_id": "deploy", "from": 3},
			want: "GET /api/v1/:namespace/flows/:flowID/versions/diff flow=deploy version= query=from=3",
		},
		{
			tool: "activate_flow_version",
			args: map[string]any{"namespace": "ops", "flow_id": "deploy", "version": 2},
			want: "POST /api/v1/:namespace/flows/:flowID/versions/:version/activate flow=deploy version=2 query=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got = ""
			res, text := h.call(t, tt.tool, tt.args)
			if res.IsError {
				t.Fatal(text)
			}
			if got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}
