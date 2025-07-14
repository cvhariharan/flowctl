package executor

import (
	"fmt"
	"net"

	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
)

// RemoteClient defines an interface for interacting with a remote machine.
// This abstraction allows for swapping the underlying client implementation
type RemoteClient interface {
	// RunCommand executes a command on the remote machine and returns the combined output.
	RunCommand(command string) (string, error)
	// Download copies a file from the remote path to a local path.
	Download(remotePath, localPath string) error
	// Upload copies a file from the local path to a remote path.
	Upload(localPath, remotePath string) error
	// Dial opens a connection to the given network and address on the remote machine.
	Dial(network, address string) (net.Conn, error)
	// Close terminates the connection to the remote machine.
	Close() error
}

// gophClient is an implementation of RemoteClient using the goph library.
type gophClient struct {
	*goph.Client
}

// NewRemoteClient creates a new client for interacting with a remote node based on the
// provided node configuration.
func NewRemoteClient(node Node) (RemoteClient, error) {
	var auth goph.Auth
	var err error

	switch node.Auth.Method {
	case "ssh_key":
		auth, err = goph.RawKey(node.Auth.Key, "")
		if err != nil {
			return nil, fmt.Errorf("failed to use ssh key: %w", err)
		}
	case "password":
		auth = goph.Password(node.Auth.Key)
	default:
		return nil, fmt.Errorf("unsupported auth method: %s", node.Auth.Method)
	}

	client, err := goph.NewConn(
		&goph.Config{
			User:     node.Username,
			Addr:     node.Hostname,
			Port:     uint(node.Port),
			Auth:     auth,
			Callback: ssh.InsecureIgnoreHostKey(),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create goph client: %w", err)
	}

	return &gophClient{client}, nil
}

// RunCommand executes a shell command on the remote host.
func (c *gophClient) RunCommand(command string) (string, error) {
	session, err := c.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return "", fmt.Errorf("failed to run command on remote: %w", err)
	}

	return string(output), nil
}
