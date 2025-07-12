package executor

import (
	"bytes"
	"context"
	"fmt"
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
		executor := NewDockerExecutor("test-local", DockerRunnerOptions{ShowImagePull: false})

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
		remotePortStr := os.Getenv("TEST_REMOTE_PORT")

		if remoteHost == "" || remoteUser == "" || remoteKey == "" {
			t.Skip("Skipping remote execution test: TEST_REMOTE_HOST, TEST_REMOTE_USER, and TEST_REMOTE_KEY must be set")
		}

		remotePort := 22
		if remotePortStr != "" {
			var err error
			_, err = fmt.Sscanf(remotePortStr, "%d", &remotePort)
			if err != nil {
				t.Fatalf("failed to parse remote port: %v", err)
			}
		}

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
		executor := NewDockerExecutor("test-remote", DockerRunnerOptions{ShowImagePull: false})

		// Execute the executor
		outputs, err := executor.Execute(context.Background(), execCtx)

		t.Log("STDOUT:", stdoutBuf.String())
		t.Log("STDERR:", stderrBuf.String())

		// Assert that there is no error
		assert.NoError(t, err)

		// Assert the output
		assert.Equal(t, "Ubuntu", outputs["NAME"])
		assert.Equal(t, "", execCtx.Stdout.(*bytes.Buffer).String())
	})
}
