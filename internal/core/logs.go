package core

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"encoding/json"

	"github.com/cvhariharan/flowctl/internal/core/models"
)

const (
	ExecutionLogPendingTimeout = 30 * time.Second
)

func (c *Core) getActionRetries(ctx context.Context, execID string, namespaceID string) map[string]int32 {
	state, err := c.loadExecutionState(ctx, execID)
	if err != nil {
		log.Printf("failed to load action state for exec %s, using empty map: %v", execID, err)
		return make(map[string]int32)
	}

	actionRetries := make(map[string]int32, len(state.Actions))
	for id, action := range state.Actions {
		actionRetries[id] = action.Attempt
	}

	return actionRetries
}

// DownloadLogs writes the raw log files for the given execID to w.
// Returns an error if the execution is still running or does not belong to the namespace.
func (c *Core) DownloadLogs(ctx context.Context, execID string, namespaceID string, w io.Writer) error {
	exec, err := c.GetExecutionSummaryByExecID(ctx, execID, namespaceID)
	if err != nil {
		return fmt.Errorf("could not get execution: %w", err)
	}

	switch exec.Status {
	case models.ExecutionStatusCompleted, models.ExecutionStatusErrored, models.ExecutionStatusCancelled:
		// ok to download
	default:
		return fmt.Errorf("execution %s is still running, download is only available for completed executions", execID)
	}

	return c.LogManager.GetRawLogs(ctx, execID, w)
}

// StreamLogs reads values from a stream from the beginning and returns a channel to which
// all the messages are sent. logID is the ID sent to the NewFlowExecution task
func (c *Core) StreamLogs(ctx context.Context, logID string, namespaceID string) (chan models.StreamMessage, error) {
	ch := make(chan models.StreamMessage)

	logCh, err := c.streamLogs(ctx, logID, namespaceID)
	if err != nil {
		return nil, fmt.Errorf("error reading logs for execution %s: %w", logID, err)
	}

	approvalCh, err := c.checkApprovalRequests(ctx, logID, namespaceID)
	if err != nil {
		return nil, fmt.Errorf("error getting approval requests for execution %s: %w", logID, err)
	}

	go func(ch chan models.StreamMessage) {
		defer close(ch)

		for approvalCh != nil || logCh != nil {
			select {
			case <-ctx.Done():
				return
			case approvalReq, ok := <-approvalCh:
				if !ok {
					approvalCh = nil
					continue
				}
				ch <- approvalReq
			case logMsg, ok := <-logCh:
				if !ok {
					logCh = nil
					continue
				}
				ch <- logMsg
			}
		}
	}(ch)

	return ch, nil
}

// streamLogs reads log messages and results from a stream and writes to a channel
func (c *Core) streamLogs(ctx context.Context, execID string, namespaceID string) (chan models.StreamMessage, error) {
	ch := make(chan models.StreamMessage)

	go func(ch chan models.StreamMessage) {
		defer close(ch)

		// Wait until logger exists with timeout
		timeout := time.After(ExecutionLogPendingTimeout)

		// If exec already executed, go to streamloop directly
		exec, err := c.GetExecutionSummaryByExecID(ctx, execID, namespaceID)
		if err == nil {
			if exec.Status == models.ExecutionStatusCompleted ||
				exec.Status == models.ExecutionStatusErrored ||
				exec.Status == models.ExecutionStatusCancelled ||
				exec.Status == models.ExecutionStatusPendingApproval {
				goto streamLoop
			}
		}

		// Wait until timeout for running flows
		for {
			select {
			case <-ctx.Done():
				return
			case <-timeout:
				log.Printf("timeout waiting for logger %s to be created, attempting to read archived logs", execID)
				return
			default:
				if c.LogManager.LoggerExists(execID) {
					goto streamLoop
				}
			}
		}

	streamLoop:
		actionRetries := c.getActionRetries(ctx, execID, namespaceID)
		logCh, err := c.LogManager.StreamLogs(ctx, execID, actionRetries)
		if err != nil {
			log.Println(err)
			return
		}

		for msg := range logCh {
			var sm models.StreamMessage
			if err := json.Unmarshal([]byte(msg), &sm); err != nil {
				log.Println(err)
				continue
			}

			ch <- sm
		}
	}(ch)

	return ch, nil
}

func (c *Core) checkApprovalRequests(ctx context.Context, execID string, namespaceID string) (chan models.StreamMessage, error) {
	ch := make(chan models.StreamMessage)

	f, err := c.GetFlowFromLogID(execID, namespaceID)
	if err != nil {
		return nil, err
	}

	if !f.IsApprovalRequired() {
		return nil, nil
	}

	go func(ctx context.Context, f models.Flow, ch chan models.StreamMessage) {
		defer close(ch)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		// send reports false once the consumer is gone, so a dag execution with several pending
		// approvals cannot block this goroutine forever.
		send := func(msg models.StreamMessage) bool {
			select {
			case ch <- msg:
				return true
			case <-ctx.Done():
				return false
			}
		}

		for {
			approvals, err := c.GetApprovalRequestsForExec(ctx, execID, namespaceID)
			if err != nil {
				log.Println(err)
				send(models.StreamMessage{MType: models.ErrMessageType, Val: err.Error()})
				return
			}

			var pending bool
			for _, a := range approvals {
				switch a.Status {
				case models.ApprovalStatusPending:
					pending = true
					if !send(models.StreamMessage{MType: models.ApprovalMessageType, Val: a.UUID}) {
						return
					}
				case models.ApprovalStatusRejected:
					send(models.StreamMessage{MType: models.ErrMessageType, Val: "approval request has been rejected"})
					return
				}
			}

			// Every request has been decided and none were rejected
			if len(approvals) > 0 && !pending {
				return
			}

			// Wait for 5 seconds
			select {
			case <-ctx.Done():
				log.Println("approval context done")
				return
			case <-ticker.C:
			}
		}
	}(ctx, f, ch)

	return ch, nil
}
