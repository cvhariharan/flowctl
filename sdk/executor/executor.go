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
	WithConfig []byte
	Artifacts  []string
	Inputs     map[string]any
	Stdout     io.Writer
	Stderr     io.Writer
}

type Executor interface {
	Execute(ctx context.Context, execCtx ExecutionContext) (outputs map[string]string, err error)
}
