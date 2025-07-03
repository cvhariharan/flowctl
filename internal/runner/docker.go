// Package runner implements the backend that executes the jobs.
//
// Docker runner uses the docker runtime to execute jobs.
package runner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gosimple/slug"
	"github.com/rs/xid"
)

const (
	WORKING_DIR = "/app"
)

type DockerRunnerOptions struct {
	ShowImagePull     bool
	Stdout            io.Writer
	Stderr            io.Writer
	MountDockerSocket bool
}

type DockerRunner struct {
	name             string
	image            string
	src              string
	env              []string
	cmd              []string
	entrypoint       []string
	containerID      string
	workingDirectory string
	artifacts        []string
	mounts           []mount.Mount
	artifactManager  ArtifactManager
	dockerOptions    DockerRunnerOptions
	authConfig       string
}

func NewDockerRunner(name string, artifactManager ArtifactManager, dockerOptions DockerRunnerOptions) *DockerRunner {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	jobName := slug.Make(fmt.Sprintf("%s-%s", name, xid.New().String()))

	if dockerOptions.Stdout == nil {
		dockerOptions.Stdout = os.Stdout
	}
	if dockerOptions.Stderr == nil {
		dockerOptions.Stderr = os.Stderr
	}

	return &DockerRunner{
		name:             jobName,
		src:              filepath.Clean(""),
		workingDirectory: wd,
		artifactManager:  artifactManager,
		dockerOptions:    dockerOptions,
	}
}

// WithImage takes the image url as input and returns a Docker runner.
// The image url format is the same one used in docker pull <url>.
func (d *DockerRunner) WithImage(image string) *DockerRunner {
	d.image = image
	return d
}

// WithSrc takes the src location and returns a Docker runner.
// The src is the path to the folder that will be copied into the docker container for running the job.
func (d *DockerRunner) WithSrc(src string) *DockerRunner {
	d.src = filepath.Clean(src)
	return d
}

// WithEnv is used to specify job variables.
// Env is an array of map[string]any. The length of the map should be 1.
func (d *DockerRunner) WithEnv(env []map[string]any) *DockerRunner {
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

// WithCmd specifies the script that should be run inside the container.
func (d *DockerRunner) WithCmd(cmd []string) *DockerRunner {
	d.cmd = cmd
	return d
}

// WithEntrypoint can be used to override the default entrypoint in a docker image.
func (d *DockerRunner) WithEntrypoint(entrypoint []string) *DockerRunner {
	d.entrypoint = entrypoint
	return d
}

// WithCredentials can be used to specify the credentials for a private image registry.
func (d *DockerRunner) WithCredentials(username, password string) *DockerRunner {
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

func (d *DockerRunner) WithMount(m mount.Mount) *DockerRunner {
	d.mounts = append(d.mounts, m)
	return d
}

// CreatesArtifacts is used to specify the files that will be stored as artifacts.
// The input is a list of file paths wrt the src specified in WithSrc.
func (d *DockerRunner) CreatesArtifacts(artifacts []string) *DockerRunner {
	d.artifacts = artifacts
	return d
}

// Run creates the container based on the provided configuration.
func (d *DockerRunner) Run(ctx context.Context) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("unable to create docker client to create container %s: %v", d.name, err)
	}
	defer cli.Close()

	if err := d.pullImage(ctx, cli); err != nil {
		return fmt.Errorf("could not pull image for container %s: %v", d.name, err)
	}

	resp, err := d.createContainer(ctx, cli)
	d.containerID = resp.ID
	if err != nil {
		return fmt.Errorf("unable to create container %s: %v", d.name, err)
	}
	defer func() {
		if rErr := cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{}); rErr != nil {
			err = rErr
		}
	}()

	if err := d.createSrcDirectories(ctx, cli); err != nil {
		return fmt.Errorf("unable to create source directories for %s: %v", d.name, err)
	}

	if err := d.artifactManager.RetrieveArtifact(d.containerID, nil); err != nil {
		return fmt.Errorf("unable to retrieve artifacts for %s: %v", d.name, err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("unable to start container %s: %v", d.name, err)
	}

	logs, err := cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return fmt.Errorf("unable to attach logs for %s: %v", d.name, err)
	}
	defer logs.Close()

	if _, err := stdcopy.StdCopy(d.dockerOptions.Stdout, d.dockerOptions.Stderr, logs); err != nil {
		return fmt.Errorf("unable to read container logs from %s: %v", d.name, err)
	}

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return fmt.Errorf("error waiting for container %s to stop: %v", d.name, err)
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("container %s exited with status code %d", d.name, status.StatusCode)
		}
		if err := d.publishArtifacts(); err != nil {
			return fmt.Errorf("unable to publish artifacts for %s: %v", d.name, err)
		}
	case <-ctx.Done():
		return fmt.Errorf("context timed out, stopping container %s", d.name)
	}

	return nil
}

func (d *DockerRunner) createSrcDirectories(ctx context.Context, cli *client.Client) error {
	tar, err := archive.TarWithOptions(d.src, &archive.TarOptions{})
	if err != nil {
		return err
	}

	return cli.CopyToContainer(ctx, d.containerID, WORKING_DIR, tar, container.CopyToContainerOptions{})
}

func (d *DockerRunner) publishArtifacts() error {
	for _, v := range d.artifacts {
		if _, err := d.artifactManager.PublishArtifact(d.containerID, filepath.Join(WORKING_DIR, v)); err != nil {
			return err
		}
	}
	return nil
}

func (d *DockerRunner) pullImage(ctx context.Context, cli *client.Client) error {
	reader, err := cli.ImagePull(ctx, d.image, image.PullOptions{RegistryAuth: d.authConfig})
	if err != nil {
		return err
	}
	defer reader.Close()

	imageLogs := io.Discard
	if d.dockerOptions.ShowImagePull {
		imageLogs = d.dockerOptions.Stdout
	}
	if _, err := io.Copy(imageLogs, reader); err != nil {
		return err
	}

	return nil
}

func (d *DockerRunner) createContainer(ctx context.Context, cli *client.Client) (container.CreateResponse, error) {
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
		Mounts: d.mounts,
	}, nil, nil, d.name)
	if err != nil {
		return container.CreateResponse{}, err
	}
	return resp, nil
}
