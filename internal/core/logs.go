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

var ExecutionLogPendingTimeout = 30 * time.Second

type LogStreamEndReason string

const (
	LogStreamEndComplete LogStreamEndReason = "complete"
	LogStreamEndTimeout  LogStreamEndReason = "timeout"
	LogStreamEndError    LogStreamEndReason = "error"
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
func (c *Core) StreamLogs(ctx context.Context, logID string, namespaceID string) (<-chan models.StreamMessage, <-chan LogStreamEndReason, error) {
	ch := make(chan models.StreamMessage)
	endCh := make(chan LogStreamEndReason, 1)
	streamCtx, cancel := context.WithCancel(ctx)

	approvalCh, err := c.checkApprovalRequests(streamCtx, logID, namespaceID)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("error getting approval requests for execution %s: %w", logID, err)
	}

	logCh, logEndCh, err := c.streamLogs(streamCtx, logID, namespaceID)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("error reading logs for execution %s: %w", logID, err)
	}

	go func(ch chan models.StreamMessage) {
		defer close(ch)
		defer close(endCh)
		defer cancel()

		for {
			select {
			case <-streamCtx.Done():
				return
			case logReason, ok := <-logEndCh:
				if !ok {
					return
				}
				endCh <- logReason
				return
			case approvalReq, ok := <-approvalCh:
				if !ok {
					approvalCh = nil
					continue
				}
				select {
				case ch <- approvalReq:
				case <-streamCtx.Done():
					return
				}
			case logMsg, ok := <-logCh:
				if !ok {
					logCh = nil
					continue
				}
				select {
				case ch <- logMsg:
				case <-streamCtx.Done():
					return
				}
			}
		}
	}(ch)

	return ch, endCh, nil
}

// streamLogs reads log messages and results from a stream and writes to a channel
func (c *Core) streamLogs(ctx context.Context, execID string, namespaceID string) (<-chan models.StreamMessage, <-chan LogStreamEndReason, error) {
	ch := make(chan models.StreamMessage)
	endCh := make(chan LogStreamEndReason, 1)

	go func(ch chan models.StreamMessage) {
		defer close(endCh)

		if !c.waitForLogger(ctx, execID, namespaceID) {
			close(ch)
			if ctx.Err() == nil {
				log.Printf("timeout waiting for logger %s to be created", execID)
				endCh <- LogStreamEndTimeout
			}
			return
		}

		actionRetries := c.getActionRetries(ctx, execID, namespaceID)
		logCh, err := c.LogManager.StreamLogs(ctx, execID, actionRetries)
		if err != nil {
			log.Println(err)
			close(ch)
			endCh <- LogStreamEndError
			return
		}

		for msg := range logCh {
			var sm models.StreamMessage
			if err := json.Unmarshal([]byte(msg), &sm); err != nil {
				log.Println(err)
				continue
			}

			select {
			case ch <- sm:
			case <-ctx.Done():
				close(ch)
				return
			}
		}
		close(ch)
		endCh <- LogStreamEndComplete
	}(ch)

	return ch, endCh, nil
}

func (c *Core) waitForLogger(ctx context.Context, execID string, namespaceID string) bool {
	exec, err := c.GetExecutionSummaryByExecID(ctx, execID, namespaceID)
	if err == nil {
		switch exec.Status {
		case models.ExecutionStatusCompleted,
			models.ExecutionStatusErrored,
			models.ExecutionStatusCancelled,
			models.ExecutionStatusPendingApproval:
			return true
		}
	}

	if c.LogManager.LoggerExists(execID) {
		return true
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(ExecutionLogPendingTimeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return false
		case <-ticker.C:
			if c.LogManager.LoggerExists(execID) {
				return true
			}
		}
	}
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
