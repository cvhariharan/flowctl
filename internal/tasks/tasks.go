package tasks

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"regexp"

	"github.com/cvhariharan/autopilot/internal/flow"
	"github.com/cvhariharan/autopilot/internal/runner"
	"github.com/hibiken/asynq"
)

const (
	TypeFlowExecution = "flow_execution"
)

type FlowExecutionPayload struct {
	Workflow flow.Flow
	Input    map[string]interface{}
}

func NewFlowExecution(f flow.Flow, input map[string]interface{}) (*asynq.Task, error) {
	payload, err := json.Marshal(FlowExecutionPayload{Workflow: f, Input: input})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask("flow_execution", payload), nil
}

type FlowRunner struct {
	logger          io.Writer
	artifactManager runner.ArtifactManager
}

func (r *FlowRunner) HandleFlowExecution(ctx context.Context, t *asynq.Task) error {
	var payload FlowExecutionPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return err
	}

	// pattern to extract interpolated variables
	pattern := `{{\s*([^}]+)\s*}}`
	re := regexp.MustCompile(pattern)

	for _, action := range payload.Workflow.Actions {
		// jobCtx, cancel := context.WithTimeout(ctx, time.Hour)
		// defer cancel()
		for _, variable := range action.Variables {
			matches := re.FindString(variable.Value())
			log.Println(matches)
		}

	}

	return nil
}
