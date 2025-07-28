package cmd

// These are the default executors included in flowctl
// Additional executors can be added here
import (
	_ "github.com/cvhariharan/flowctl/executors/docker"
	_ "github.com/cvhariharan/flowctl/executors/script"
	_ "github.com/cvhariharan/flowctl/remote/qssh"
	_ "github.com/cvhariharan/flowctl/remote/ssh"
	_ "github.com/cvhariharan/flowctl/sdk/executor"
)
