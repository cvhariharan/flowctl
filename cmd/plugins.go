package cmd

// These are the default executors included in autopilot
// Additional executors can be added here
import (
	_ "github.com/cvhariharan/autopilot/executors/docker"
	_ "github.com/cvhariharan/autopilot/executors/script"
	_ "github.com/cvhariharan/autopilot/remote/ssh"
	_ "github.com/cvhariharan/autopilot/remote/qssh"
	_ "github.com/cvhariharan/autopilot/sdk/executor"
)
