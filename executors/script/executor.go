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
	Interpreter string `yaml:"interpreter,omitempty" json:"interpreter,omitempty" jsonschema:"title=interpreter,description=Shell interpreter to use (default: /bin/bash)"`
}

type ScriptExecutor struct {
	name             string
	stdout           io.Writer
	stderr           io.Writer
	workingDirectory string
	driver           executor.NodeDriver
}

func init() {
	executor.RegisterExecutor("script", NewScriptExecutor)
	executor.RegisterSchema("script", GetSchema())
}

func GetSchema() interface{} {
	return jsonschema.Reflect(&ScriptWithConfig{})
}

func NewScriptExecutor(name string, driver executor.NodeDriver) (executor.Executor, error) {
	jobName := fmt.Sprintf("script-%s-%s", name, xid.New().String())

	executor := &ScriptExecutor{
		name:             jobName,
		workingDirectory: driver.GetWorkingDirectory(),
		driver:           driver,
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

	if err := s.driver.CreateDir(ctx, s.workingDirectory); err != nil {
		return nil, fmt.Errorf("failed to create working directory: %w", err)
	}

	tempFile := s.driver.Join(s.driver.TempDir(), fmt.Sprintf("script-executor-output-%s", xid.New().String()))
	if err := s.driver.CreateFile(ctx, tempFile); err != nil {
		return nil, fmt.Errorf("failed to create temp file for output: %w", err)
	}

	// Create artifacts directory
	artifactsDir := s.driver.Join(s.driver.TempDir(), fmt.Sprintf("artifacts-%s", execCtx.ExecID))
	if err := s.driver.CreateDir(ctx, artifactsDir); err != nil {
		return nil, fmt.Errorf("failed to create artifacts directory: %w", err)
	}

	// Prepare environment variables
	env := s.prepareEnvironment(execCtx.Inputs, tempFile, artifactsDir)

	// Execute the script
	if err := s.runScript(ctx, config, env); err != nil {
		return nil, err
	}

	// Read output file and parse environment variables
	outputContents, err := s.readTempFileContents(ctx, tempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read temp file contents: %w", err)
	}

	outputEnv, err := envparse.Parse(outputContents)
	if err != nil {
		return nil, fmt.Errorf("could not load output env: %w", err)
	}

	return outputEnv, nil
}

func (s *ScriptExecutor) prepareEnvironment(inputs map[string]interface{}, outputFile string, artifactsDir string) []string {
	var env []string

	for k, v := range inputs {
		env = append(env, fmt.Sprintf("%s=%s", k, fmt.Sprint(v)))
	}

	env = append(env, fmt.Sprintf("FC_OUTPUT=%s", outputFile))
	env = append(env, fmt.Sprintf("FC_ARTIFACTS=%s", artifactsDir))

	return env
}

func (s *ScriptExecutor) runScript(ctx context.Context, config ScriptWithConfig, env []string) error {
	localScriptFile := fmt.Sprintf("/tmp/local-script-%s.sh", xid.New().String())
	if err := os.WriteFile(localScriptFile, []byte(config.Script), 0755); err != nil {
		return fmt.Errorf("failed to write local script file: %w", err)
	}
	defer os.Remove(localScriptFile)

	remoteScriptFile := s.driver.Join(s.driver.TempDir(), fmt.Sprintf("script-%s.sh", xid.New().String()))
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

func (s *ScriptExecutor) readTempFileContents(ctx context.Context, tempFile string) (io.Reader, error) {
	localTempFile, err := os.CreateTemp("/tmp", "script-executor-output-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create local temp file: %w", err)
	}
	defer os.Remove(localTempFile.Name())
	defer localTempFile.Close()

	if err := s.driver.Download(ctx, tempFile, localTempFile.Name()); err != nil {
		return nil, fmt.Errorf("failed to download temp file: %w", err)
	}

	content, err := os.ReadFile(localTempFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read temp file %s: %w", localTempFile.Name(), err)
	}
	return strings.NewReader(string(content)), nil
}
