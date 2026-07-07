package scheduler

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

const (
	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
	TaskStatusCancelled = "cancelled"
)

var (
	ErrPendingApproval    = errors.New("pending approval")
	ErrExecutionCancelled = errors.New("execution cancelled")
)

type TriggerType string

const (
	TriggerTypeManual    TriggerType = "manual"
	TriggerTypeScheduled TriggerType = "scheduled"
)

type Task struct {
	UUID      string
	ExecID    string
	Payload   []byte
	Status    string
	CreatedAt time.Time
}

type InputType string

const (
	INPUT_TYPE_STRING       InputType = "string"
	INPUT_TYPE_INT          InputType = "int"
	INPUT_TYPE_FLOAT        InputType = "float"
	INPUT_TYPE_BOOL         InputType = "bool"
	INPUT_TYPE_SLICE_STRING InputType = "slice_string"
	INPUT_TYPE_SLICE_INT    InputType = "slice_int"
	INPUT_TYPE_SLICE_UINT   InputType = "slice_uint"
	INPUT_TYPE_SLICE_FLOAT  InputType = "slice_float"
)

type AuthMethod string

const (
	AuthMethodPrivateKey AuthMethod = "private_key"
	AuthMethodPassword   AuthMethod = "password"
)

type ExecResults struct {
	result map[string]string
	err    error
}

type Node struct {
	ID             string
	Name           string
	Hostname       string
	Port           int
	Username       string
	OSFamily       string
	ConnectionType string
	Tags           []string
	Auth           NodeAuth
}

const NodeConnectionTimeout = 5 * time.Second

// CheckConnectivity can be used to check if a remote node is accessible at the given IP:Port
// The default connection timeout is 5 seconds
// Non-nil error is returned if the node is not accessible
func (n *Node) CheckConnectivity() error {
	address := fmt.Sprintf("%s:%d", n.Hostname, n.Port)

	if n.ConnectionType == "qssh" {
		ctx, cancel := context.WithTimeout(context.Background(), NodeConnectionTimeout)
		defer cancel()

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}

		conn, err := quic.DialAddr(ctx, address, tlsConfig, &quic.Config{
			HandshakeIdleTimeout: NodeConnectionTimeout,
		})
		if err != nil {
			return fmt.Errorf("failed to connect to %s via QUIC: %w", address, err)
		}
		defer conn.CloseWithError(0, "connectivity check complete")
		return nil
	}

	conn, err := net.DialTimeout("tcp", address, NodeConnectionTimeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	defer conn.Close()
	return nil
}

type NodeAuth struct {
	CredentialID string
	Method       AuthMethod
	Key          string
}

type Input struct {
	Name        string    `yaml:"name" json:"name" validate:"required,alphanum_underscore"`
	Type        InputType `yaml:"type" json:"type" validate:"required,oneof=string int float bool slice_string slice_int slice_uint slice_float"`
	Label       string    `yaml:"label" json:"label"`
	Description string    `yaml:"description" json:"description"`
	Validation  string    `yaml:"validation" json:"validation"`
	Required    bool      `yaml:"required" json:"required"`
	Default     string    `yaml:"default" json:"default"`
	MaxFileSize int64     `yaml:"max_file_size" json:"max_file_size"`
}

type Action struct {
	ID             string         `yaml:"id" validate:"required,alphanum_underscore"`
	Name           string         `yaml:"name" validate:"required"`
	Executor       string         `yaml:"executor"`
	With           map[string]any `yaml:"with" validate:"required"`
	Approval       bool           `yaml:"approval"`
	Approvers      []string       `yaml:"approvers,omitempty"`
	ApprovalGroups []string       `yaml:"approval_groups,omitempty"`
	Variables      []Variable     `yaml:"variables"`
	On             []Node         `yaml:"on"`
}

type Scheduling struct {
	Cron     string `yaml:"cron" json:"cron"`
	Timezone string `yaml:"timezone" json:"timezone"`
}

type Metadata struct {
	ID          string `yaml:"id" validate:"required,alphanum_underscore"`
	DBID        int32  `yaml:"-"`
	Name        string `yaml:"name" validate:"required"`
	Description string `yaml:"description"`
	SrcDir      string `yaml:"-"`
	Namespace   string `yaml:"namespace"`
	MaxRetries  int    `yaml:"max_retries"`
}

type Variable map[string]any

func (v Variable) Valid() bool {
	return !(len(v) > 1)
}

func (v Variable) Name() string {
	if !v.Valid() {
		return ""
	}

	for k := range v {
		return k
	}
	return ""
}

func (v Variable) Value() string {
	if !v.Valid() {
		return ""
	}

	for _, v := range v {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return ""
}

type Output map[string]any

type NotifyEvent string

const (
	NotifyEventOnSuccess   NotifyEvent = "on_success"
	NotifyEventOnFailure   NotifyEvent = "on_failure"
	NotifyEventOnWaiting   NotifyEvent = "on_waiting"
	NotifyEventOnCancelled NotifyEvent = "on_cancelled"
)

type Notify struct {
	Channel string         `yaml:"channel" json:"channel"`
	Config  map[string]any `yaml:"config" json:"config"`
	Events  []NotifyEvent  `yaml:"events" json:"events"`
}

type Flow struct {
	Meta      Metadata     `yaml:"metadata" validate:"required"`
	Inputs    []Input      `yaml:"inputs" validate:"required"`
	Actions   []Action     `yaml:"actions" validate:"required"`
	Outputs   []Output     `yaml:"outputs"`
	Schedules []Scheduling `yaml:"scheduling"`
	Notify    []Notify     `yaml:"notify"`
}

type FlowExecutionPayload struct {
	Workflow          Flow
	Input             map[string]any
	StartingActionIdx int
	NamespaceID       string
	TriggerType       TriggerType
	UserUUID          string
	FlowDirectory     string

	// Resumed should be set to true if resuming an existing execution (after approval or retry)
	Resumed bool
}

// Hook function types for flow execution
type HookFn func(ctx context.Context, execID string, action Action, namespaceID string) error
type SecretsProviderFn func(ctx context.Context, flowID string, namespaceID string) (map[string]string, error)
type FlowLoaderFn func(ctx context.Context, flowSlug string, namespaceUUID string) (Flow, error)

// TaskQueuer allows handlers to enqueue new tasks
type TaskQueuer interface {
	QueueTask(ctx context.Context, payloadType PayloadType, execID string, payload any) (string, error)
	QueueTaskWithRetries(ctx context.Context, payloadType PayloadType, execID string, payload any, maxRetries int) (string, error)
}

// PayloadType identifies different types of jobs in the queue
type PayloadType string

// Handler processes jobs of a specific payload type
type Handler interface {
	// Type returns the payload type this handler processes
	Type() PayloadType
	// Handle processes a job
	Handle(ctx context.Context, job Job) error
}

// Job represents a job passed to handlers
type Job struct {
	ID          int64
	ExecID      string
	PayloadType PayloadType
	Payload     []byte
	CreatedAt   time.Time
	ScheduledAt time.Time
	MaxRetries  int
	Attempt     int
}

func (j Job) ShouldRetry() bool {
	return j.Attempt < j.MaxRetries
}

// RetryOptions configures exponential backoff for job retries
type RetryOptions struct {
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

func DefaultRetryOptions() RetryOptions {
	return RetryOptions{
		InitialDelay:  15 * time.Second,
		MaxDelay:      5 * time.Minute,
		BackoffFactor: 2.0,
	}
}

// CalculateDelay returns the delay for the given attempt using exponential backoff
func (r RetryOptions) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return r.InitialDelay
	}

	delay := r.InitialDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * r.BackoffFactor)
		if delay > r.MaxDelay {
			return r.MaxDelay
		}
	}
	return delay
}

// QueueWeight defines weight for a payload type
type QueueWeight struct {
	PayloadType PayloadType
	Weight      int // 1-100, all weights must sum to 100
}

// QueueConfig holds the weighted queue configuration
type QueueConfig struct {
	Queues []QueueWeight
}

// Validate ensures queue weights sum to 100
func (c QueueConfig) Validate() error {
	total := 0
	for _, q := range c.Queues {
		if q.Weight < 0 || q.Weight > 100 {
			return fmt.Errorf("weight must be 0-100, got %d", q.Weight)
		}
		total += q.Weight
	}
	if total != 100 {
		return fmt.Errorf("queue weights must sum to 100, got %d", total)
	}
	return nil
}

// GetWorkerCount calculates the number of goroutines for a payload type
func (c QueueConfig) GetWorkerCount(pt PayloadType, totalWorkers int) int {
	for _, q := range c.Queues {
		if q.PayloadType == pt {
			count := (totalWorkers * q.Weight) / 100
			if count < 1 && q.Weight > 0 {
				count = 1
			}
			return count
		}
	}
	return 0
}

// handlerRegistry manages handler registration and lookup
type handlerRegistry struct {
	handlers map[PayloadType]Handler
	mu       sync.RWMutex
}

func newHandlerRegistry() *handlerRegistry {
	return &handlerRegistry{
		handlers: make(map[PayloadType]Handler),
	}
}

// Register adds a handler for a specific payload type
func (r *handlerRegistry) Register(h Handler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[h.Type()]; exists {
		return fmt.Errorf("handler already registered for type: %s", h.Type())
	}

	r.handlers[h.Type()] = h
	return nil
}

// Get retrieves a handler by payload type
func (r *handlerRegistry) Get(pt PayloadType) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[pt]
	return handler, exists
}

// ScheduledJob represents a job that can be scheduled via cron
type ScheduledJob struct {
	ID          string
	Name        string
	Cron        string
	Timezone    string
	PayloadType PayloadType
	Payload     any
}

// JobSyncerFn syncs scheduled jobs from a data source
type JobSyncerFn func(ctx context.Context) ([]ScheduledJob, error)
