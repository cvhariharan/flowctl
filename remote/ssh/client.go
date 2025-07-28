package ssh

import (
	"fmt"

	"github.com/cvhariharan/flowctl/sdk/executor"
	"github.com/cvhariharan/flowctl/sdk/remoteclient"
	"github.com/melbahja/goph"
	"golang.org/x/crypto/ssh"
)

// gophClient is an implementation of RemoteClient using the goph library.
type gophClient struct {
	*goph.Client
}

func init() {
	remoteclient.Register("ssh", NewRemoteClient)
}

// NewRemoteClient creates a new client for interacting with a remote node based on the
// provided node configuration.
func NewRemoteClient(node executor.Node) (remoteclient.RemoteClient, error) {
	var auth goph.Auth
	var err error

	switch node.Auth.Method {
	case "private_key":
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
