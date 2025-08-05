package script

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cvhariharan/flowctl/sdk/executor"
	"github.com/cvhariharan/flowctl/sdk/remoteclient"
	"github.com/hashicorp/go-envparse"
	"github.com/invopop/jsonschema"
	"github.com/rs/xid"
	"gopkg.in/yaml.v3"
)

type ScriptWithConfig struct {
	Script      string `yaml:"script" json:"script" jsonschema:"title=script"`
	Interpreter string `yaml:"interpreter" json:"interpreter" jsonschema:"title=interpreter"`
}

type ScriptExecutor struct {
	name               string
	remoteClient       remoteclient.RemoteClient
	artifactsDirectory string
	stdout             io.Writer
	stderr             io.Writer
}

func init() {
	executor.RegisterExecutor("script", NewScriptExecutor)
	executor.RegisterSchema("script", GetSchema())
}

func GetSchema() interface{} {
	return jsonschema.Reflect(&ScriptWithConfig{})
}

func NewScriptExecutor(name string, node executor.Node) (executor.Executor, error) {
	jobName := fmt.Sprintf("script-%s-%s", name, xid.New().String())

	executor := &ScriptExecutor{
		name:               jobName,
		artifactsDirectory: fmt.Sprintf("/tmp/script-artifacts-%s", xid.New().String()),
	}

	// Initialize remote client if this is for remote execution
	if node.Hostname != "" {
		clientType := "ssh"
		if node.ConnectionType != "" {
			clientType = node.ConnectionType
		}
		remoteClient, err := remoteclient.GetClient(clientType, node)
		if err != nil {
			return nil, fmt.Errorf("failed to create remote client for node %s: %w", node.Hostname, err)
		}
		executor.remoteClient = remoteClient
	}

	return executor, nil
}

func (s *ScriptExecutor) Execute(ctx context.Context, execCtx executor.ExecutionContext) (map[string]string, error) {
	var config ScriptWithConfig
	if err := yaml.Unmarshal(execCtx.WithConfig, &config); err != nil {
		return nil, fmt.Errorf("could not read config for script executor %s: %w", s.name, err)
	}

	// Set default interpreter
	if config.Interpreter == "" {
		config.Interpreter = "/bin/bash"
	}

	s.stdout = execCtx.Stdout
	s.stderr = execCtx.Stderr

	// Create output file for capturing environment variables
	tempFile := fmt.Sprintf("/tmp/script-executor-output-%s", xid.New().String())
	if err := s.createFileOrDirectory(tempFile, false); err != nil {
		return nil, fmt.Errorf("failed to create temp file for output: %w", err)
	}

	// Create artifacts directories
	if err := s.createFileOrDirectory(filepath.Join(s.artifactsDirectory, "push"), true); err != nil {
		return nil, fmt.Errorf("failed to create artifacts directory: %w", err)
	}
	if err := s.createFileOrDirectory(filepath.Join(s.artifactsDirectory, "pull"), true); err != nil {
		return nil, fmt.Errorf("failed to create artifacts directory: %w", err)
	}

	// Prepare environment variables
	env := s.prepareEnvironment(execCtx.Inputs, tempFile)

	// Execute the script
	if err := s.runScript(ctx, config, env); err != nil {
		return nil, err
	}

	// Read output file and parse environment variables
	outputContents, err := s.readTempFileContents(tempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read temp file contents: %w", err)
	}

	outputEnv, err := envparse.Parse(outputContents)
	if err != nil {
		return nil, fmt.Errorf("could not load output env: %w", err)
	}

	return outputEnv, nil
}

func (s *ScriptExecutor) prepareEnvironment(inputs map[string]interface{}, outputFile string) []string {
	var env []string

	// Add input variables
	for k, v := range inputs {
		env = append(env, fmt.Sprintf("%s=%s", k, fmt.Sprint(v)))
	}

	// Add output file location
	env = append(env, fmt.Sprintf("OUTPUT=%s", outputFile))

	return env
}

func (s *ScriptExecutor) runScript(ctx context.Context, config ScriptWithConfig, env []string) error {
	if s.remoteClient != nil {
		return s.runRemoteScript(ctx, config, env)
	}
	return s.runLocalScript(ctx, config, env)
}

func (s *ScriptExecutor) runLocalScript(ctx context.Context, config ScriptWithConfig, env []string) error {
	cmd := exec.CommandContext(ctx, config.Interpreter, "-c", config.Script)
	cmd.Env = env
	cmd.Stdout = s.stdout
	cmd.Stderr = s.stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("script execution failed: %w", err)
	}

	return nil
}

func (s *ScriptExecutor) runRemoteScript(ctx context.Context, config ScriptWithConfig, env []string) error {
	// Create a script file with the environment and script content
	scriptContent := s.buildRemoteScript(config, env)

	// Upload the script to the remote machine
	scriptFile := fmt.Sprintf("/tmp/script-%s.sh", xid.New().String())
	localScriptFile := fmt.Sprintf("/tmp/local-script-%s.sh", xid.New().String())

	// Write script to local temp file
	if err := os.WriteFile(localScriptFile, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write local script file: %w", err)
	}
	defer os.Remove(localScriptFile)

	// Upload script to remote
	if err := s.remoteClient.Upload(localScriptFile, scriptFile); err != nil {
		return fmt.Errorf("failed to upload script to remote: %w", err)
	}

	// Execute the script on remote
	output, err := s.remoteClient.RunCommand(fmt.Sprintf("chmod +x %s && %s", scriptFile, scriptFile))
	if err != nil {
		return fmt.Errorf("remote script execution failed: %w", err)
	}

	// Write output to stdout
	if s.stdout != nil {
		s.stdout.Write([]byte(output))
	}

	// Clean up remote script file
	s.remoteClient.RunCommand(fmt.Sprintf("rm -f %s", scriptFile))

	return nil
}

func (s *ScriptExecutor) buildRemoteScript(config ScriptWithConfig, env []string) string {
	var script strings.Builder

	script.WriteString("#!" + config.Interpreter + "\n")
	script.WriteString("set -e\n\n")

	// Export environment variables
	for _, envVar := range env {
		if strings.Contains(envVar, "=") {
			script.WriteString("export " + envVar + "\n")
		}
	}

	script.WriteString("\n")
	script.WriteString(config.Script)
	script.WriteString("\n")

	return script.String()
}

func (s *ScriptExecutor) readTempFileContents(tempFile string) (io.Reader, error) {
	readFile := func(filePath string) (io.Reader, error) {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read temp file %s: %w", filePath, err)
		}
		return strings.NewReader(string(content)), nil
	}

	if s.remoteClient != nil {
		// For remote execution, download the file using the remote client
		localTempFile, err := os.CreateTemp("/tmp", "script-executor-output-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create local temp file: %w", err)
		}
		defer os.Remove(localTempFile.Name())
		defer localTempFile.Close()

		if err := s.remoteClient.Download(tempFile, localTempFile.Name()); err != nil {
			return nil, fmt.Errorf("failed to download temp file from remote: %w", err)
		}

		return readFile(localTempFile.Name())
	} else {
		// For local execution, read the file directly
		return readFile(tempFile)
	}
}

func (s *ScriptExecutor) createFileOrDirectory(name string, dir bool) error {
	if s.remoteClient == nil {
		if dir {
			return os.MkdirAll(name, 0755)
		}
		_, err := os.Create(name)
		return err
	}

	// Remote execution
	var cmd string
	if dir {
		cmd = fmt.Sprintf("mkdir -p %s && chmod 755 %s", name, name)
	} else {
		cmd = fmt.Sprintf("touch %s && chmod 755 %s", name, name)
	}
	_, err := s.remoteClient.RunCommand(cmd)
	return err
}

func (s *ScriptExecutor) PushFile(ctx context.Context, localFilePath string, remoteFilePath string) error {
	destPath := filepath.Join(s.artifactsDirectory, "push", remoteFilePath)
	if err := s.createFileOrDirectory(filepath.Dir(destPath), true); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", destPath, err)
	}

	if s.remoteClient == nil {
		// Local execution: copy file directly
		srcFile, err := os.Open(filepath.Clean(localFilePath))
		if err != nil {
			return fmt.Errorf("failed to open local file %s: %w", localFilePath, err)
		}
		defer srcFile.Close()

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

	// Remote execution: upload file to remote machine
	if err := s.remoteClient.Upload(localFilePath, destPath); err != nil {
		return fmt.Errorf("failed to upload file %s to remote path %s: %w", localFilePath, destPath, err)
	}
	return nil
}

func (s *ScriptExecutor) PullFile(ctx context.Context, remoteFilePath string, localFilePath string) error {
	srcFile := filepath.Join(s.artifactsDirectory, "pull", remoteFilePath)
	destFile, err := os.Create(filepath.Clean(localFilePath))
	if err != nil {
		return fmt.Errorf("failed to create local file %s: %w", localFilePath, err)
	}
	defer destFile.Close()

	if s.remoteClient == nil {
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
	if err := s.remoteClient.Download(srcFile, localFilePath); err != nil {
		return fmt.Errorf("failed to download file from remote path %s to local path %s: %w", srcFile, localFilePath, err)
	}
	return nil
}

func (s *ScriptExecutor) Close() error {
	if s.remoteClient != nil {
		return s.remoteClient.Close()
	}
	return nil
}
