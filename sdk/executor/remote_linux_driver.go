package executor

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cvhariharan/flowctl/sdk/remoteclient"
	"github.com/rs/xid"
)

type RemoteLinuxDriver struct {
	client           remoteclient.RemoteClient
	workingDirectory string
}

func NewRemoteLinux(client remoteclient.RemoteClient) (NodeDriver, error) {
	r := &RemoteLinuxDriver{
		client: client,
	}
	wd := r.Join(r.TempDir(), fmt.Sprintf("flows-%s", xid.New().String()))
	if err := r.CreateDir(context.Background(), wd); err != nil {
		return nil, err
	}
	r.workingDirectory = wd
	return r, nil
}

func (d *RemoteLinuxDriver) GetWorkingDirectory() string {
	return d.workingDirectory
}

func (d *RemoteLinuxDriver) Upload(ctx context.Context, localPath, remotePath string) error {
	if err := d.CreateDir(ctx, filepath.Dir(remotePath)); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	return d.client.Upload(ctx, localPath, remotePath)
}

func (d *RemoteLinuxDriver) Download(ctx context.Context, remotePath, localPath string) error {
	return d.client.Download(ctx, remotePath, localPath)
}

func (d *RemoteLinuxDriver) CreateDir(ctx context.Context, dirPath string) error {
	cmd := fmt.Sprintf("mkdir -p %s", dirPath)
	return d.client.RunCommand(ctx, cmd, io.Discard, io.Discard)
}

func (d *RemoteLinuxDriver) CreateFile(ctx context.Context, filePath string) error {
	cmd := fmt.Sprintf("touch %s", filePath)
	return d.client.RunCommand(ctx, cmd, io.Discard, io.Discard)
}

func (d *RemoteLinuxDriver) Remove(ctx context.Context, filePath string) error {
	cmd := fmt.Sprintf("rm -rf %s", filePath)
	return d.client.RunCommand(ctx, cmd, io.Discard, io.Discard)
}

func (d *RemoteLinuxDriver) SetPermissions(ctx context.Context, filePath string, perms os.FileMode) error {
	cmd := fmt.Sprintf("chmod %o %s", perms, filePath)
	return d.client.RunCommand(ctx, cmd, io.Discard, io.Discard)
}

func (d *RemoteLinuxDriver) Exec(ctx context.Context, command string, workingDir string, env []string, stdout, stderr io.Writer) error {
	var parts []string

	// Add environment variable exports
	for _, envVar := range env {
		parts = append(parts, fmt.Sprintf("export %s", envVar))
	}

	// Add working directory change if needed
	if workingDir != "" {
		parts = append(parts, fmt.Sprintf("cd %s", workingDir))
	}

	// Add the actual command
	parts = append(parts, command)

	// Join with &&
	fullCommand := strings.Join(parts, " && ")

	return d.client.RunCommand(ctx, fullCommand, stdout, stderr)
}

func (d *RemoteLinuxDriver) Dial(network, address string) (net.Conn, error) {
	return d.client.Dial(network, address)
}

func (d *RemoteLinuxDriver) IsRemote() bool {
	return true
}

func (d *RemoteLinuxDriver) TempDir() string {
	return "/tmp"
}

func (d *RemoteLinuxDriver) Join(parts ...string) string {
	return path.Join(parts...)
}

func (d *RemoteLinuxDriver) ListFiles(ctx context.Context, dirPath string) ([]string, error) {
	var output strings.Builder

	// Use find command to list only top-level files (not directories) relative to dirPath
	cmd := fmt.Sprintf("cd %s && find . -maxdepth 1 -type f -printf '%%P\\n' 2>/dev/null || true", dirPath)

	if err := d.client.RunCommand(ctx, cmd, &output, io.Discard); err != nil {
		return nil, fmt.Errorf("failed to list files in %s: %w", dirPath, err)
	}

	files := strings.Split(strings.TrimSpace(output.String()), "\n")

	// Filter out empty strings
	var result []string
	for _, file := range files {
		if file != "" {
			result = append(result, file)
		}
	}

	return result, nil
}

func (d *RemoteLinuxDriver) Close() error {
	return d.client.Close()
}
