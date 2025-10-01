package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// checkPeriodicTasks checks for flows with cron schedules that should run now
// Similar to FlowScheduleProvider.GetConfigs() but simplified for immediate execution
func (s *Scheduler) checkPeriodicTasks(ctx context.Context) error {
	// Get flows with cron schedules using existing GetScheduledFlows from store
	scheduledFlows, err := s.store.GetScheduledFlows(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	log.Printf("Checking %d scheduled flows for execution", len(scheduledFlows))

	for _, flow := range scheduledFlows {
		// Skip flows without cron schedule
		if !flow.CronSchedule.Valid || flow.CronSchedule.String == "" {
			continue
		}

		// Check if this flow should run now
		if s.shouldRunNow(flow.CronSchedule.String, now) {
			if err := s.createImmediateTaskFromFlow(ctx, flow); err != nil {
				log.Printf("Failed to create immediate task from scheduled flow %s: %v", flow.Slug, err)
			}
		}
	}

	return nil
}

// shouldRunNow evaluates if a cron expression should execute in the current minute
func (s *Scheduler) shouldRunNow(cronExpr string, now time.Time) bool {
	// Parse the cron expression
	schedule, err := cron.ParseStandard(cronExpr)
	if err != nil {
		log.Printf("Failed to parse cron expression '%s': %v", cronExpr, err)
		return false
	}

	// Check if we're within the current minute window for execution
	// We round down to the nearest minute to avoid multiple executions within the same minute
	currentMinute := now.Truncate(time.Minute)

	// Get the last scheduled time (previous minute) and check if next run falls in current minute
	lastMinute := currentMinute.Add(-time.Minute)
	nextRun := schedule.Next(lastMinute)

	// Task should run if the next scheduled time falls within the current minute
	return nextRun.Equal(currentMinute) || (nextRun.After(currentMinute) && nextRun.Before(currentMinute.Add(time.Minute)))
}

// createImmediateTaskFromFlow creates an immediate task from a scheduled flow
// This is a simplified version that creates a minimal payload
// The actual flow YAML will be loaded during execution via the flow_id
func (s *Scheduler) createImmediateTaskFromFlow(ctx context.Context, flow repo.GetScheduledFlowsRow) error {
	// Get namespace info
	namespace, err := s.store.GetNamespaceByUUID(ctx, flow.NamespaceUuid)
	if err != nil {
		log.Printf("Failed to get namespace for flow %s: %v", flow.Slug, err)
		return err
	}

	// Create minimal FlowExecutionPayload - the actual flow YAML will be loaded during execution
	payload := FlowExecutionPayload{
		Workflow: Flow{
			Meta: Metadata{
				DBID:      flow.ID,
				ID:        flow.Slug,
				Name:      flow.Name,
				Namespace: namespace.Name,
			},
			// Actions will be loaded from the flow YAML file during execution
			Inputs:  []Input{},
			Actions: []Action{},
		},
		Input:             make(map[string]interface{}),
		StartingActionIdx: 0,
		ExecID:            uuid.NewString(),
		NamespaceID:       namespace.Uuid.String(),
		TriggerType:       TriggerTypeScheduled,
		UserUUID:          "00000000-0000-0000-0000-000000000000", // System user
	}

	// Queue the task as immediate
	_, err = s.QueueTask(ctx, payload)
	if err != nil {
		return err
	}

	log.Printf("Created immediate task from scheduled flow %s (ID: %d) with schedule %s", flow.Slug, flow.ID, flow.CronSchedule.String)
	return nil
}

// Helper function to validate cron expressions
func ValidateCronExpression(cronExpr string) error {
	_, err := cron.ParseStandard(cronExpr)
	return err
}
