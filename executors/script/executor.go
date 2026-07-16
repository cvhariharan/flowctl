package script

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cvhariharan/flowctl/sdk/executor"
	"github.com/hashicorp/go-envparse"
	"github.com/invopop/jsonschema"
	"github.com/rs/xid"
	"gopkg.in/yaml.v3"
)

type ScriptWithConfig struct {
	Script      string `yaml:"script" json:"script" jsonschema:"title=script" jsonschema_extras:"widget=codeeditor"`
	Interpreter string `yaml:"interpreter,omitempty" json:"interpreter,omitempty" jsonschema:"title=interpreter,description=Shell interpreter to use (default: /bin/bash)" jsonschema_extras:"placeholder=/bin/bash"`
	Extension   string `yaml:"extension,omitempty" json:"extension,omitempty" jsonschema:"title=extension,description=File extension for the script (default: .sh)" jsonschema_extras:"placeholder=.sh`
}

type ScriptExecutor struct {
	name             string
	stdout           io.Writer
	stderr           io.Writer
	workingDirectory string
	driver           executor.NodeDriver
	artifactsDir     string
	execID           string
}

func GetSchema() interface{} {
	return jsonschema.Reflect(&ScriptWithConfig{})
}

func NewScriptExecutor(name string, node executor.Node, execID string) (executor.Executor, error) {
	jobName := fmt.Sprintf("script-%s-%s", name, xid.New().String())

	driver, err := executor.NewNodeDriver(context.Background(), node)
	if err != nil {
		return nil, fmt.Errorf("failed to create node driver: %w", err)
	}

	// Create artifacts directory
	artifactsDir := driver.Join(driver.TempDir(), fmt.Sprintf("artifacts-%s", execID))
	if err := driver.CreateDir(context.Background(), artifactsDir); err != nil {
		driver.Close()
		return nil, fmt.Errorf("failed to create artifacts directory: %w", err)
	}

	exec := &ScriptExecutor{
		name:             jobName,
		workingDirectory: driver.GetWorkingDirectory(),
		driver:           driver,
		artifactsDir:     artifactsDir,
		execID:           execID,
	}

	return exec, nil
}

func (s *ScriptExecutor) GetArtifactsDir() string {
	return s.artifactsDir
}

func (s *ScriptExecutor) Close() error {
	return s.driver.Close()
}

func GetCapabilities() executor.Capability {
	return executor.RemoteExecution | executor.EnvironmentVariables | executor.FileTransfer | executor.StreamingOutput
}

func (s *ScriptExecutor) Execute(ctx context.Context, execCtx executor.ExecutionContext) (executor.ExecutionResult, error) {
	var config ScriptWithConfig
	if err := yaml.Unmarshal(execCtx.WithConfig, &config); err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("could not read config for script executor %s: %w", s.name, err)
	}

	if config.Interpreter == "" {
		config.Interpreter = "/bin/bash"
	}

	s.stdout = execCtx.Stdout
	s.stderr = execCtx.Stderr

	if err := s.driver.CreateDir(ctx, s.workingDirectory); err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("failed to create working directory: %w", err)
	}

	outputFile := s.driver.Join(s.driver.TempDir(), fmt.Sprintf("script-executor-output-%s", xid.New().String()))
	if err := s.driver.CreateFile(ctx, outputFile); err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("failed to create temp file for output: %w", err)
	}

	globalFile := s.driver.Join(s.driver.TempDir(), fmt.Sprintf("script-executor-global-%s", xid.New().String()))
	if err := s.driver.CreateFile(ctx, globalFile); err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("failed to create temp file for global output: %w", err)
	}

	env := s.prepareEnvironment(execCtx.Inputs, outputFile, globalFile)

	if err := s.runScript(ctx, config, env); err != nil {
		return executor.ExecutionResult{}, err
	}

	outputs, err := s.readEnvFile(ctx, outputFile)
	if err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("failed to read output: %w", err)
	}

	globals, err := s.readEnvFile(ctx, globalFile)
	if err != nil {
		return executor.ExecutionResult{}, fmt.Errorf("failed to read global output: %w", err)
	}

	return executor.ExecutionResult{Outputs: outputs, Globals: globals}, nil
}

func (s *ScriptExecutor) prepareEnvironment(inputs map[string]interface{}, outputFile, globalFile string) []string {
	var env []string

	for k, v := range inputs {
		env = append(env, fmt.Sprintf("%s=%s", k, fmt.Sprint(v)))
	}

	env = append(env, fmt.Sprintf("FC_OUTPUT=%s", outputFile))
	env = append(env, fmt.Sprintf("FC_OUTPUT_GLOBAL=%s", globalFile))
	env = append(env, fmt.Sprintf("FC_ARTIFACTS=%s", s.artifactsDir))

	return env
}

func (s *ScriptExecutor) runScript(ctx context.Context, config ScriptWithConfig, env []string) error {
	// Normalize extension (add dot if not present)
	if config.Extension == "" {
		config.Extension = ".sh"
	}
	ext := config.Extension
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	localScriptFile := fmt.Sprintf("/tmp/local-script-%s%s", xid.New().String(), ext)
	if err := os.WriteFile(localScriptFile, []byte(config.Script), 0755); err != nil {
		return fmt.Errorf("failed to write local script file: %w", err)
	}
	defer os.Remove(localScriptFile)

	remoteScriptFile := s.driver.Join(s.driver.TempDir(), fmt.Sprintf("script-%s%s", xid.New().String(), ext))
	if err := s.driver.Upload(ctx, localScriptFile, remoteScriptFile); err != nil {
		return fmt.Errorf("failed to upload script: %w", err)
	}
	defer s.driver.Remove(ctx, remoteScriptFile)

	if err := s.driver.SetPermissions(ctx, remoteScriptFile, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	command := fmt.Sprintf("%s %s", config.Interpreter, remoteScriptFile)
	return s.driver.Exec(ctx, command, s.workingDirectory, env, s.stdout, s.stderr)
}

func (s *ScriptExecutor) readEnvFile(ctx context.Context, remoteFile string) (map[string]string, error) {
	localFile, err := os.CreateTemp("/tmp", "script-executor-output-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create local temp file: %w", err)
	}
	defer os.Remove(localFile.Name())
	defer localFile.Close()

	if err := s.driver.Download(ctx, remoteFile, localFile.Name()); err != nil {
		return nil, fmt.Errorf("failed to download temp file: %w", err)
	}

	content, err := os.ReadFile(localFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read temp file %s: %w", localFile.Name(), err)
	}

	return envparse.Parse(strings.NewReader(string(content)))
}

// ScriptExecutorPlugin implements executor.ExecutorPlugin for the script executor.
type ScriptExecutorPlugin struct{}

func (p *ScriptExecutorPlugin) GetName() string {
	return "script"
}

func (p *ScriptExecutorPlugin) GetSchema() interface{} {
	return GetSchema()
}

func (p *ScriptExecutorPlugin) GetCapabilities() executor.Capability {
	return GetCapabilities()
}

func (p *ScriptExecutorPlugin) New(name string, node executor.Node, execID string) (executor.Executor, error) {
	return NewScriptExecutor(name, node, execID)
}
