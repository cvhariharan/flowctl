package core

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"encoding/json"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/google/uuid"
)

const (
	ExecutionLogPendingTimeout = 30 * time.Second
)

// StreamLogs reads values from a redis stream from the beginning and returns a channel to which
// all the messages are sent. logID is the ID sent to the NewFlowExecution task
func (c *Core) StreamLogs(ctx context.Context, logID string, namespaceID string) (chan models.StreamMessage, error) {
	ch := make(chan models.StreamMessage)

	// Remove because the log messages will already contain error messages
	// errCh, err := c.checkErrors(ctx, logID, namespaceID)
	// if err != nil {
	// 	return nil, fmt.Errorf("error getting execution %s errors: %w", logID, err)
	// }

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
			// case errMsg, ok := <-errCh:
			// 	if !ok {
			// 		errCh = nil
			// 		continue
			// 	}
			// 	ch <- errMsg
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

// streamLogs reads log messages and results from a redis stream and writes to a channel
func (c *Core) streamLogs(ctx context.Context, execID string, namespaceID string) (chan models.StreamMessage, error) {
	ch := make(chan models.StreamMessage)

	go func(ch chan models.StreamMessage) {
		defer close(ch)

		// Wait until logger exists with timeout
		timeout := time.After(ExecutionLogPendingTimeout)
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		exec, err := c.GetExecutionSummaryByExecID(ctx, execID, namespaceID)
		if err == nil {
			if exec.Status == models.ExecutionStatusCompleted ||
				exec.Status == models.ExecutionStatusErrored ||
				exec.Status == models.ExecutionStatusCancelled {
				goto streamLoop
			}
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-timeout:
				log.Printf("timeout waiting for logger %s to be created, attempting to read archived logs", execID)
				return
			case <-ticker.C:
				if c.LogManager.LoggerExists(execID) {
					goto streamLoop
				}
			}
		}

	streamLoop:
		logCh, err := c.LogManager.StreamLogs(ctx, execID)
		if err != nil {
			log.Println(err)
			return
		}

		for msg := range logCh {
			log.Println("test", msg)
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

func (c *Core) checkErrors(ctx context.Context, execID string, namespaceID string) (chan models.StreamMessage, error) {
	ch := make(chan models.StreamMessage)

	go func(ch chan models.StreamMessage) {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				namespaceUUID, err := uuid.Parse(namespaceID)
				if err != nil {
					ch <- models.StreamMessage{MType: models.ErrMessageType, Val: []byte(fmt.Errorf("invalid namespace UUID: %w", err).Error())}
					return
				}
				exec, err := c.store.GetExecutionByExecID(ctx, repo.GetExecutionByExecIDParams{
					ExecID: execID,
					Uuid:   namespaceUUID,
				})
				if err != nil {
					ch <- models.StreamMessage{MType: models.ErrMessageType, Val: []byte(fmt.Errorf("error reading task status: %w", err).Error())}
					return
				}

				if exec.Error.Valid {
					ch <- models.StreamMessage{MType: models.ErrMessageType, Val: []byte(exec.Error.String)}
				}

				if exec.Status == "completed" || exec.Status == "errored" {
					return
				}
			}
			time.Sleep(5 * time.Second)
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

	go func(f models.Flow, ch chan models.StreamMessage) {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				a, err := c.GetApprovalsRequestsForExec(ctx, execID, namespaceID)
				if err != nil && !errors.Is(err, ErrNil) {
					log.Println(err)
					ch <- models.StreamMessage{MType: models.ErrMessageType, Val: []byte(err.Error())}
					return
				}

				if a.Status == "pending" {
					ch <- models.StreamMessage{MType: models.ApprovalMessageType, Val: []byte(a.UUID)}
					return
				} else if a.Status == "rejected" {
					ch <- models.StreamMessage{MType: models.ErrMessageType, Val: []byte("approval request has been rejected")}
					return
				}
				log.Printf("approval request: %v", a)
			}
			time.Sleep(5 * time.Second)
		}
	}(f, ch)

	return ch, nil
}
