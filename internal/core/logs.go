package core

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/cvhariharan/autopilot/internal/core/models"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// StreamLogs reads values from a redis stream from the beginning and returns a channel to which
// all the messages are sent. logID is the ID sent to the NewFlowExecution task
func (c *Core) StreamLogs(ctx context.Context, logID string, namespaceID string) (chan models.StreamMessage, error) {
	ch := make(chan models.StreamMessage)

	errCh, err := c.checkErrors(ctx, logID, namespaceID)
	if err != nil {
		return nil, fmt.Errorf("error getting execution %s errors: %w", logID, err)
	}

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

		for errCh != nil || approvalCh != nil || logCh != nil {
			select {
			case <-ctx.Done():
				return
			case errMsg, ok := <-errCh:
				if !ok {
					errCh = nil
					continue
				}
				ch <- errMsg
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

	exec, err := c.GetExecutionByExecID(ctx, execID, namespaceID)
	if err != nil {
		return nil, err
	}

	go func(ch chan models.StreamMessage) {
		defer close(ch)
		lastProcessedID := "0"

		// used to decide when to close the stream
		// a close message is used to signify an end of stream but all the children write to the same stream
		// this can create many close messages so the stream should only be closed when the last child sends a close message
		closeCount := exec.Version
		for {
			result, err := c.redisClient.XRead(ctx, &redis.XReadArgs{
				Streams: []string{execID, lastProcessedID},
				Count:   200,
				Block:   0,
			}).Result()

			if err != nil {
				log.Println(err)
				if err == redis.Nil {
					continue
				}
				ch <- models.StreamMessage{MType: models.ErrMessageType, Val: []byte(fmt.Errorf("error reading from stream: %w", err).Error())}
				return
			}

			for _, stream := range result {
				for _, message := range stream.Messages {
					if _, ok := message.Values["closed"]; ok {
						if closeCount == 0 {
							return
						}
						closeCount -= 1
					}

					if checkpoint, ok := message.Values["checkpoint"]; ok {
						var sm models.StreamMessage
						if err := sm.UnmarshalBinary([]byte(checkpoint.(string))); err != nil {
							log.Println(err)
							continue
						}

						if !ok {
							log.Printf("checkpoint not of StreamMessage type: %T", checkpoint)
							continue
						}

						ch <- sm
					}

					lastProcessedID = message.ID
				}
			}

			time.Sleep(1 * time.Second)
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
				a, err := c.GetPendingApprovalsForExec(ctx, execID, namespaceID)
				if err != nil && !errors.Is(err, ErrNil) {
					ch <- models.StreamMessage{MType: models.ErrMessageType, Val: []byte(err.Error())}
					return
				}

				if a.Status == "pending" {
					ch <- models.StreamMessage{MType: models.ApprovalMessageType, Val: []byte(a.UUID)}
					return
				}
			}
			time.Sleep(5 * time.Second)
		}
	}(f, ch)

	return ch, nil
}
