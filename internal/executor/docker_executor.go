package executor

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"strings"

	"net"
	"net/http"
	"os"
	"time"

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
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

const (
	WORKING_DIR = "/app"
)

type DockerWithConfig struct {
	Image  string `yaml:"image"`
	Script string `yaml:"script"`
}

type DockerExecutor struct {
	name             string
	image            string
	src              string
	env              []string
	cmd              []string
	entrypoint       []string
	containerID      string
	workingDirectory string
	mounts           []mount.Mount
	dockerOptions    DockerRunnerOptions
	authConfig       string
	stdout           io.Writer
	stderr           io.Writer
}

func (d *DockerExecutor) withImage(image string) *DockerExecutor {
	d.image = image
	return d
}

func (d *DockerExecutor) withSrc(src string) *DockerExecutor {
	d.src = filepath.Clean(src)
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

func (d *DockerExecutor) withMount(m mount.Mount) *DockerExecutor {
	d.mounts = append(d.mounts, m)
	return d
}

type DockerRunnerOptions struct {
	ShowImagePull     bool
	MountDockerSocket bool
}

func NewDockerExecutor(name string, dockerOptions DockerRunnerOptions) Executor {
	jobName := slug.Make(fmt.Sprintf("%s-%s", name, xid.New().String()))

	return &DockerExecutor{
		name:          jobName,
		dockerOptions: dockerOptions,
	}
}

func (d *DockerExecutor) Execute(ctx context.Context, execCtx ExecutionContext) (map[string]string, error) {
	var config DockerWithConfig
	if err := yaml.Unmarshal(execCtx.WithConfig, &config); err != nil {
		return nil, fmt.Errorf("could not read config for docker executor %s: %w", d.name, err)
	}

	// Create temp file for outputs
	outfile, err := os.CreateTemp("", fmt.Sprintf("output-executor-%s-*", d.name))
	if err != nil {
		return nil, fmt.Errorf("could not create tmp file for storing executor %s outputs: %w", d.name, err)
	}
	defer func() {
		outfile.Close()
		os.Remove(outfile.Name())
	}()

	vars := make([]map[string]any, 0)
	for k, v := range execCtx.Inputs {
		vars = append(vars, map[string]any{k: v})
	}
	// Add output env variable
	vars = append(vars, map[string]any{"OUTPUT": "/tmp/flow/output"})

	d.withImage(config.Image).
		withCmd([]string{config.Script}).
		withEnv(vars).
		withMount(mount.Mount{
			Type:   mount.TypeBind,
			Source: outfile.Name(),
			Target: "/tmp/flow/output",
		})
	d.stdout = execCtx.Stdout
	d.stderr = execCtx.Stderr

	if err := d.run(ctx, execCtx); err != nil {
		return nil, err
	}

	// Parse output file env
	outputTempFile, err := os.Open(outfile.Name())
	if err != nil {
		return nil, fmt.Errorf("error opening output file for reading: %w", err)
	}
	defer outputTempFile.Close()

	outputEnv, err := envparse.Parse(outputTempFile)
	if err != nil {
		return nil, fmt.Errorf("could not load output env: %w", err)
	}

	return outputEnv, nil
}

func (d *DockerExecutor) run(ctx context.Context, execCtx ExecutionContext) error {
	cli, err := d.getDockerClient(execCtx)
	if err != nil {
		return fmt.Errorf("failed to get docker client: %w", err)
	}
	defer cli.Close()

	if err := d.pullImage(ctx, cli); err != nil {
		return fmt.Errorf("could not pull image: %v", err)
	}

	resp, err := d.createContainer(ctx, cli)
	if err != nil {
		return fmt.Errorf("unable to create container: %v", err)
	}
	d.containerID = resp.ID

	defer func() {
		if rErr := cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{}); rErr != nil {
			log.Printf("Error removing container: %v", rErr)
		}
	}()

	if d.src != "" {
		if err := d.createSrcDirectories(ctx, cli); err != nil {
			return fmt.Errorf("unable to create source directories: %v", err)
		}
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("unable to start container: %v", err)
	}

	logs, err := cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{
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

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return fmt.Errorf("error waiting for container: %v", err)
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("container exited with code %d", status.StatusCode)
		}
	}

	return nil
}

func (d *DockerExecutor) createSrcDirectories(ctx context.Context, cli *client.Client) error {
	tar, err := archive.TarWithOptions(d.src, &archive.TarOptions{})
	if err != nil {
		return err
	}

	return cli.CopyToContainer(ctx, d.containerID, WORKING_DIR, tar, container.CopyToContainerOptions{})
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

func (d *DockerExecutor) PushFile(ctx context.Context, localFilePath string, remoteFilePath string) error {
	// TODO: Implement this method
	return nil
}

func (d *DockerExecutor) getDockerClient(execCtx ExecutionContext) (*client.Client, error) {
	if execCtx.Node.Hostname == "" {
		return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	}

	dialer, err := createCustomSSHDialer(execCtx.Node)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: dialer,
		},
	}

	return client.NewClientWithOpts(
		client.WithHTTPClient(httpClient),
		client.WithHost("unix:///var/run/docker.sock"),
		client.WithAPIVersionNegotiation(),
	)
}

func createCustomSSHDialer(node Node) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	var authMethod ssh.AuthMethod
	switch node.Auth.Method {
	case "ssh_key":
		signer, err := ssh.ParsePrivateKey([]byte(node.Auth.Key))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethod = ssh.PublicKeys(signer)
	case "password":
		authMethod = ssh.Password(node.Auth.Key)
	default:
		return nil, fmt.Errorf("unsupported auth method: %s", node.Auth.Method)
	}

	sshConfig := &ssh.ClientConfig{
		User: node.Username,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Use proper verification in production
		Timeout:         30 * time.Second,
	}

	sshHost := fmt.Sprintf("%s:%d", node.Hostname, node.Port)

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		sshConn, err := ssh.Dial("tcp", sshHost, sshConfig)
		if err != nil {
			return nil, fmt.Errorf("SSH connection failed: %w", err)
		}

		conn, err := sshConn.Dial("unix", "/var/run/docker.sock")
		if err != nil {
			sshConn.Close()
			return nil, fmt.Errorf("Docker daemon connection failed: %w", err)
		}
		return conn, nil
	}, nil
}

func (d *DockerExecutor) PullFile(ctx context.Context, remoteFilePath string, localFilePath string) error {
	// TODO: Implement this method
	return nil
}
