package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/cvhariharan/flowctl/internal/config"
	"github.com/cvhariharan/flowctl/internal/core"
	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/labstack/echo/v4"
)

func TestLogWebSocketProtocol(t *testing.T) {
	messages := make(chan models.StreamMessage, 12)
	terminal := make(chan core.LogStreamEndReason, 1)
	for i := 0; i < 10; i++ {
		messages <- models.StreamMessage{
			ActionID: "a1", MType: models.LogMessageType, NodeID: "n1",
			Val: fmt.Sprintf("line %d\n", i), Timestamp: "2026-08-12T10:04:11Z",
		}
	}
	messages <- models.StreamMessage{ActionID: "a1", MType: models.ResultMessageType, Val: `{"answer":"42"}`, Timestamp: "2026-08-12T10:04:12Z"}
	close(messages)
	terminal <- core.LogStreamEndComplete
	close(terminal)

	server := newLogWebSocketTestServer(t, func(context.Context) (<-chan models.StreamMessage, <-chan core.LogStreamEndReason, error) {
		return messages, terminal, nil
	})
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.CloseNow()

	var open openFrame
	readJSONFrame(t, ctx, conn, &open)
	if open.Type != frameOpen || open.Protocol != protocolVersion || open.ExecID != "exec-1" || !open.Replay {
		t.Fatalf("unexpected open frame: %+v", open)
	}

	// Client data frames are ignored in v1 and must not terminate the stream.
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"ignored":true}`)); err != nil {
		t.Fatalf("client Write() error = %v", err)
	}

	var batch batchFrame
	readJSONFrame(t, ctx, conn, &batch)
	if batch.Type != frameBatch || len(batch.Messages) != 11 {
		t.Fatalf("unexpected coalesced batch: type=%q messages=%d", batch.Type, len(batch.Messages))
	}
	if got := batch.Messages[10].Results["answer"]; got != "42" {
		t.Fatalf("result value = %q, want 42", got)
	}

	var end endFrame
	readJSONFrame(t, ctx, conn, &end)
	if end.Type != frameEnd || end.Reason != endComplete {
		t.Fatalf("unexpected end frame: %+v", end)
	}
	if _, _, err := conn.Read(ctx); websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		t.Fatalf("close status = %v, error = %v", websocket.CloseStatus(err), err)
	}
}

func TestLogWebSocketTimeoutEnd(t *testing.T) {
	messages := make(chan models.StreamMessage)
	close(messages)
	terminal := make(chan core.LogStreamEndReason, 1)
	terminal <- core.LogStreamEndTimeout
	close(terminal)
	server := newLogWebSocketTestServer(t, func(context.Context) (<-chan models.StreamMessage, <-chan core.LogStreamEndReason, error) {
		return messages, terminal, nil
	})
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.CloseNow()

	var open openFrame
	readJSONFrame(t, ctx, conn, &open)
	var end endFrame
	readJSONFrame(t, ctx, conn, &end)
	if end.Reason != endTimeout {
		t.Fatalf("end reason = %q, want %q", end.Reason, endTimeout)
	}
}

func TestLogWebSocketRejectsOversizedClientFrame(t *testing.T) {
	messages := make(chan models.StreamMessage)
	terminal := make(chan core.LogStreamEndReason)
	server := newLogWebSocketTestServer(t, func(context.Context) (<-chan models.StreamMessage, <-chan core.LogStreamEndReason, error) {
		return messages, terminal, nil
	})
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.CloseNow()

	var open openFrame
	readJSONFrame(t, ctx, conn, &open)
	if err := conn.Write(ctx, websocket.MessageText, make([]byte, websocketReadLimit+1)); err != nil {
		t.Fatalf("client Write() error = %v", err)
	}
	if _, _, err := conn.Read(ctx); websocket.CloseStatus(err) != websocket.StatusMessageTooBig {
		t.Fatalf("close status = %v, want %v (error = %v)", websocket.CloseStatus(err), websocket.StatusMessageTooBig, err)
	}
}

func newLogWebSocketTestServer(t *testing.T, source logStreamSource) *httptest.Server {
	t.Helper()
	h := newTestHandler(config.Config{})
	e := echo.New()
	e.GET("/", func(c echo.Context) error { return h.serveLogStream(c, "exec-1", source) })
	return httptest.NewServer(e)
}

func readJSONFrame(t *testing.T, ctx context.Context, conn *websocket.Conn, dst any) {
	t.Helper()
	typ, payload, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("message type = %v, want text", typ)
	}
	if err := json.Unmarshal(payload, dst); err != nil {
		t.Fatalf("Unmarshal(%s) error = %v", payload, err)
	}
}
