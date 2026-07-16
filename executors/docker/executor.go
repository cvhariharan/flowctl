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
	WORKING_DIR  = "/flows"
	ArtifactsDir = "/tmp/flow/artifacts"
)

type DockerWithConfig struct {
	Image       string `yaml:"image" json:"image" jsonschema:"title=image,description=Docker Image" jsonschema_extras:"placeholder=docker.io/alpine:latest"`
	Script      string `yaml:"script" json:"script" jsonschema:"title=script" jsonschema_extras:"widget=codeeditor"`
	Interpreter string `yaml:"interpreter,omitempty" json:"interpreter,omitempty" jsonschema:"title=interpreter,description=Shell interpreter to use (default: /bin/sh)" jsonschema_extras:"placeholder=/bin/sh"`
	Extension   string `yaml:"extension,omitempty" json:"extension,omitempty" jsonschema:"title=extension,description=File extension for the script (default: .sh)" jsonschema_extras:"placeholder=.sh"`
}

type DockerExecutor struct {
	name             string
	image            string
	env              []string
	script           string
	entrypoint       []string
	interpreter      string
	containerID      string
	mounts           []mount.Mount
	dockerOptions    DockerRunnerOptions
	authConfig       string
	stdout           io.Writer
	stderr           io.Writer
	client           *client.Client
	workingDirectory string
	driver           executor.NodeDriver
	execID           string
}

type DockerRunnerOptions struct {
	ShowImagePull     bool
	MountDockerSocket bool
	KeepContainer     bool
}

func NewDockerExecutor(name string, node executor.Node, execID string) (executor.Executor, error) {
	jobName := slug.Make(fmt.Sprintf("%s-%s", name, xid.New().String()))

	driver, err := executor.NewNodeDriver(context.Background(), node)
	if err != nil {
		return nil, fmt.Errorf("failed to create node driver: %w", err)
	}

	exec := &DockerExecutor{
		name:             jobName,
		workingDirectory: driver.GetWorkingDirectory(),
		driver:           driver,
		execID:           execID,
	}

	return exec, nil
}

func (d *DockerExecutor) GetArtifactsDir() string {
	return ArtifactsDir
}

func (d *DockerExecutor) Close() error {
	return d.driver.Close()
}

func GetCapabilities() executor.Capability {
	return executor.RemoteExecution | executor.EnvironmentVariables | executor.FileTransfer | executor.StreamingOutput
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

func (d *DockerExecutor) withScript(scriptPath string) *DockerExecutor {
	d.script = scriptPath
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

func (d *DockerExecutor) Execute(ctx context.Context, execCtx executor.ExecutionContext) (executor.ExecutionResult, error) {
	var config DockerWithConfig
	if err := yaml.Unmarshal(execCtx.WithConfig, &config); err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("could not read config for docker executor %s: %w", d.name, err)
	}

	if config.Interpreter == "" {
		config.Interpreter = "/bin/sh"
	}

	if config.Extension == "" {
		config.Extension = ".sh"
	}
	ext := config.Extension
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cli, err := d.getDockerClient(ctx)
	if err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("failed to get docker client: %w", err)
	}
	defer cli.Close()

	d.client = cli

	outputFile := d.driver.Join(d.driver.TempDir(), fmt.Sprintf("docker-executor-output-%s", xid.New().String()))
	if err := d.driver.CreateFile(ctx, outputFile); err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("failed to create temp file for output: %w", err)
	}

	globalFile := d.driver.Join(d.driver.TempDir(), fmt.Sprintf("docker-executor-global-%s", xid.New().String()))
	if err := d.driver.CreateFile(ctx, globalFile); err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("failed to create temp file for global output: %w", err)
	}

	artifactsDir := d.driver.Join(d.driver.TempDir(), fmt.Sprintf("artifacts-%s", d.execID))
	if err := d.driver.CreateDir(ctx, artifactsDir); err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("failed to create artifacts directory: %w", err)
	}

	localScriptFile := fmt.Sprintf("/tmp/docker-script-%s%s", xid.New().String(), ext)
	if err := os.WriteFile(localScriptFile, []byte(config.Script), 0755); err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("failed to write local script file: %w", err)
	}
	defer os.Remove(localScriptFile)

	remoteScriptFile := d.driver.Join(d.driver.TempDir(), fmt.Sprintf("docker-script-%s%s", xid.New().String(), ext))
	if err := d.driver.Upload(ctx, localScriptFile, remoteScriptFile); err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("failed to upload script: %w", err)
	}
	defer d.driver.Remove(ctx, remoteScriptFile)

	if err := d.driver.SetPermissions(ctx, remoteScriptFile, 0755); err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("failed to set executable permissions: %w", err)
	}

	d.mounts = append(d.mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: outputFile,
		Target: "/tmp/flow/output",
	})

	d.mounts = append(d.mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: globalFile,
		Target: "/tmp/flow/output_global",
	})

	d.mounts = append(d.mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: artifactsDir,
		Target: ArtifactsDir,
	})

	containerScriptPath := fmt.Sprintf("/tmp/flow/script%s", ext)
	d.mounts = append(d.mounts, mount.Mount{
		Type:   mount.TypeBind,
		Source: remoteScriptFile,
		Target: containerScriptPath,
	})

	vars := make([]map[string]any, 0)
	for k, v := range execCtx.Inputs {
		vars = append(vars, map[string]any{k: v})
	}
	vars = append(vars, map[string]any{"FC_OUTPUT": "/tmp/flow/output"})
	vars = append(vars, map[string]any{"FC_OUTPUT_GLOBAL": "/tmp/flow/output_global"})
	vars = append(vars, map[string]any{"FC_ARTIFACTS": "/tmp/flow/artifacts"})

	d.withImage(config.Image).
		withScript(containerScriptPath).
		withEnv(vars)
	d.interpreter = config.Interpreter
	d.stdout = execCtx.Stdout
	d.stderr = execCtx.Stderr

	if err := d.run(ctx); err != nil {
		return executor.ExecutionResult{}, err
	}

	outputs, err := d.readEnvFile(ctx, outputFile)
	if err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("failed to read output: %w", err)
	}

	globals, err := d.readEnvFile(ctx, globalFile)
	if err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("failed to read global output: %w", err)
	}

	return executor.ExecutionResult{Outputs: outputs, Globals: globals}, nil
}

func (d *DockerExecutor) readEnvFile(ctx context.Context, remoteFile string) (map[string]string, error) {
	localFile, err := os.CreateTemp("/tmp", "docker-executor-output-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create local temp file: %w", err)
	}
	defer os.Remove(localFile.Name())
	defer localFile.Close()

	if err := d.driver.Download(ctx, remoteFile, localFile.Name()); err != nil {
		return nil, fmt.Errorf("failed to download temp file: %w", err)
	}

	content, err := os.ReadFile(localFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read temp file %s: %w", localFile.Name(), err)
	}

	return envparse.Parse(strings.NewReader(string(content)))
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
	interpreter := d.interpreter
	if interpreter == "" {
		interpreter = "/bin/sh"
	}

	// Split interpreter by spaces
	interpreterParts := strings.Fields(interpreter)
	cmd := append(interpreterParts, d.script)

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

// DockerExecutorPlugin implements executor.ExecutorPlugin for the docker executor.
type DockerExecutorPlugin struct{}

func (p *DockerExecutorPlugin) GetName() string {
	return "docker"
}

func (p *DockerExecutorPlugin) GetSchema() interface{} {
	return GetSchema()
}

func (p *DockerExecutorPlugin) GetCapabilities() executor.Capability {
	return GetCapabilities()
}

func (p *DockerExecutorPlugin) New(name string, node executor.Node, execID string) (executor.Executor, error) {
	return NewDockerExecutor(name, node, execID)
}
