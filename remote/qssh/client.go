package qssh

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/cvhariharan/autopilot/sdk/executor"
	"github.com/cvhariharan/autopilot/sdk/remoteclient"
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

func NewRemoteClient(node executor.Node) (remoteclient.RemoteClient, error) {
	var qconfig qssh.Config

	switch node.Auth.Method {
	case "private_key":
		privateKey, err := ssh.ParsePrivateKey([]byte(node.Auth.Key))
		if err != nil {
			return nil, fmt.Errorf("could not parse private key for node %s: %w", node.Hostname, err)
		}
		qconfig = qssh.KeyConfig(node.Username, privateKey)
	case "password":
		qconfig = qssh.PasswordConfig(node.Username, node.Auth.Key)
	default:
		return nil, fmt.Errorf("unsupported auth method: %s", node.Auth.Method)
	}

	client, conn, err := qssh.Dial(fmt.Sprintf("%s:%d", node.Hostname, node.Port), qconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s:%s: %w", node.Hostname, node.Port, err)
	}
	return &qsshClient{
		sshClient: client,
		conn:      conn,
	}, nil
}

func (q *qsshClient) RunCommand(command string) (string, error) {
	session, err := q.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("could not create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return "", fmt.Errorf("could not run command: %w", err)
	}

	return string(output), err
}

func (q *qsshClient) Download(remotePath, localPath string) error {
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

func (q *qsshClient) Upload(localPath, remotePath string) error {
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
