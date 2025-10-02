package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// syncScheduledFlows syncs scheduled flows from the database into the in-memory cache
func (s *Scheduler) syncScheduledFlows(ctx context.Context) error {
	scheduledFlows, err := s.store.GetScheduledFlows(ctx)
	if err != nil {
		return err
	}

	s.scheduledMu.Lock()
	defer s.scheduledMu.Unlock()

	// Clear and rebuild the map
	s.scheduledFlows = make(map[string]repo.GetScheduledFlowsRow)
	for _, flow := range scheduledFlows {
		if flow.CronSchedule.Valid && flow.CronSchedule.String != "" {
			s.scheduledFlows[flow.Slug] = flow
		}
	}

	s.logger.Debug("synced scheduled flows to cache", "count", len(s.scheduledFlows))
	return nil
}

// checkPeriodicTasks checks for flows with cron schedules that should run now
func (s *Scheduler) checkPeriodicTasks(ctx context.Context) error {
	s.scheduledMu.RLock()
	scheduledFlows := make([]repo.GetScheduledFlowsRow, 0, len(s.scheduledFlows))
	for _, flow := range s.scheduledFlows {
		scheduledFlows = append(scheduledFlows, flow)
	}
	s.scheduledMu.RUnlock()

	now := time.Now()

	for _, flow := range scheduledFlows {
		if s.shouldRunNow(flow.CronSchedule.String, now) {
			if err := s.createImmediateTaskFromFlow(ctx, flow); err != nil {
				s.logger.Error("failed to create immediate task from scheduled flow", "flow", flow.Name, "error", err)
			}
		}
	}

	return nil
}

// shouldRunNow evaluates if a cron expression should execute in the current minute
func (s *Scheduler) shouldRunNow(cronExpr string, now time.Time) bool {
	schedule, err := cron.ParseStandard(cronExpr)
	if err != nil {
		log.Printf("Failed to parse cron expression '%s': %v", cronExpr, err)
		return false
	}

	currentMinute := now.Truncate(time.Minute)

	lastMinute := currentMinute.Add(-time.Minute)
	nextRun := schedule.Next(lastMinute)

	// Task should run if the next scheduled time falls within the current minute
	return nextRun.Equal(currentMinute) || (nextRun.After(currentMinute) && nextRun.Before(currentMinute.Add(time.Minute)))
}

// createImmediateTaskFromFlow creates an immediate task from a scheduled flow
func (s *Scheduler) createImmediateTaskFromFlow(ctx context.Context, flow repo.GetScheduledFlowsRow) error {
	namespace, err := s.store.GetNamespaceByUUID(ctx, flow.NamespaceUuid)
	if err != nil {
		return fmt.Errorf("could not create periodic task %s: %w", flow.Name, err)
	}

	if s.flowLoader == nil {
		return fmt.Errorf("flow loader not configured")
	}

	schedulerFlow, err := s.flowLoader(ctx, flow.Slug, flow.NamespaceUuid.String())
	if err != nil {
		log.Printf("Failed to load flow %s: %v", flow.Slug, err)
		return err
	}

	input := applyDefaultInputValues(schedulerFlow.Inputs)

	payload := FlowExecutionPayload{
		Workflow:          schedulerFlow,
		Input:             input,
		StartingActionIdx: 0,
		ExecID:            uuid.NewString(),
		NamespaceID:       namespace.Uuid.String(),
		TriggerType:       TriggerTypeScheduled,
		UserUUID:          "00000000-0000-0000-0000-000000000000", // System user
	}

	_, err = s.QueueTask(ctx, payload)
	if err != nil {
		return err
	}

	log.Printf("Created immediate task from scheduled flow %s (ID: %d) with schedule %s", flow.Slug, flow.ID, flow.CronSchedule.String)
	return nil
}

// applyDefaultInputValues creates an input map using default values from flow inputs
func applyDefaultInputValues(inputs []Input) map[string]interface{} {
	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Default != "" {
			result[input.Name] = input.Default
		}
	}
	return result
}
