package messengers

import (
	"bytes"
	"context"
	"crypto/tls"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"strings"

	"github.com/cvhariharan/flowctl/internal/config"
	"github.com/invopop/jsonschema"
	"github.com/knadh/smtppool/v2"
)

//go:embed templates/*.html
var templateFS embed.FS

// EmailNotifyConfig defines the messenger-specific configuration schema for email notifications.
type EmailNotifyConfig struct {
	Receivers []string `json:"receivers" jsonschema:"title=Recipients,description=Users or groups to notify" jsonschema_extras:"widget=userselector"`
}

func GetEmailNotifySchema() interface{} {
	return jsonschema.Reflect(&EmailNotifyConfig{})
}

// EmailMessenger sends emails using an SMTP connection pool
type EmailMessenger struct {
	pool          *smtppool.Pool
	from          string
	groupResolver GroupResolver
	logger        *slog.Logger
	templates     *template.Template
	rootURL       string
}

// NewEmailMessenger creates a new EmailMessenger with the given SMTP configuration
func NewEmailMessenger(cfg config.SMTPConfig, groupResolver GroupResolver, logger *slog.Logger, rootURL string) (*EmailMessenger, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("email messenger is disabled")
	}

	var sslType smtppool.SSLType
	switch cfg.SSL {
	case "tls":
		sslType = smtppool.SSLTLS
	case "starttls":
		sslType = smtppool.SSLSTARTTLS
	default:
		sslType = smtppool.SSLNone
	}

	pool, err := smtppool.New(smtppool.Opt{
		Host:     cfg.Host,
		Port:     cfg.Port,
		MaxConns: cfg.MaxConns,
		Auth:     smtpAuth(cfg.Username, cfg.Password),
		SSL:      sslType,
		TLSConfig: smtpTLSConfig(cfg.Host, cfg.TLSSkipVerify),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create SMTP pool: %w", err)
	}

	fromAddr := cfg.FromAddress
	if cfg.FromName != "" {
		fromAddr = fmt.Sprintf("%s <%s>", cfg.FromName, cfg.FromAddress)
	}

	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse email templates: %w", err)
	}

	return &EmailMessenger{
		pool:          pool,
		from:          fromAddr,
		groupResolver: groupResolver,
		logger:        logger,
		templates:     tmpl,
		rootURL:       rootURL,
	}, nil
}

// Send sends an email message to receivers specified in msg.Config["receivers"].
// Receivers can be email addresses or "group:name" references that are resolved via GroupResolver.
func (e *EmailMessenger) Send(ctx context.Context, msg Message) error {
	receivers := configStringSlice(msg.Config, "receivers")
	if len(receivers) == 0 {
		return nil
	}

	to := e.resolveReceivers(ctx, receivers)
	if len(to) == 0 {
		return nil
	}

	var subject, body string
	switch msg.Event {
	case EventFlowExecution:
		evt, ok := msg.Data.(FlowExecutionEvent)
		if !ok {
			return fmt.Errorf("email messenger: expected FlowExecutionEvent, got %T", msg.Data)
		}
		subject = e.buildSubject(evt)
		body = e.buildBody(evt)
	default:
		return fmt.Errorf("email messenger: unsupported event type %q", msg.Event)
	}

	email := smtppool.Email{
		From:    e.from,
		To:      to,
		Subject: subject,
		HTML:    []byte(body),
	}

	if err := e.pool.Send(email); err != nil {
		e.logger.Error("failed to send email",
			"to", to,
			"subject", subject,
			"error", err,
		)
		return fmt.Errorf("failed to send email: %w", err)
	}

	e.logger.Debug("email sent",
		"to", to,
		"subject", subject,
	)
	return nil
}

// buildSubject creates the email subject line from event data.
func (e *EmailMessenger) buildSubject(evt FlowExecutionEvent) string {
	var status string
	switch evt.Status {
	case "completed":
		status = "[Success]"
	case "errored":
		status = "[Failed]"
	case "cancelled":
		status = "[Cancelled]"
	case "pending_approval":
		status = "[Waiting]"
	default:
		status = "[Update]"
	}

	return fmt.Sprintf("%s Flow %s - %s", status, evt.FlowName, evt.ExecID[:8])
}

// buildBody renders the HTML email body from event data.
func (e *EmailMessenger) buildBody(evt FlowExecutionEvent) string {
	var statusMsg string
	switch evt.Status {
	case "completed":
		statusMsg = "has completed successfully"
	case "errored":
		statusMsg = "has failed with an error"
	case "cancelled":
		statusMsg = "was cancelled"
	case "pending_approval":
		statusMsg = "is waiting for approval"
	default:
		statusMsg = "status changed to " + evt.Status
	}

	data := struct {
		FlowName  string
		FlowID    string
		ExecID    string
		Status    string
		Namespace string
		StatusMsg string
		Error     string
		RootURL   string
	}{
		FlowName:  evt.FlowName,
		FlowID:    evt.FlowID,
		ExecID:    evt.ExecID,
		Status:    evt.Status,
		StatusMsg: statusMsg,
		Namespace: evt.Namespace,
		Error:     evt.Error,
		RootURL:   e.rootURL,
	}

	var buf bytes.Buffer
	if err := e.templates.ExecuteTemplate(&buf, "notification.html", data); err != nil {
		e.logger.Error("failed to execute template", "template", "notification.html", "error", err)
		return fmt.Sprintf("Flow %s has %s", evt.FlowName, statusMsg)
	}

	return buf.String()
}

// resolveReceivers expands "group:name" entries into member emails and passes
// plain email addresses unchanged.
func (e *EmailMessenger) resolveReceivers(ctx context.Context, receivers []string) []string {
	var to []string
	for _, r := range receivers {
		if groupName, ok := strings.CutPrefix(r, "group:"); ok {
			if groupName == "" {
				continue
			}
			if e.groupResolver == nil {
				e.logger.Warn("group resolver not configured, skipping group", "group", groupName)
				continue
			}
			emails, err := e.groupResolver.ResolveGroupEmails(ctx, groupName)
			if err != nil {
				e.logger.Error("failed to resolve group", "group", groupName, "error", err)
				continue
			}
			to = append(to, emails...)
		} else if r != "" {
			to = append(to, r)
		}
	}
	return to
}

// smtpAuth returns nil when username is empty, skipping SMTP AUTH entirely.
// This allows connecting to local MTAs that don't require authentication.
func smtpAuth(username, password string) *smtppool.LoginAuth {
	if username == "" {
		return nil
	}
	return &smtppool.LoginAuth{Username: username, Password: password}
}

// smtpTLSConfig returns a TLS config with the given server name.
// If skipVerify is true, certificate verification is disabled.
func smtpTLSConfig(host string, skipVerify bool) *tls.Config {
	if !skipVerify && host != "" {
		return &tls.Config{ServerName: host}
	}
	if skipVerify {
		return &tls.Config{InsecureSkipVerify: true}
	}
	return nil
}

// Close closes the SMTP connection pool
func (e *EmailMessenger) Close() {
	if e.pool != nil {
		e.pool.Close()
	}
}
