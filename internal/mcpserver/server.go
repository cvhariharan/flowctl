package mcpserver

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/labstack/echo/v4"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Options struct {
	RootURL     string
	Version     string
	Logger      *slog.Logger
	MaxBodySize int64
}

type callerKey struct{}

type caller struct {
	userID     string
	remoteAddr string
}

type server struct {
	d       *dispatcher
	rootURL string
	logger  *slog.Logger
	mcp     *mcp.Server
}

func NewHandler(h http.Handler, o Options) echo.HandlerFunc {
	s := newServer(h, o)
	sh := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.mcp }, &mcp.StreamableHTTPOptions{
		Stateless:                  true,
		JSONResponse:               true,
		Logger:                     o.Logger,
		MaxRequestBodyBytes:        o.MaxBodySize,
		DisableLocalhostProtection: true,
	})

	return func(c echo.Context) error {
		user, _ := c.Get("user").(models.UserInfo)
		r := c.Request()
		ctx := context.WithValue(r.Context(), callerKey{}, caller{userID: user.ID, remoteAddr: r.RemoteAddr})
		sh.ServeHTTP(c.Response(), r.WithContext(ctx))
		return nil
	}
}

func newServer(h http.Handler, o Options) *server {
	s := &server{
		d:       &dispatcher{h: h},
		rootURL: strings.TrimSuffix(o.RootURL, "/"),
		logger:  o.Logger,
		mcp:     mcp.NewServer(&mcp.Implementation{Name: "flowctl", Version: o.Version}, nil),
	}
	s.registerRunTools()
	s.registerAuthorTools()
	s.registerScheduleTools()
	s.registerVersionTools()
	return s
}

type toolFunc[In any] func(ctx context.Context, a *api, in In) (any, error)

func addTool[In any](s *server, t *mcp.Tool, fn toolFunc[In]) {
	mcp.AddTool(s.mcp, t, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		cl, _ := ctx.Value(callerKey{}).(caller)
		a := &api{d: s.d, remoteAddr: cl.remoteAddr}
		if req.Extra != nil {
			a.auth = req.Extra.Header.Get("Authorization")
		}

		out, err := fn(ctx, a, in)
		s.logger.Info("mcp tool call", "tool", t.Name, "user", cl.userID, "status", a.status)
		return nil, out, err
	})
}

func (s *server) executionURL(namespace, flowID, execID string) string {
	return s.rootURL + "/view/" + url.PathEscape(namespace) + "/results/" + url.PathEscape(flowID) + "/" + url.PathEscape(execID)
}

func readOnly() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{ReadOnlyHint: true}
}

func destructive(idempotent bool) *mcp.ToolAnnotations {
	t := true
	return &mcp.ToolAnnotations{DestructiveHint: &t, IdempotentHint: idempotent}
}

func idempotent() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{IdempotentHint: true}
}
