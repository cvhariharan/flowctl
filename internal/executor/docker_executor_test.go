package executor

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestDockerExecutor_Execute(t *testing.T) {
	// Test case for local execution
	t.Run("local execution", func(t *testing.T) {
		// Create a mock execution context
		config := DockerWithConfig{
			Image:  "alpine:latest",
			Script: `echo "MESSAGE=hellothere" > $OUTPUT`,
		}
		withConfig, err := yaml.Marshal(config)
		assert.NoError(t, err)

		execCtx := ExecutionContext{
			WithConfig: withConfig,
			Inputs:     make(map[string]interface{}),
			Stdout:     new(bytes.Buffer),
			Stderr:     new(bytes.Buffer),
			Node:       Node{}, // Empty node for local execution
		}

		// Create a new DockerExecutor
		executor, err := NewDockerExecutor("test-local", DockerRunnerOptions{ShowImagePull: false, KeepContainer: true})
		assert.NoError(t, err)

		// Execute the executor
		outputs, err := executor.Execute(context.Background(), execCtx)

		// Assert that there is no error
		assert.NoError(t, err)

		// Assert the output
		assert.Equal(t, "hellothere", outputs["MESSAGE"])

		// The script redirects to a file, so stdout should be empty.
		// Image pull logs are discarded when ShowImagePull is false.
		assert.Equal(t, "", execCtx.Stdout.(*bytes.Buffer).String())
	})

	// Test case for remote execution
	// This will require a running ssh server with docker.
	// This test is skipped if the required environment variables are not set.
	t.Run("remote execution", func(t *testing.T) {
		remoteHost := os.Getenv("TEST_REMOTE_HOST")
		remoteUser := os.Getenv("TEST_REMOTE_USER")
		remoteKey := os.Getenv("TEST_REMOTE_KEY")

		if remoteHost == "" || remoteUser == "" || remoteKey == "" {
			t.Skip("Skipping remote execution test: TEST_REMOTE_HOST, TEST_REMOTE_USER, and TEST_REMOTE_KEY must be set")
		}

		remotePort := 22
		// Create a mock execution context
		config := DockerWithConfig{
			Image:  "ubuntu:latest",
			Script: `grep '^NAME=' /etc/os-release > $OUTPUT`,
		}
		withConfig, err := yaml.Marshal(config)
		assert.NoError(t, err)

		// Create buffers for stdout and stderr
		stdoutBuf := new(bytes.Buffer)
		stderrBuf := new(bytes.Buffer)

		execCtx := ExecutionContext{
			WithConfig: withConfig,
			Inputs:     make(map[string]interface{}),
			Stdout:     stdoutBuf,
			Stderr:     stderrBuf,
			Node: Node{
				Hostname: remoteHost,
				Port:     remotePort,
				Username: remoteUser,
				Auth: NodeAuth{
					Method: "ssh_key",
					Key:    remoteKey,
				},
			},
		}

		// Create a new DockerExecutor
		executor, err := NewDockerExecutor("test-remote", DockerRunnerOptions{ShowImagePull: false, KeepContainer: true})
		assert.NoError(t, err)

		// Execute the executor
		outputs, err := executor.Execute(context.Background(), execCtx)

		// Assert that there is no error
		assert.NoError(t, err)

		// Assert the output
		assert.Equal(t, "Ubuntu", outputs["NAME"])
		assert.Equal(t, "", execCtx.Stdout.(*bytes.Buffer).String())
	})
}

// New test for artifact file handling
func TestDockerExecutor_ArtifactFile(t *testing.T) {
	artifactFile := "artifact_test.txt"
	artifactContent := "artifact-content-123"

	config := DockerWithConfig{
		Image:  "alpine:latest",
		Script: "echo '" + artifactContent + "' > " + artifactFile,
	}
	withConfig, err := yaml.Marshal(config)
	assert.NoError(t, err)

	execCtx := ExecutionContext{
		WithConfig: withConfig,
		Artifacts:  []string{artifactFile},
		Inputs:     make(map[string]interface{}),
		Stdout:     new(bytes.Buffer),
		Stderr:     new(bytes.Buffer),
		Node:       Node{},
	}

	executor, err := NewDockerExecutor("test-artifact", DockerRunnerOptions{ShowImagePull: false, KeepContainer: true})
	assert.NoError(t, err)

	_, err = executor.Execute(context.Background(), execCtx)
	assert.NoError(t, err)

	// Pull the artifact file from the container
	localPath := "local_" + artifactFile
	err = executor.PullFile(context.Background(), artifactFile, localPath)
	assert.NoError(t, err)

	// Read and check the contents
	data, err := ioutil.ReadFile(localPath)
	assert.NoError(t, err)
	assert.Equal(t, artifactContent+"\n", string(data))

	// Cleanup
	_ = os.Remove(localPath)
}
