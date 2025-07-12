package executor

import (
	"fmt"

	"github.com/melbahja/goph"
)

type SSHClient struct {
	*goph.Client
}

func GetSSHClient(node Node) (*SSHClient, error) {
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
	sc, err := goph.New(node.Username, node.Hostname, auth)
	return &SSHClient{sc}, err
}

func (client *SSHClient) RunCommand(command string) (string, error) {
	session, err := client.NewSession()
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
