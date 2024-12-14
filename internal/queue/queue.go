package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/cvhariharan/autopilot/internal/flow"
	"github.com/cvhariharan/autopilot/internal/repo"
	"github.com/lib/pq"
)

const BATCH_INTERVAL = 10 * time.Second

type QueueItem struct {
	UUID   string
	FlowID int32
	Input  []byte
}

type Queue struct {
	store repo.Store
}

func NewQueue(r repo.Store) *Queue {
	return &Queue{
		store: r,
	}
}

func (q *Queue) Enqueue(ctx context.Context, f flow.Flow, input map[string]interface{}) (int32, error) {
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return -1, fmt.Errorf("error marshaling input to json: %w", err)
	}
	e, err := q.store.AddToQueue(ctx, repo.AddToQueueParams{
		FlowID: f.Meta.DBID,
		Input:  inputBytes,
	})

	if err != nil {
		return -1, fmt.Errorf("error adding flow to queue: %w", err)
	}

	return e.ID, err
}

func (q *Queue) ListenForJobs(ctx context.Context, listener *pq.Listener, count int) (<-chan QueueItem, error) {
	if count <= 0 {
		count = 1
	}
	ch := make(chan QueueItem, count)

	err := listener.Listen("new_flow")
	if err != nil {
		return nil, fmt.Errorf("error starting postgres listener for notifications: %w", err)
	}

	ticker := time.NewTicker(BATCH_INTERVAL)

	go func() error {
		for {
			select {
			// TODO: Refactor later
			// This could lead to starvation if there are new jobs arriving faster than we can process them
			// but since select cases are evaluated in random, this should be fine for now.
			case n := <-listener.Notify:
				eid, err := strconv.Atoi(n.Extra)
				if err != nil {
					log.Println(err)
					continue
				}
				job, err := q.store.DequeueByID(ctx, int32(eid))
				if err != nil {
					log.Println(err)
					continue
				}
				ch <- QueueItem{UUID: job.Uuid.String(), FlowID: job.FlowID, Input: job.Input}
			case <-ticker.C:
				jobs, err := q.store.Dequeue(ctx, int32(count))
				if err != nil {
					log.Println(err)
					continue
				}
				for _, job := range jobs {
					ch <- QueueItem{UUID: job.Uuid.String(), FlowID: job.FlowID, Input: job.Input}
				}
			}
		}
	}()
	return ch, nil
}
