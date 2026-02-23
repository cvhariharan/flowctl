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
	// SetActionID is a global value that is used in Write calls
	SetActionID(id string)
	// SetRetry sets the retry count for the current action
	SetRetry(retry int32)
	// Checkpoint is an underlying function to log different message types. Used by Write calls too. If the id is set, it will
	// override the global action ID
	Checkpoint(id string, nodeID string, val interface{}, mtype MessageType) error

	Close() error
}

// LogManager manages multiple loggers and can be used for enforce retention, log rotation etc.
type LogManager interface {
	NewLogger(id string) (Logger, error)
	LoggerExists(execID string) bool
	StreamLogs(ctx context.Context, execID string, actionRetries map[string]int32) (<-chan string, error)
	Run(ctx context.Context, logger *slog.Logger) error
}

type MessageType string

const (
	LogMessageType       MessageType = "log"
	ErrMessageType       MessageType = "error"
	ResultMessageType    MessageType = "result"
	StateMessageType     MessageType = "state"
	CancelledMessageType MessageType = "cancelled"
	ApprovedMessageType  MessageType = "approved"
)

type StreamMessage struct {
	ActionID  string      `json:"action_id"`
	MType     MessageType `json:"message_type"`
	NodeID    string      `json:"node_id"`
	Val       string      `json:"value"`
	Timestamp string      `json:"timestamp"`
	Retry     int32       `json:"retry"`
}

// NodeContextLogger wraps a Logger to provide node context for concurrent execution
type NodeContextLogger struct {
	logger   Logger
	actionID string
	nodeID   string
}

// NewNodeContextLogger creates a new NodeContextLogger.
func NewNodeContextLogger(logger Logger, actionID, nodeID string) *NodeContextLogger {
	return &NodeContextLogger{
		logger:   logger,
		actionID: actionID,
		nodeID:   nodeID,
	}
}

// Write implements io.Writer by delegating to Checkpoint with node context.
func (n *NodeContextLogger) Write(p []byte) (int, error) {
	if err := n.logger.Checkpoint(n.actionID, n.nodeID, p, LogMessageType); err != nil {
		return 0, err
	}
	return len(p), nil
}

// GetID delegates to the underlying logger.
func (n *NodeContextLogger) GetID() string {
	return n.logger.GetID()
}

// SetActionID updates the action ID for this node context.
func (n *NodeContextLogger) SetActionID(id string) {
	n.actionID = id
}

// SetRetry delegates to the underlying logger.
func (n *NodeContextLogger) SetRetry(retry int32) {
	n.logger.SetRetry(retry)
}

// Checkpoint delegates to the underlying logger with node context.
// If id is empty, uses the stored actionID.
func (n *NodeContextLogger) Checkpoint(id string, nodeID string, val interface{}, mtype MessageType) error {
	if id == "" {
		id = n.actionID
	}
	if nodeID == "" {
		nodeID = n.nodeID
	}
	return n.logger.Checkpoint(id, nodeID, val, mtype)
}

// Close delegates to the underlying logger.
func (n *NodeContextLogger) Close() error {
	return n.logger.Close()
}
