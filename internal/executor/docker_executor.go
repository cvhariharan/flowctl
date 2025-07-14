package executor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"path/filepath"
	"strings"

	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gosimple/slug"
	"github.com/hashicorp/go-envparse"
	"github.com/rs/xid"
	"gopkg.in/yaml.v3"
)

const (
	WORKING_DIR = "/app"
	PUSH_DIR    = "/push"
	PULL_DIR    = "/pull"
)

type DockerWithConfig struct {
	Image  string `yaml:"image"`
	Script string `yaml:"script"`
}

type DockerExecutor struct {
	name               string
	image              string
	env                []string
	cmd                []string
	entrypoint         []string
	containerID        string
	workingDirectory   string
	mounts             []mount.Mount
	dockerOptions      DockerRunnerOptions
	authConfig         string
	stdout             io.Writer
	stderr             io.Writer
	client             *client.Client
	remoteClient       RemoteClient
	artifactsDirectory string
}

type DockerRunnerOptions struct {
	ShowImagePull     bool
	MountDockerSocket bool
	KeepContainer     bool
}

func NewDockerExecutor(name string, dockerOptions DockerRunnerOptions) (Executor, error) {
	jobName := slug.Make(fmt.Sprintf("%s-%s", name, xid.New().String()))

	return &DockerExecutor{
		name:          jobName,
		dockerOptions: dockerOptions,
	}, nil
}

func (d *DockerExecutor) withImage(image string) *DockerExecutor {
	d.image = image
	return d
}

func (d *DockerExecutor) withEnv(env []map[string]any) *DockerExecutor {
	variables := make([]string, 0)
	for _, v := range env {
		if len(v) > 1 {
			log.Fatal("variables should be defined as a key value pair")
		}
		for k, v := range v {
			variables = append(variables, fmt.Sprintf("%s=%s", k, fmt.Sprint(v)))
		}
	}
	d.env = variables
	return d
}

func (d *DockerExecutor) withCmd(cmd []string) *DockerExecutor {
	d.cmd = cmd
	return d
}

func (d *DockerExecutor) withEntrypoint(entrypoint []string) *DockerExecutor {
	d.entrypoint = entrypoint
	return d
}

func (d *DockerExecutor) withCredentials(username, password string) *DockerExecutor {
	authConfig := registry.AuthConfig{
		Username: username,
		Password: password,
	}

	jsonVal, err := json.Marshal(authConfig)
	if err != nil {
		log.Fatal("could not create auth config for docker authentication: ", err)
	}
	d.authConfig = base64.URLEncoding.EncodeToString(jsonVal)
	return d
}

func (d *DockerExecutor) Execute(ctx context.Context, execCtx ExecutionContext) (map[string]string, error) {
	var config DockerWithConfig
	if err := yaml.Unmarshal(execCtx.WithConfig, &config); err != nil {
		return nil, fmt.Errorf("could not read config for docker executor %s: %w", d.name, err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var remoteClient RemoteClient
	if execCtx.Node.Hostname != "" {
		var err error
		remoteClient, err = NewRemoteClient(execCtx.Node)
		if err != nil {
			return nil, fmt.Errorf("failed to get remote client: %w", err)
		}
	}
	d.remoteClient = remoteClient

	cli, err := d.getDockerClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get docker client: %w", err)
	}
	defer cli.Close()

	d.client = cli

	// create a file for storing output
	tempFile := fmt.Sprintf("/tmp/docker-executor-output-%s", xid.New().String())
	err = d.createFileOrDirectory(tempFile, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for output: %w", err)
	}

	// create artifacts directory
	d.artifactsDirectory = fmt.Sprintf("/tmp/docker-artifacts-%s", xid.New().String())
	if err := d.createFileOrDirectory(filepath.Join(d.artifactsDirectory, PUSH_DIR), true); err != nil {
		return nil, fmt.Errorf("failed to create artifacts directory: %w", err)
	}
	if err := d.createFileOrDirectory(filepath.Join(d.artifactsDirectory, PULL_DIR), true); err != nil {
		return nil, fmt.Errorf("failed to create artifacts directory: %w", err)
	}

	d.mounts = append(d.mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: tempFile,
		Target: "/tmp/flow/output",
	})

	vars := make([]map[string]any, 0)
	for k, v := range execCtx.Inputs {
		vars = append(vars, map[string]any{k: v})
	}
	// Add output env variable
	vars = append(vars, map[string]any{"OUTPUT": "/tmp/flow/output"})

	d.withImage(config.Image).
		withCmd([]string{config.Script}).
		withEnv(vars)
	d.stdout = execCtx.Stdout
	d.stderr = execCtx.Stderr

	if err := d.run(ctx, execCtx); err != nil {
		return nil, err
	}

	outputContents, err := d.readTempFileContents(tempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read temp file contents: %w", err)
	}

	outputEnv, err := envparse.Parse(outputContents)
	if err != nil {
		return nil, fmt.Errorf("could not load output env: %w", err)
	}

	return outputEnv, nil
}

func (d *DockerExecutor) readTempFileContents(tempFile string) (io.Reader, error) {
	readFile := func(filePath string) (io.Reader, error) {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read temp file %s: %w", filePath, err)
		}
		return strings.NewReader(string(content)), nil
	}

	if d.remoteClient != nil {
		// For remote execution, download the file using the remote client
		localTempFile, err := os.CreateTemp("/tmp", "docker-executor-output-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create local temp file: %w", err)
		}
		defer os.Remove(localTempFile.Name())
		defer localTempFile.Close()

		if err := d.remoteClient.Download(tempFile, localTempFile.Name()); err != nil {
			return nil, fmt.Errorf("failed to download temp file from remote: %w", err)
		}

		return readFile(localTempFile.Name())
	} else {
		// For local execution, read the file directly
		return readFile(tempFile)
	}
}

// The run, createSrcDirectories, pullImage, and createContainer
func (d *DockerExecutor) run(ctx context.Context, execCtx ExecutionContext) error {
	if err := d.pullImage(ctx, d.client); err != nil {
		return fmt.Errorf("could not pull image: %v", err)
	}

	resp, err := d.createContainer(ctx, d.client)
	if err != nil {
		return fmt.Errorf("unable to create container: %v", err)
	}
	d.containerID = resp.ID

	// Only schedule removal if KeepContainer is false
	if !d.dockerOptions.KeepContainer {
		defer func() {
			if rErr := d.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{}); rErr != nil {
				log.Printf("Error removing container: %v", rErr)
			}
		}()
	}

	// Copy the artifacts directory to the container
	if err := d.copyArtifactsToContainer(ctx, resp.ID); err != nil {
		return fmt.Errorf("unable to copy artifacts to container: %v", err)
	}

	if err := d.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("unable to start container: %v", err)
	}

	logs, err := d.client.ContainerLogs(ctx, resp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return fmt.Errorf("unable to get container logs: %v", err)
	}
	defer logs.Close()

	if _, err := stdcopy.StdCopy(d.stdout, d.stderr, logs); err != nil {
		return fmt.Errorf("error copying logs: %v", err)
	}

	statusCh, errCh := d.client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return fmt.Errorf("error waiting for container: %v", err)
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("container exited with code %d", status.StatusCode)
		}
	}

	// Retrieve artifacts from the container
	if err := d.getArtifactsFromContainer(ctx, resp.ID, execCtx.Artifacts); err != nil {
		return fmt.Errorf("unable to retrieve artifacts from container: %v", err)
	}

	return nil
}

func (d *DockerExecutor) copyArtifactsToContainer(ctx context.Context, containerID string) error {
	if d.remoteClient == nil {
		// Local execution - existing logic
		tar, err := archive.TarWithOptions(filepath.Join(d.artifactsDirectory, PUSH_DIR), &archive.TarOptions{})
		if err != nil {
			return err
		}

		if err := d.client.CopyToContainer(ctx, containerID, WORKING_DIR, tar, container.CopyToContainerOptions{}); err != nil {
			return fmt.Errorf("unable to copy artifacts directory to container: %v", err)
		}
		return nil
	}

	// Remote execution - execute docker cp directly on remote machine
	pushDir := filepath.Join(d.artifactsDirectory, PUSH_DIR)

	// Use docker cp to copy from remote filesystem to container
	// First, create a tar and pipe it directly to docker cp
	dockerCpCmd := fmt.Sprintf("cd %s && tar -c . | docker cp - %s:%s", pushDir, containerID, WORKING_DIR)

	if _, err := d.remoteClient.RunCommand(dockerCpCmd); err != nil {
		return fmt.Errorf("failed to copy artifacts to container via docker cp: %v", err)
	}

	return nil
}

func (d *DockerExecutor) getArtifactsFromContainer(ctx context.Context, containerID string, artifacts []string) error {
	for _, f := range artifacts {
		if d.remoteClient == nil {
			// Local execution - existing logic
			tar, _, err := d.client.CopyFromContainer(ctx, containerID, filepath.Join(WORKING_DIR, filepath.Clean(f)))
			if err != nil {
				return fmt.Errorf("unable to copy artifact %s from container: %v", f, err)
			}
			defer tar.Close()

			if err := d.createFileOrDirectory(filepath.Join(d.artifactsDirectory, PULL_DIR, f), false); err != nil {
				return fmt.Errorf("unable to create artifact file %s: %v", f, err)
			}

			if err := archive.Untar(tar, filepath.Join(d.artifactsDirectory, PULL_DIR), &archive.TarOptions{
				NoLchown: true,
			}); err != nil {
				return fmt.Errorf("unable to untar artifact %s: %v", f, err)
			}
		} else {
			// Remote execution - execute docker cp directly on remote machine
			artifactDir := filepath.Join(d.artifactsDirectory, PULL_DIR, filepath.Dir(f))
			if err := d.createFileOrDirectory(artifactDir, true); err != nil {
				return fmt.Errorf("unable to create artifact directory %s: %v", artifactDir, err)
			}

			// Use docker cp to copy from container to remote filesystem
			containerPath := filepath.Join(WORKING_DIR, filepath.Clean(f))
			remotePath := filepath.Join(d.artifactsDirectory, PULL_DIR, f)

			dockerCpCmd := fmt.Sprintf("docker cp %s:%s - | tar -xf - -C %s",
				containerID, containerPath, filepath.Dir(remotePath))

			if _, err := d.remoteClient.RunCommand(dockerCpCmd); err != nil {
				return fmt.Errorf("failed to copy artifact %s from container via docker cp: %v", f, err)
			}
		}
	}

	return nil
}

// createFileOrDirectory creates a temporary directory or file, handling local and remote execution.
func (d *DockerExecutor) createFileOrDirectory(name string, dir bool) error {
	if d.remoteClient == nil {
		if dir {
			return os.MkdirAll(name, 0755)
		}
		_, err := os.Create(name)
		if err != nil {
			return err
		}
		return nil
	}

	cmd := "touch %s && chmod 755 %s"
	if dir {
		cmd = "mkdir -p %s && chmod 755 %s"
	}
	cmd = fmt.Sprintf(cmd, name, name)
	_, err := d.remoteClient.RunCommand(cmd)
	if err != nil {
		return err
	}
	return nil
}

func (d *DockerExecutor) pullImage(ctx context.Context, cli *client.Client) error {
	reader, err := cli.ImagePull(ctx, d.image, image.PullOptions{RegistryAuth: d.authConfig})
	if err != nil {
		return err
	}
	defer reader.Close()

	imageLogs := io.Discard
	if d.dockerOptions.ShowImagePull {
		imageLogs = d.stdout
	}
	if d.stdout == nil {
		imageLogs = os.Stdout
	}
	if _, err := io.Copy(imageLogs, reader); err != nil {
		return err
	}

	return nil
}

func (d *DockerExecutor) createContainer(ctx context.Context, cli *client.Client) (container.CreateResponse, error) {
	commandScript := strings.Join(d.cmd, "\n")
	cmd := []string{"/bin/sh", "-c", commandScript}
	if len(d.entrypoint) > 0 {
		cmd = []string{commandScript}
	}

	if d.dockerOptions.MountDockerSocket {
		d.mounts = append(d.mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: "/var/run/docker.sock",
			Target: "/var/run/docker.sock",
		})
	}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:      d.image,
		Env:        d.env,
		Entrypoint: d.entrypoint,
		Cmd:        cmd,
		WorkingDir: WORKING_DIR,
	}, &container.HostConfig{
		Mounts:      d.mounts,
		SecurityOpt: []string{"label=disable"},
	}, nil, nil, d.name)
	if err != nil {
		return container.CreateResponse{}, err
	}
	return resp, nil
}

func (d *DockerExecutor) getDockerClient(ctx context.Context) (*client.Client, error) {
	if d.remoteClient == nil {
		// No remote client means local execution
		return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	}

	// Remote execution: create a tunnel
	localListener, err := createSSHTunnel(ctx, d.remoteClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH tunnel: %w", err)
	}

	dockerHost := "tcp://" + localListener.Addr().String()

	return client.NewClientWithOpts(
		client.WithHost(dockerHost),
		client.WithAPIVersionNegotiation(),
	)
}

func createSSHTunnel(ctx context.Context, client RemoteClient) (net.Listener, error) {
	localListener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to listen on a local port: %w", err)
	}

	go func() {
		// Use the Dial method from our interface to connect to the remote Docker socket
		remoteConn, err := client.Dial("unix", "/var/run/docker.sock")
		if err != nil {
			log.Printf("failed to dial remote Docker socket: %s", err)
			return
		}
		defer remoteConn.Close()
		defer localListener.Close()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				localConn, err := localListener.Accept()
				if err != nil {
					continue
				}
				defer localConn.Close()

				go func() {
					io.Copy(localConn, remoteConn)
				}()
				io.Copy(remoteConn, localConn)
			}
		}
	}()

	return localListener, nil
}

// PushFile uploads a file from the local machine to the remote Docker container.
// This should be used before calling the `Execute` method to ensure that the file is available in the container's context.
// remoteFilePath is the path inside the container where the file should be uploaded
func (d *DockerExecutor) PushFile(ctx context.Context, localFilePath string, remoteFilePath string) error {
	destPath := filepath.Join(d.artifactsDirectory, PUSH_DIR, remoteFilePath)
	srcFile, err := os.Open(filepath.Clean(localFilePath))
	if err != nil {
		return fmt.Errorf("failed to open local file %s: %w", localFilePath, err)
	}
	defer srcFile.Close()
	if d.remoteClient == nil {
		destFile, err := os.Create(filepath.Clean(destPath))
		if err != nil {
			return fmt.Errorf("failed to create destination file %s: %w", destPath, err)
		}
		defer destFile.Close()

		if _, err := io.Copy(destFile, srcFile); err != nil {
			return fmt.Errorf("failed to copy file from %s to %s: %w", localFilePath, destPath, err)
		}
		return nil
	}

	// Upload the file to the remote machine at the destination path
	if err := d.remoteClient.Upload(localFilePath, destPath); err != nil {
		return fmt.Errorf("failed to upload file %s to remote path %s: %w", localFilePath, destPath, err)
	}
	return nil
}

// Pullfile is used to download files that are declared as artifacts.
// This should be used after the `Execute` method has been called to retrieve files generated by the Docker container.
// remoteFilePath is the path inside the container where the file is located
func (d *DockerExecutor) PullFile(ctx context.Context, remoteFilePath string, localFilePath string) error {
	srcFile := filepath.Join(d.artifactsDirectory, PULL_DIR, remoteFilePath)
	destFile, err := os.Create(filepath.Clean(localFilePath))
	if err != nil {
		return fmt.Errorf("failed to create local file %s: %w", localFilePath, err)
	}
	defer destFile.Close()

	if d.remoteClient == nil {
		srcFile, err := os.Open(filepath.Clean(srcFile))
		if err != nil {
			return fmt.Errorf("failed to open source file: %w", err)
		}
		defer srcFile.Close()

		if _, err := io.Copy(destFile, srcFile); err != nil {
			return fmt.Errorf("failed to copy file from %s to %s: %w", srcFile.Name(), localFilePath, err)
		}
		return nil
	}

	// Download the file from the remote machine to the local path
	if err := d.remoteClient.Download(srcFile, localFilePath); err != nil {
		return fmt.Errorf("failed to download file from remote path %s to local path %s: %w", srcFile, localFilePath, err)
	}
	return nil
}

func (d *DockerExecutor) Close() error {
	if d.remoteClient != nil {
		return d.remoteClient.Close()
	}

	return nil
}
