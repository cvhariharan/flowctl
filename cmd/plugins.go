package cmd

// These are the default executors and remote clients included in flowctl
// Additional executors and remote clients can be added here
import (
	_ "github.com/cvhariharan/flowctl/executors/docker"
	_ "github.com/cvhariharan/flowctl/executors/script"
	_ "github.com/cvhariharan/flowctl/remoteclients/qssh"
	_ "github.com/cvhariharan/flowctl/remoteclients/ssh"
	_ "github.com/cvhariharan/flowctl/sdk/executor"
)
