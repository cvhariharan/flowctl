package core

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cvhariharan/autopilot/internal/models"
	"github.com/redis/go-redis/v9"
)

// StreamLogs reads values from a redis stream from the beginning and returns a channel to which
// all the messages are sent. logID is the ID sent to the NewFlowExecution task
func (c *Core) StreamLogs(ctx context.Context, logID string) (chan models.StreamMessage, error) {
	ch := make(chan models.StreamMessage)

	errCh, err := c.checkErrors(ctx, logID)
	if err != nil {
		return nil, err
	}

	logCh, err := c.streamLogs(ctx, logID)
	if err != nil {
		return nil, err
	}

	go func(ch chan models.StreamMessage) {
		defer func() {
			// copy pending log messages
			for logMsg := range logCh {
				ch <- logMsg
			}
			close(ch)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case errMsg, ok := <-errCh:
				if ok {
					ch <- errMsg
				}
				return
			default:
				ch <- <-logCh
			}
		}
	}(ch)

	return ch, nil
}

func (c *Core) streamLogs(ctx context.Context, execID string) (chan models.StreamMessage, error) {
	ch := make(chan models.StreamMessage)

	go func(ch chan models.StreamMessage) {
		defer close(ch)
		lastProcessedID := "0"
		for {
			result, err := c.redisClient.XRead(ctx, &redis.XReadArgs{
				Streams: []string{execID, lastProcessedID},
				Count:   10,
				Block:   0,
			}).Result()

			if err != nil {
				if err == redis.Nil {
					continue
				}
				ch <- models.StreamMessage{MType: models.ErrMessageType, Val: []byte(fmt.Errorf("error reading from stream: %w", err).Error())}
				return
			}

			for _, stream := range result {
				for _, message := range stream.Messages {
					if _, ok := message.Values["closed"]; ok {
						return
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

func (c *Core) checkErrors(ctx context.Context, execID string) (chan models.StreamMessage, error) {
	ch := make(chan models.StreamMessage)

	go func(ch chan models.StreamMessage) {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				exec, err := c.store.GetExecutionByExecID(ctx, execID)
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
		}
	}(ch)

	return ch, nil
}
