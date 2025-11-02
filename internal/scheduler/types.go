package scheduler

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
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
	ErrPendingApproval = errors.New("pending approval")
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

// CheckConnectivity can be used to check if a remote node is accessible at the given IP:Port
// The default connection timeout is 5 seconds
// Non-nil error is returned if the node is not accessible
func (n *Node) CheckConnectivity() error {
	address := fmt.Sprintf("%s:%d", n.Hostname, n.Port)

	if n.ConnectionType == "qssh" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}

		conn, err := quic.DialAddr(ctx, address, tlsConfig, &quic.Config{
			HandshakeIdleTimeout: 5 * time.Second,
		})
		if err != nil {
			return fmt.Errorf("failed to connect to %s via QUIC: %w", address, err)
		}
		defer conn.CloseWithError(0, "connectivity check complete")
		return nil
	}

	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
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
}

type Action struct {
	ID        string         `yaml:"id" validate:"required,alphanum_underscore"`
	Name      string         `yaml:"name" validate:"required"`
	Executor  string         `yaml:"executor" validate:"required,oneof=script docker"`
	With      map[string]any `yaml:"with" validate:"required"`
	Approval  bool           `yaml:"approval"`
	Variables []Variable     `yaml:"variables"`
	On        []Node         `yaml:"on"`
}

type Metadata struct {
	ID          string   `yaml:"id" validate:"required,alphanum_underscore"`
	DBID        int32    `yaml:"-"`
	Name        string   `yaml:"name" validate:"required"`
	Description string   `yaml:"description"`
	Schedules   []string `yaml:"schedules"`
	SrcDir      string   `yaml:"-"`
	Namespace   string   `yaml:"namespace"`
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

type Flow struct {
	Meta    Metadata `yaml:"metadata" validate:"required"`
	Inputs  []Input  `yaml:"inputs" validate:"required"`
	Actions []Action `yaml:"actions" validate:"required"`
	Outputs []Output `yaml:"outputs"`
}

type FlowExecutionPayload struct {
	Workflow          Flow
	Input             map[string]interface{}
	StartingActionIdx int
	ExecID            string
	NamespaceID       string
	TriggerType       TriggerType
	UserUUID          string
}

// Hook function types for flow execution
type HookFn func(ctx context.Context, execID string, action Action, namespaceID string) error
type SecretsProviderFn func(ctx context.Context, flowID string, namespaceID string) (map[string]string, error)
type FlowLoaderFn func(ctx context.Context, flowSlug string, namespaceUUID string) (Flow, error)

// SchedulerDependencies contains dependencies needed by the scheduler
type SchedulerDependencies struct {
	OnBeforeAction  HookFn
	OnAfterAction   HookFn
	SecretsProvider SecretsProviderFn
	FlowLoader      FlowLoaderFn
	LogManager      interface{} // streamlogger.LogManager
}
