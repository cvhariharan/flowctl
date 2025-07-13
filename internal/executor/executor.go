package executor

import (
	"context"
	"io"
)

type Node struct {
	Hostname string
	Port     int
	Username string
	Auth     NodeAuth
}

type NodeAuth struct {
	Method string
	Key    string
}

type ExecutionContext struct {
	// WithConfig is the yaml config passed to the executor
	WithConfig []byte
	Artifacts  []string
	Inputs     map[string]interface{}
	Stdout     io.Writer
	Stderr     io.Writer
	Node       Node
}

type Executor interface {
	Execute(ctx context.Context, execCtx ExecutionContext) (outputs map[string]string, err error)
	PushFile(ctx context.Context, localFilePath string, remoteFilePath string) error
	PullFile(ctx context.Context, remoteFilePath string, localFilePath string) error
	Close() error
}
