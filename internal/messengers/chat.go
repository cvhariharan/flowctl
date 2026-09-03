package messengers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cvhariharan/flowctl/internal/config"
	"github.com/invopop/jsonschema"
)

const (
	chatMaxErrorLen    = 1500
	chatMaxResponseLen = 512
	chatTimeFormat     = "2006-01-02 15:04:05 MST"
)

type ChatNotifyConfig struct {
	URL string `json:"url" jsonschema:"title=Webhook URL,description=Slack or Mattermost incoming webhook URL" jsonschema_extras:"placeholder=slack or mattermost webhook URL"`
}

func GetChatNotifySchema() interface{} {
	return jsonschema.Reflect(&ChatNotifyConfig{})
}

type chatField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

type chatAttachment struct {
	Color    string      `json:"color"`
	Fallback string      `json:"fallback"`
	Title    string      `json:"title"`
	Fields   []chatField `json:"fields"`
	MrkdwnIn []string    `json:"mrkdwn_in"`
	Footer   string      `json:"footer"`
}

type chatPayload struct {
	Attachments []chatAttachment `json:"attachments"`
}

type ChatMessenger struct {
	client  *http.Client
	logger  *slog.Logger
	rootURL string
}

func NewChatMessenger(cfg config.ChatConfig, logger *slog.Logger, rootURL string) (*ChatMessenger, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("chat messenger is disabled")
	}

	timeout := 30 * time.Second
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}

	return &ChatMessenger{
		client:  &http.Client{Timeout: timeout},
		logger:  logger,
		rootURL: rootURL,
	}, nil
}

func (c *ChatMessenger) Send(ctx context.Context, msg Message) error {
	targetURL, _ := msg.Config["url"].(string)
	if targetURL == "" {
		return fmt.Errorf("chat messenger requires a url in config")
	}

	var payload chatPayload
	switch msg.Event {
	case EventFlowExecution:
		evt, ok := msg.Data.(FlowExecutionEvent)
		if !ok {
			return fmt.Errorf("chat messenger: expected FlowExecutionEvent, got %T", msg.Data)
		}
		payload = c.buildPayload(evt)
	default:
		return fmt.Errorf("chat messenger: unsupported event type %q", msg.Event)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal chat payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.logger.Error("failed to send chat notification", "error", err)
		return fmt.Errorf("failed to send chat notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, chatMaxResponseLen))
		reason := strings.TrimSpace(string(respBody))
		c.logger.Error("chat webhook returned non-2xx status", "status", resp.StatusCode, "response", reason)
		return fmt.Errorf("chat webhook returned status %d: %s", resp.StatusCode, reason)
	}

	c.logger.Debug("chat notification sent", "event", msg.Event)
	return nil
}

func (c *ChatMessenger) buildPayload(evt FlowExecutionEvent) chatPayload {
	fields := []chatField{
		{Title: "Flow", Value: evt.FlowName, Short: true},
		{Title: "Flow ID", Value: evt.FlowID, Short: true},
		{Title: "Namespace", Value: evt.Namespace, Short: true},
		{Title: "Execution", Value: c.execLink(evt), Short: true},
	}
	if trigger := triggerTypeLabel(evt.TriggerType); trigger != "" {
		fields = append(fields, chatField{Title: "Trigger", Value: trigger, Short: true})
	}
	if evt.TriggeredBy != "" {
		fields = append(fields, chatField{Title: "Triggered by", Value: evt.TriggeredBy, Short: true})
	}
	if evt.StartedAt != nil {
		fields = append(fields, chatField{Title: "Started", Value: evt.StartedAt.UTC().Format(chatTimeFormat), Short: true})
	}
	if evt.Error != "" {
		fields = append(fields, chatField{Title: "Error", Value: truncate(evt.Error, chatMaxErrorLen)})
	}

	return chatPayload{
		Attachments: []chatAttachment{{
			Color:    chatStatusColor(evt.Status),
			Fallback: fmt.Sprintf("%s %s", evt.FlowName, statusText(evt.Status)),
			Title:    fmt.Sprintf("%s: %s", evt.FlowName, statusLabel(evt.Status)),
			Fields:   fields,
			MrkdwnIn: []string{"fields"},
			Footer:   "flowctl",
		}},
	}
}

func (c *ChatMessenger) execLink(evt FlowExecutionEvent) string {
	short := shortExecID(evt.ExecID)
	if c.rootURL == "" {
		return short
	}
	return fmt.Sprintf("<%s/view/%s/results/%s/%s|%s>",
		strings.TrimSuffix(c.rootURL, "/"), evt.Namespace, evt.FlowID, evt.ExecID, short)
}

func triggerTypeLabel(triggerType string) string {
	switch triggerType {
	case "manual":
		return "Manual"
	case "scheduled":
		return "Scheduled"
	default:
		return ""
	}
}

func statusLabel(status string) string {
	switch status {
	case "completed":
		return "Completed"
	case "errored":
		return "Failed"
	case "cancelled":
		return "Cancelled"
	case "pending_approval":
		return "Waiting for Approval"
	default:
		return status
	}
}

func chatStatusColor(status string) string {
	switch status {
	case "completed":
		return "#2eb886"
	case "errored":
		return "#d64545"
	case "cancelled":
		return "#808080"
	case "pending_approval":
		return "#daa038"
	default:
		return "#439fe0"
	}
}

func shortExecID(execID string) string {
	if len(execID) > 8 {
		return execID[:8]
	}
	return execID
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func (c *ChatMessenger) Close() {}
