package streamlogger

import (
	"context"
	"io"
	"log/slog"
)

// Logger is used to write individual execution logs to different backends
type Logger interface {
	io.Writer
	GetID() string
	// Checkpoint is an underlying function to log different message types. Used by Write calls too.
	Checkpoint(id string, nodeID string, val interface{}, mtype MessageType, retry int32) error

	Close() error
}

// LogManager manages multiple loggers and can be used for enforce retention, log rotation etc.
type LogManager interface {
	NewLogger(id string) (Logger, error)
	LoggerExists(execID string) bool
	StreamLogs(ctx context.Context, execID string, actionRetries map[string]int32) (<-chan string, error)
	GetRawLogs(ctx context.Context, execID string, w io.Writer) error
	Run(ctx context.Context, logger *slog.Logger) error
}

type MessageType string

const (
	LogMessageType       MessageType = "log"
	ErrMessageType       MessageType = "error"
	ResultMessageType    MessageType = "result"
	CancelledMessageType MessageType = "cancelled"
)

type StreamMessage struct {
	ActionID  string      `json:"action_id"`
	MType     MessageType `json:"message_type"`
	NodeID    string      `json:"node_id"`
	Val       string      `json:"value"`
	Timestamp string      `json:"timestamp"`
	Retry     int32       `json:"retry"`
}

// NodeContextLogger wraps a Logger to provide action and node context for concurrent execution
type NodeContextLogger struct {
	logger   Logger
	actionID string
	nodeID   string
	retry    int32
}

// NewNodeContextLogger creates a new NodeContextLogger.
func NewNodeContextLogger(logger Logger, actionID, nodeID string, retry int32) *NodeContextLogger {
	return &NodeContextLogger{
		logger:   logger,
		actionID: actionID,
		nodeID:   nodeID,
		retry:    retry,
	}
}

// Write implements io.Writer by delegating to Checkpoint with node context.
func (n *NodeContextLogger) Write(p []byte) (int, error) {
	if err := n.logger.Checkpoint(n.actionID, n.nodeID, p, LogMessageType, n.retry); err != nil {
		return 0, err
	}
	return len(p), nil
}

// GetID delegates to the underlying logger.
func (n *NodeContextLogger) GetID() string {
	return n.logger.GetID()
}

// Checkpoint delegates to the underlying logger with node context.
// If id is empty, uses the stored actionID.
func (n *NodeContextLogger) Checkpoint(id string, nodeID string, val interface{}, mtype MessageType, retry int32) error {
	if id == "" {
		id = n.actionID
	}
	if nodeID == "" {
		nodeID = n.nodeID
	}
	return n.logger.Checkpoint(id, nodeID, val, mtype, retry)
}

// Close delegates to the underlying logger.
func (n *NodeContextLogger) Close() error {
	return n.logger.Close()
}
