package ssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/cvhariharan/flowctl/sdk/remoteclient"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// sshClient is an implementation of RemoteClient using the native SSH library.
type sshClient struct {
	client *ssh.Client
}

func init() {
	remoteclient.Register("ssh", NewRemoteClient)
}

// NewRemoteClient creates a new client for interacting with a remote node based on the
// provided node configuration.
func NewRemoteClient(config remoteclient.NodeConfig) (remoteclient.RemoteClient, error) {
	var authMethod ssh.AuthMethod
	var err error

	switch config.Auth.Method {
	case "private_key":
		signer, err := ssh.ParsePrivateKey([]byte(config.Auth.Key))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethod = ssh.PublicKeys(signer)
	case "password":
		authMethod = ssh.Password(config.Auth.Key)
	default:
		return nil, fmt.Errorf("unsupported auth method: %s", config.Auth.Method)
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.Username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%d", config.Hostname, config.Port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create ssh client: %w", err)
	}

	return &sshClient{client: client}, nil
}

// Close closes the SSH client connection
func (c *sshClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// Dial opens a connection to the given network and address on the remote machine.
func (c *sshClient) Dial(network, address string) (net.Conn, error) {
	return c.client.Dial(network, address)
}

// RunCommand executes a shell command on the remote host
func (c *sshClient) RunCommand(ctx context.Context, command string, stdout, stderr io.Writer) error {
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	session.Stdout = stdout
	session.Stderr = stderr

	type result struct {
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		err := session.Run(command)
		resultCh <- result{err}
	}()

	select {
	case <-ctx.Done():
		// Context was cancelled, close the session to interrupt the command
		session.Close()
		return ctx.Err()
	case res := <-resultCh:
		if res.err != nil {
			return fmt.Errorf("failed to run command on remote: %w", res.err)
		}
		return nil
	}
}

// Download copies a file from the remote path to a local path with context cancellation support.
func (c *sshClient) Download(ctx context.Context, remotePath, localPath string) error {
	// Create a channel to receive the result
	type result struct {
		err error
	}
	resultCh := make(chan result, 1)

	// Run download in goroutine
	go func() {
		err := c.downloadFile(remotePath, localPath)
		resultCh <- result{err}
	}()

	// Wait for either context cancellation or download completion
	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-resultCh:
		if res.err != nil {
			return fmt.Errorf("failed to download file from remote: %w", res.err)
		}
		return nil
	}
}

// Upload copies a file from the local path to a remote path with context cancellation support.
func (c *sshClient) Upload(ctx context.Context, localPath, remotePath string) error {
	// Create a channel to receive the result
	type result struct {
		err error
	}
	resultCh := make(chan result, 1)

	// Run upload in goroutine
	go func() {
		err := c.uploadFile(localPath, remotePath)
		resultCh <- result{err}
	}()

	// Wait for either context cancellation or upload completion
	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-resultCh:
		if res.err != nil {
			return fmt.Errorf("failed to upload file to remote: %w", res.err)
		}
		return nil
	}
}

// downloadFile implements SFTP download using native SSH
func (c *sshClient) downloadFile(remotePath, localPath string) error {
	sftpClient, err := sftp.NewClient(c.client)
	if err != nil {
		return fmt.Errorf("could not create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Create local directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("could not create local directory: %w", err)
	}

	// Open remote file
	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("could not open remote file: %w", err)
	}
	defer remoteFile.Close()

	// Create local file
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("could not create local file: %w", err)
	}
	defer localFile.Close()

	// Copy file
	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("could not copy file: %w", err)
	}

	return nil
}

// uploadFile implements SFTP upload using native SSH
func (c *sshClient) uploadFile(localPath, remotePath string) error {
	sftpClient, err := sftp.NewClient(c.client)
	if err != nil {
		return fmt.Errorf("could not create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Create remote directory if it doesn't exist
	remoteDir := filepath.Dir(remotePath)
	if remoteDir != "." && remoteDir != "/" {
		if err := sftpClient.MkdirAll(remoteDir); err != nil {
			return fmt.Errorf("could not create remote directory: %w", err)
		}
	}

	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("could not open local file: %w", err)
	}
	defer localFile.Close()

	// Create remote file
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("could not create remote file: %w", err)
	}
	defer remoteFile.Close()

	// Copy file
	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("could not copy file: %w", err)
	}

	return nil
}
