package qssh

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/cvhariharan/flowctl/sdk/remoteclient"
	"github.com/cvhariharan/qssh"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type qsshClient struct {
	sshClient *ssh.Client
	conn      *qssh.QSSHConnection
}

func init() {
	remoteclient.Register("qssh", NewRemoteClient)
}

func NewRemoteClient(config remoteclient.NodeConfig) (remoteclient.RemoteClient, error) {
	var qconfig qssh.Config

	switch config.Auth.Method {
	case "private_key":
		privateKey, err := ssh.ParsePrivateKey([]byte(config.Auth.Key))
		if err != nil {
			return nil, fmt.Errorf("could not parse private key for node %s: %w", config.Hostname, err)
		}
		qconfig = qssh.KeyConfig(config.Username, privateKey)
	case "password":
		qconfig = qssh.PasswordConfig(config.Username, config.Auth.Key)
	default:
		return nil, fmt.Errorf("unsupported auth method: %s", config.Auth.Method)
	}

	client, conn, err := qssh.Dial(fmt.Sprintf("%s:%d", config.Hostname, config.Port), qconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s:%d: %w", config.Hostname, config.Port, err)
	}
	return &qsshClient{
		sshClient: client,
		conn:      conn,
	}, nil
}

func (q *qsshClient) RunCommand(ctx context.Context, command string, stdout, stderr io.Writer) error {
	session, err := q.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("could not create session: %w", err)
	}
	defer session.Close()

	// Set stdout and stderr writers
	session.Stdout = stdout
	session.Stderr = stderr

	// Create a channel to receive the result
	type result struct {
		err error
	}
	resultCh := make(chan result, 1)

	// Run command in goroutine
	go func() {
		err := session.Run(command)
		resultCh <- result{err}
	}()

	// Wait for either context cancellation or command completion
	select {
	case <-ctx.Done():
		// Context was cancelled, close the session to interrupt the command
		session.Close()
		return ctx.Err()
	case res := <-resultCh:
		if res.err != nil {
			return fmt.Errorf("could not run command: %w", res.err)
		}
		return nil
	}
}

func (q *qsshClient) Download(ctx context.Context, remotePath, localPath string) error {
	// Create a channel to receive the result
	type result struct {
		err error
	}
	resultCh := make(chan result, 1)

	// Run download in goroutine
	go func() {
		err := q.downloadFile(remotePath, localPath)
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

// downloadFile implements the actual SFTP download
func (q *qsshClient) downloadFile(remotePath, localPath string) error {
	sftpClient, err := sftp.NewClient(q.sshClient)
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

func (q *qsshClient) Upload(ctx context.Context, localPath, remotePath string) error {
	// Create a channel to receive the result
	type result struct {
		err error
	}
	resultCh := make(chan result, 1)

	// Run upload in goroutine
	go func() {
		err := q.uploadFile(localPath, remotePath)
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

// uploadFile implements the actual SFTP upload
func (q *qsshClient) uploadFile(localPath, remotePath string) error {
	sftpClient, err := sftp.NewClient(q.sshClient)
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

func (q *qsshClient) Dial(network, address string) (net.Conn, error) {
	// Use SSH client's Dial method for port forwarding
	conn, err := q.sshClient.Dial(network, address)
	if err != nil {
		return nil, fmt.Errorf("could not dial %s://%s: %w", network, address, err)
	}
	return conn, nil
}

func (q *qsshClient) Close() error {
	if err := q.sshClient.Close(); err != nil {
		return err
	}
	return q.conn.Close()
}
