package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/cvhariharan/flowctl/sdk/executor"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gosimple/slug"
	"github.com/hashicorp/go-envparse"
	"github.com/invopop/jsonschema"
	"github.com/rs/xid"
	"gopkg.in/yaml.v3"
)

const (
	WORKING_DIR = "/flows"
)

type DockerWithConfig struct {
	Image  string `yaml:"image" json:"image" jsonschema:"title=image,description=Docker Image" jsonschema_extras:"placeholder=docker.io/alpine:latest"`
	Script string `yaml:"script" json:"script" jsonschema:"title=script" jsonschema_extras:"widget=codeeditor"`
}

type DockerExecutor struct {
	name             string
	image            string
	env              []string
	cmd              []string
	entrypoint       []string
	containerID      string
	mounts           []mount.Mount
	dockerOptions    DockerRunnerOptions
	authConfig       string
	stdout           io.Writer
	stderr           io.Writer
	client           *client.Client
	workingDirectory string
	driver           executor.NodeDriver
}

type DockerRunnerOptions struct {
	ShowImagePull     bool
	MountDockerSocket bool
	KeepContainer     bool
}

func init() {
	executor.RegisterExecutor("docker", NewDockerExecutor)
	executor.RegisterSchema("docker", GetSchema())
}

func NewDockerExecutor(name string, driver executor.NodeDriver) (executor.Executor, error) {
	jobName := slug.Make(fmt.Sprintf("%s-%s", name, xid.New().String()))

	executor := &DockerExecutor{
		name:             jobName,
		workingDirectory: driver.GetWorkingDirectory(),
		driver:           driver,
	}

	return executor, nil
}

func GetSchema() interface{} {
	return jsonschema.Reflect(&DockerWithConfig{})
}

func (d *DockerExecutor) withImage(image string) *DockerExecutor {
	d.image = image
	return d
}

func (d *DockerExecutor) withEnv(env []map[string]any) *DockerExecutor {
	variables := make([]string, 0)
	for _, v := range env {
		if len(v) > 1 {
			log.Println("variables should be defined as a key value pair")
			return nil
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

func (d *DockerExecutor) Execute(ctx context.Context, execCtx executor.ExecutionContext) (map[string]string, error) {
	var config DockerWithConfig
	if err := yaml.Unmarshal(execCtx.WithConfig, &config); err != nil {
		return nil, fmt.Errorf("could not read config for docker executor %s: %w", d.name, err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cli, err := d.getDockerClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get docker client: %w", err)
	}
	defer cli.Close()

	d.client = cli

	// create a file for storing output
	tempFile := d.driver.Join(d.driver.TempDir(), fmt.Sprintf("docker-executor-output-%s", xid.New().String()))
	if err := d.driver.CreateFile(ctx, tempFile); err != nil {
		return nil, fmt.Errorf("failed to create temp file for output: %w", err)
	}

	// create artifacts directory
	artifactsDir := d.driver.Join(d.driver.TempDir(), fmt.Sprintf("artifacts-%s", execCtx.ExecID))
	if err := d.driver.CreateDir(ctx, artifactsDir); err != nil {
		return nil, fmt.Errorf("failed to create artifacts directory: %w", err)
	}

	if err := d.driver.CreateDir(ctx, d.workingDirectory); err != nil {
		return nil, fmt.Errorf("failed to create working directory: %w", err)
	}

	d.mounts = append(d.mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: d.workingDirectory,
		Target: WORKING_DIR,
	})

	d.mounts = append(d.mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: tempFile,
		Target: "/tmp/flow/output",
	})

	d.mounts = append(d.mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: artifactsDir,
		Target: "/tmp/flow/artifacts",
	})

	vars := make([]map[string]any, 0)
	for k, v := range execCtx.Inputs {
		vars = append(vars, map[string]any{k: v})
	}
	// Add output env variable
	vars = append(vars, map[string]any{"FC_OUTPUT": "/tmp/flow/output"})
	// Add artifacts env variable
	vars = append(vars, map[string]any{"FC_ARTIFACTS": "/tmp/flow/artifacts"})

	d.withImage(config.Image).
		withCmd([]string{config.Script}).
		withEnv(vars)
	d.stdout = execCtx.Stdout
	d.stderr = execCtx.Stderr

	if err := d.run(ctx); err != nil {
		return nil, err
	}

	outputContents, err := d.readTempFileContents(ctx, tempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read temp file contents: %w", err)
	}

	outputEnv, err := envparse.Parse(outputContents)
	if err != nil {
		return nil, fmt.Errorf("could not load output env: %w", err)
	}

	return outputEnv, nil
}

func (d *DockerExecutor) readTempFileContents(ctx context.Context, tempFile string) (io.Reader, error) {
	localTempFile, err := os.CreateTemp("/tmp", "docker-executor-output-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create local temp file: %w", err)
	}
	defer os.Remove(localTempFile.Name())
	defer localTempFile.Close()

	if err := d.driver.Download(ctx, tempFile, localTempFile.Name()); err != nil {
		return nil, fmt.Errorf("failed to download temp file: %w", err)
	}

	content, err := os.ReadFile(localTempFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read temp file %s: %w", localTempFile.Name(), err)
	}
	return strings.NewReader(string(content)), nil
}

func (d *DockerExecutor) run(ctx context.Context) error {
	if err := d.pullImage(ctx, d.client); err != nil {
		return fmt.Errorf("could not pull image: %w", err)
	}

	resp, err := d.createContainer(ctx, d.client)
	if err != nil {
		return fmt.Errorf("unable to create container: %w", err)
	}
	d.containerID = resp.ID

	// Only schedule removal if KeepContainer is false
	if !d.dockerOptions.KeepContainer {
		defer func() {
			if ctx.Err() == nil {
				if rErr := d.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{}); rErr != nil {
					log.Printf("Error removing container: %v", rErr)
				}
			}
		}()
	}

	if err := d.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("unable to start container: %w", err)
	}

	// Start goroutine to monitor context cancellation and stop container if cancelled
	go func() {
		<-ctx.Done()
		if ctx.Err() != nil {
			// Context was cancelled, stop the container
			stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := d.client.ContainerStop(stopCtx, resp.ID, container.StopOptions{}); err != nil {
				log.Printf("Error stopping container %s after cancellation: %v", resp.ID, err)
			}
		}
	}()

	logs, err := d.client.ContainerLogs(ctx, resp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return fmt.Errorf("unable to get container logs: %w", err)
	}
	defer logs.Close()

	if _, err := stdcopy.StdCopy(d.stdout, d.stderr, logs); err != nil {
		return fmt.Errorf("error copying logs: %w", err)
	}

	statusCh, errCh := d.client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return fmt.Errorf("error waiting for container: %w", err)
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("container exited with code %d", status.StatusCode)
		}
	case <-ctx.Done():
		return ctx.Err()
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
	if !d.driver.IsRemote() {
		return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	}

	localListener, err := d.createSSHTunnel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH tunnel: %w", err)
	}

	dockerHost := "tcp://" + localListener.Addr().String()

	return client.NewClientWithOpts(
		client.WithHost(dockerHost),
		client.WithAPIVersionNegotiation(),
	)
}

func (d *DockerExecutor) createSSHTunnel(ctx context.Context) (net.Listener, error) {
	localListener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to listen on a local port: %w", err)
	}

	go func() {
		remoteConn, err := d.driver.Dial("unix", "/var/run/docker.sock")
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
