package executor

import (
	"context"
	"io"
)

type Node struct {
	Hostname       string
	Port           int
	Username       string
	Auth           NodeAuth
	ConnectionType string
	OSFamily       string
}

type NodeAuth struct {
	Method string
	Key    string
}

type ExecutionContext struct {
	// WithConfig is the yaml config passed to the executor
	WithConfig    []byte
	Inputs        map[string]any
	Stdout        io.Writer
	Stderr        io.Writer
	UserUUID      string
	NamespaceName string // human-readable namespace name for API calls
	APIKey        string // executor API key for authenticating with the server
	APIBaseURL    string // server base URL for API calls
	ExecID        string // unique ID for this execution
	FlowID        string // ID of the flow being executed
	FlowName      string // human-readable flow name
	ActionID      string // ID of the current action being executed
	ActionName    string // human-readable action name
	// Nodes contains all target nodes for this action. Executors that handle
	// node dispatch internally can use this list
	Nodes []Node
}

type Capability uint64

const (
	RemoteExecution Capability = 1 << iota
	EnvironmentVariables
	FileTransfer
	StreamingOutput
	NodeDispatch
)

var capabilityNames = []struct {
	cap  Capability
	name string
}{
	{RemoteExecution, "remote_execution"},
	{EnvironmentVariables, "environment_variables"},
	{FileTransfer, "file_transfer"},
	{StreamingOutput, "streaming_output"},
	{NodeDispatch, "node_dispatch"},
}

func CapabilityStrings(c Capability) []string {
	names := make([]string, 0)
	for _, entry := range capabilityNames {
		if c&entry.cap != 0 {
			names = append(names, entry.name)
		}
	}
	return names
}

type ExecutionResult struct {
	Outputs map[string]string
	Globals map[string]string
}

type Executor interface {
	Execute(ctx context.Context, execCtx ExecutionContext) (ExecutionResult, error)
	GetArtifactsDir() string
	Close() error
}
