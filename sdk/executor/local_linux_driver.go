package executor

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/xid"
)

type LocalLinuxDriver struct {
	workingDirectory string
}

func NewLocalLinux() (NodeDriver, error) {
	l := &LocalLinuxDriver{}
	wd := l.Join(l.TempDir(), fmt.Sprintf("flows-%s", xid.New().String()))
	if err := l.CreateDir(context.Background(), wd); err != nil {
		return nil, err
	}
	l.workingDirectory = wd
	return l, nil
}

func (d *LocalLinuxDriver) Upload(ctx context.Context, localPath, remotePath string) error {
	srcFile, err := os.Open(filepath.Clean(localPath))
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", localPath, err)
	}
	defer srcFile.Close()

	if err := d.CreateDir(ctx, filepath.Dir(remotePath)); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	destFile, err := os.Create(filepath.Clean(remotePath))
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", remotePath, err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file from %s to %s: %w", localPath, remotePath, err)
	}

	return nil
}

func (d *LocalLinuxDriver) GetWorkingDirectory() string {
	return d.workingDirectory
}

func (d *LocalLinuxDriver) Download(ctx context.Context, remotePath, localPath string) error {
	return d.Upload(ctx, remotePath, localPath)
}

func (d *LocalLinuxDriver) CreateDir(ctx context.Context, path string) error {
	return os.MkdirAll(path, 0755)
}

func (d *LocalLinuxDriver) CreateFile(ctx context.Context, path string) error {
	file, err := os.Create(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}
	return file.Close()
}

func (d *LocalLinuxDriver) Remove(ctx context.Context, path string) error {
	return os.RemoveAll(path)
}

func (d *LocalLinuxDriver) SetPermissions(ctx context.Context, path string, perms os.FileMode) error {
	return os.Chmod(path, perms)
}

func (d *LocalLinuxDriver) Exec(ctx context.Context, command string, workingDir string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Dir = workingDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func (d *LocalLinuxDriver) Dial(network, address string) (net.Conn, error) {
	return nil, fmt.Errorf("dial not supported for local execution")
}

func (d *LocalLinuxDriver) IsRemote() bool {
	return false
}

func (d *LocalLinuxDriver) TempDir() string {
	return "/tmp"
}

func (d *LocalLinuxDriver) Join(parts ...string) string {
	return filepath.Join(parts...)
}

func (d *LocalLinuxDriver) Close() error {
	return nil
}
