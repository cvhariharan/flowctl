package executor

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"

	"github.com/cvhariharan/flowctl/sdk/remoteclient"
)

type NodeDriver interface {
	Upload(ctx context.Context, localPath, remotePath string) error
	Download(ctx context.Context, remotePath, localPath string) error
	CreateDir(ctx context.Context, path string) error
	CreateFile(ctx context.Context, path string) error
	GetWorkingDirectory() string
	Remove(ctx context.Context, path string) error
	SetPermissions(ctx context.Context, path string, perms os.FileMode) error
	Exec(ctx context.Context, command string, workingDir string, stdout, stderr io.Writer) error
	Dial(network, address string) (net.Conn, error)
	IsRemote() bool
	TempDir() string
	Join(parts ...string) string
	Close() error
}

func NewNodeDriver(ctx context.Context, node Node) (NodeDriver, error) {
	if node.Hostname == "" {
		if runtime.GOOS == "windows" {
			return nil, fmt.Errorf("windows local execution not yet supported")
		}
		return NewLocalLinux()
	}

	remoteClient, err := remoteclient.GetClient(node.ConnectionType, remoteclient.NodeConfig{
		Hostname: node.Hostname,
		Port:     node.Port,
		Username: node.Username,
		Auth: remoteclient.NodeAuth{
			Method: node.Auth.Method,
			Key:    node.Auth.Key,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create remote client: %w", err)
	}

	if node.OSFamily == "windows" {
		remoteClient.Close()
		return nil, fmt.Errorf("windows remote execution not yet supported")
	}

	return NewRemoteLinux(remoteClient)
}
