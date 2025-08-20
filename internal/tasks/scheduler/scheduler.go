package scheduler

import (
	"context"
	"encoding/json"
	"log"

	"github.com/cvhariharan/flowctl/internal/core"
	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/tasks"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// FlowScheduleProvider implements asynq.PeriodicTaskConfigProvider
// It provides periodic task configurations for flows with cron schedules
type FlowScheduleProvider struct {
	core *core.Core
}

// NewFlowScheduleProvider creates a new FlowScheduleProvider
func NewFlowScheduleProvider(core *core.Core) *FlowScheduleProvider {
	return &FlowScheduleProvider{
		core: core,
	}
}

// GetConfigs implements asynq.PeriodicTaskConfigProvider interface
// It retrieves all flows with cron schedules and generates task configs
func (p *FlowScheduleProvider) GetConfigs() ([]*asynq.PeriodicTaskConfig, error) {
	// Use the existing GetScheduledFlows method from Core
	scheduledFlows := p.core.GetScheduledFlows()
	log.Printf("FlowScheduleProvider.GetConfigs() called, found %d scheduled flows", len(scheduledFlows))

	var configs []*asynq.PeriodicTaskConfig

	for _, flow := range scheduledFlows {
		// Convert flow to task model without node resolution (nodes will be resolved in PreEnqueueFunc)
		taskFlow, err := models.ToTaskFlowModel(flow, func(nodeNames []string) ([]models.Node, error) {
			// Return empty nodes - they will be resolved just before execution in PreEnqueueFunc
			return []models.Node{}, nil
		})
		if err != nil {
			log.Printf("failed to convert flow %s to task flow: %v", flow.Meta.ID, err)
			continue
		}

		n, err := p.core.GetNamespaceByName(context.Background(), flow.Meta.Namespace)
		if err != nil {
			log.Printf("could not get namespace ID for %s: %v", flow.Meta.Namespace, err)
			continue
		}

		systemUserUUID := "00000000-0000-0000-0000-000000000000"
		task, err := tasks.NewFlowExecution(taskFlow, make(map[string]any), 0, uuid.NewString(), n.ID, tasks.TriggerTypeScheduled, systemUserUUID)
		if err != nil {
			log.Printf("failed to create task for flow %s: %v", flow.Meta.ID, err)
			continue
		}

		// Create periodic task config
		config := &asynq.PeriodicTaskConfig{
			Cronspec: flow.Meta.Schedule,
			Task:     task,
		}

		configs = append(configs, config)
		log.Printf("Added config for flow %s with schedule %s", flow.Meta.ID, flow.Meta.Schedule)
	}

	log.Printf("FlowScheduleProvider.GetConfigs() returning %d configs", len(configs))
	return configs, nil
}

// NewSchedulePreEnqueueFunc creates a PreEnqueueFunc that can be used with asynq.ScheduleOpts
// This function retrieves node details before task execution using Core's GetNodesByNames method
func NewSchedulePreEnqueueFunc(core *core.Core) func(task *asynq.Task, opts []asynq.Option) {
	return func(task *asynq.Task, opts []asynq.Option) {
		// Only process flow execution tasks
		if task.Type() != tasks.TypeFlowExecution {
			return
		}

		// Parse the task payload to get flow information
		var payload tasks.FlowExecutionPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			log.Printf("failed to unmarshal flow execution payload: %v", err)
			return
		}
		payload.ExecID = uuid.NewString()

		namespaceUUID, err := uuid.Parse(payload.NamespaceID)
		if err != nil {
			log.Printf("invalid namespace UUID: %v", err)
			return
		}

		var nodeNames []string
		for _, action := range payload.Workflow.Actions {
			for _, node := range action.On {
				if node.Name != "" {
					nodeNames = append(nodeNames, node.Name)
				}
			}
		}

		if len(nodeNames) == 0 {
			updatedPayload, err := json.Marshal(payload)
			if err != nil {
				log.Printf("failed to marshal updated payload: %v", err)
				return
			}
			newTask := asynq.NewTask(task.Type(), updatedPayload)
			*task = *newTask
			return
		}

		coreNodes, err := core.GetNodesByNames(context.Background(), nodeNames, namespaceUUID)
		if err != nil {
			log.Printf("failed to get nodes: %v", err)
			return
		}

		taskNodes := models.NodesToTaskNodesModel(coreNodes)

		nodeMap := make(map[string]tasks.Node)
		for _, node := range taskNodes {
			nodeMap[node.Name] = node
		}

		// Update the flow with resolved nodes
		for i := range payload.Workflow.Actions {
			for j := range payload.Workflow.Actions[i].On {
				if nodeName := payload.Workflow.Actions[i].On[j].Name; nodeName != "" {
					if resolvedNode, exists := nodeMap[nodeName]; exists {
						payload.Workflow.Actions[i].On[j] = resolvedNode
					}
				}
			}
		}

		updatedPayload, err := json.Marshal(payload)
		if err != nil {
			log.Printf("failed to marshal updated payload: %v", err)
			return
		}

		newTask := asynq.NewTask(task.Type(), updatedPayload)
		*task = *newTask
	}
}
