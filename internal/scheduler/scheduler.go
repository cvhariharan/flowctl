package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/cvhariharan/flowctl/internal/scheduler/storage"
)

const (
	TaskTicker     = 2 * time.Second
	PeriodicTicker = 1 * time.Minute
)

type TaskScheduler interface {
	QueueTask(ctx context.Context, payloadType PayloadType, execID string, payload any) (string, error)
	QueueScheduledTask(ctx context.Context, payloadType PayloadType, execID string, payload any, scheduledAt time.Time) (string, error)
	CancelTask(ctx context.Context, execID string) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Scheduler implements TaskScheduler
type Scheduler struct {
	jobStore         storage.Storage
	handlers         *handlerRegistry
	queueConfig      QueueConfig
	workerCount      int
	cronSyncInterval time.Duration
	jobSyncer        JobSyncerFn

	cancelFuncs   map[string]context.CancelFunc
	cancelMu      sync.RWMutex
	scheduledJobs map[string]ScheduledJob
	scheduledMu   sync.RWMutex

	taskTicker     *time.Ticker
	periodicTicker *time.Ticker
	cronSyncTicker *time.Ticker
	stopCh         chan struct{}
	stopped        bool
	logger         *slog.Logger
}

// SchedulerBuilder provides an interface for building schedulers
type SchedulerBuilder struct {
	jobStore         storage.Storage
	handlers         []Handler
	queueConfig      QueueConfig
	workerCount      int
	cronSyncInterval time.Duration
	jobSyncer        JobSyncerFn
	logger           *slog.Logger
}

// NewSchedulerBuilder creates a new scheduler builder
func NewSchedulerBuilder(logger *slog.Logger) *SchedulerBuilder {
	return &SchedulerBuilder{
		logger: logger,
	}
}

// WithJobStore sets the job store
func (b *SchedulerBuilder) WithJobStore(jobStore storage.Storage) *SchedulerBuilder {
	b.jobStore = jobStore
	return b
}

// WithHandler adds a task handler
func (b *SchedulerBuilder) WithHandler(h Handler) *SchedulerBuilder {
	b.handlers = append(b.handlers, h)
	return b
}

// WithQueueConfig sets the queue configuration
func (b *SchedulerBuilder) WithQueueConfig(cfg QueueConfig) *SchedulerBuilder {
	b.queueConfig = cfg
	return b
}

// WithWorkerCount sets the worker count
func (b *SchedulerBuilder) WithWorkerCount(c int) *SchedulerBuilder {
	b.workerCount = c
	return b
}

// WithCronSyncInterval sets the cron sync interval
func (b *SchedulerBuilder) WithCronSyncInterval(s time.Duration) *SchedulerBuilder {
	b.cronSyncInterval = s
	return b
}

// Build creates the scheduler instance
func (b *SchedulerBuilder) Build() (*Scheduler, error) {
	if b.jobStore == nil {
		return nil, fmt.Errorf("job store is required")
	}

	workerCount := b.workerCount
	if workerCount == 0 {
		workerCount = runtime.NumCPU()
	}

	cronInterval := b.cronSyncInterval
	if cronInterval == 0 {
		cronInterval = 5 * time.Minute
	}

	return &Scheduler{
		jobStore:         b.jobStore,
		handlers:         newHandlerRegistry(),
		queueConfig:      b.queueConfig,
		workerCount:      workerCount,
		cronSyncInterval: cronInterval,
		jobSyncer:        b.jobSyncer,
		cancelFuncs:      make(map[string]context.CancelFunc),
		scheduledJobs:    make(map[string]ScheduledJob),
		stopCh:           make(chan struct{}),
		logger:           b.logger,
	}, nil
}

// SetJobSyncer sets the job syncer for cron-based scheduling
func (s *Scheduler) SetJobSyncer(syncer JobSyncerFn) {
	s.jobSyncer = syncer
}

// SetHandler registers a handler for a payload type
func (s *Scheduler) SetHandler(h Handler) error {
	return s.handlers.Register(h)
}

// SetQueueConfig sets the queue configuration
func (s *Scheduler) SetQueueConfig(cfg QueueConfig) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid queue config: %w", err)
	}
	s.queueConfig = cfg
	return nil
}

// Start begins the scheduler's task processing loops
func (s *Scheduler) Start(ctx context.Context) error {
	if s.stopped {
		return nil
	}

	s.logger.Debug("starting scheduler task processing", "workers", s.workerCount, "cronsyncinterval", s.cronSyncInterval)

	if err := s.jobStore.Initialize(ctx); err != nil {
		return err
	}

	s.taskTicker = time.NewTicker(TaskTicker)

	// Check periodic tasks every minute
	s.periodicTicker = time.NewTicker(PeriodicTicker)

	// Sync crons from DB every 5 minutes
	s.cronSyncTicker = time.NewTicker(s.cronSyncInterval)

	if err := s.syncScheduledJobs(ctx); err != nil {
		s.logger.Error("failed to perform initial sync of scheduled jobs", "error", err)
	}

	go s.processLoop(ctx)

	return nil
}

// Stop shuts down the scheduler
func (s *Scheduler) Stop(ctx context.Context) error {
	if s.stopped {
		return nil
	}

	s.stopped = true
	close(s.stopCh)

	if s.taskTicker != nil {
		s.taskTicker.Stop()
	}
	if s.periodicTicker != nil {
		s.periodicTicker.Stop()
	}
	if s.cronSyncTicker != nil {
		s.cronSyncTicker.Stop()
	}

	for _, cancel := range s.cancelFuncs {
		cancel()
	}

	return nil
}

// QueueTask queues a task for execution with specified payload type
func (s *Scheduler) QueueTask(ctx context.Context, payloadType PayloadType, execID string, payload any) (string, error) {
	job, err := storage.NewJob(execID, string(payloadType), payload)
	if err != nil {
		return "", err
	}

	err = s.jobStore.Put(ctx, job)
	if err != nil {
		return "", err
	}

	return execID, nil
}

// QueueScheduledTask queues a task for delayed execution at the specified time
func (s *Scheduler) QueueScheduledTask(ctx context.Context, payloadType PayloadType, execID string, payload any, scheduledAt time.Time) (string, error) {
	if scheduledAt.Before(time.Now()) {
		return "", fmt.Errorf("scheduled_at must be in the future")
	}

	job, err := storage.NewScheduledJob(execID, string(payloadType), payload, scheduledAt)
	if err != nil {
		return "", err
	}

	err = s.jobStore.Put(ctx, job)
	if err != nil {
		return "", err
	}

	s.logger.Info("queued scheduled task", "execID", execID, "scheduledAt", scheduledAt)
	return execID, nil
}

// CancelTask cancels a running or pending execution
func (s *Scheduler) CancelTask(ctx context.Context, execID string) error {
	s.cancelMu.Lock()
	if cancel, exists := s.cancelFuncs[execID]; exists {
		cancel()
		delete(s.cancelFuncs, execID)
	}
	s.cancelMu.Unlock()

	go func() {
		if err := s.jobStore.CancelByExecID(context.Background(), execID); err != nil {
			s.logger.Error("error cancelling exec", "exec_id", execID, "error", err)
		}
	}()

	return nil
}

// processLoop runs the main processing loop
func (s *Scheduler) processLoop(ctx context.Context) {
	for {
		select {
		case <-s.taskTicker.C:
			if err := s.processPendingTasks(ctx); err != nil {
				s.logger.Error("error processing pending tasks", "error", err)
			}
		case <-s.periodicTicker.C:
			if err := s.checkPeriodicTasks(ctx); err != nil {
				s.logger.Error("error checking periodic tasks", "error", err)
			}
		case <-s.cronSyncTicker.C:
			if err := s.syncScheduledJobs(ctx); err != nil {
				s.logger.Error("error syncing scheduled jobs", "error", err)
			}
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// processPendingTasks gets pending tasks and executes them with weighted distribution
func (s *Scheduler) processPendingTasks(ctx context.Context) error {
	for _, qw := range s.queueConfig.Queues {
		handler, ok := s.handlers.Get(qw.PayloadType)
		if !ok {
			continue
		}

		goroutineCount := s.queueConfig.GetWorkerCount(qw.PayloadType, s.workerCount)

		for i := 0; i < goroutineCount; i++ {
			done := make(chan struct{})
			job, err := s.jobStore.GetByPayloadType(ctx, string(qw.PayloadType), done)
			if err != nil {
				if errors.Is(err, storage.ErrNoJobs) {
					break
				}
				return err
			}

			go func(done chan struct{}, j storage.Job, h Handler) {
				defer close(done)

				// Create cancellable context for this job
				execCtx, cancel := context.WithCancel(ctx)

				// Track cancellation function
				s.cancelMu.Lock()
				s.cancelFuncs[j.ExecID] = cancel
				s.cancelMu.Unlock()

				defer func() {
					s.cancelMu.Lock()
					delete(s.cancelFuncs, j.ExecID)
					s.cancelMu.Unlock()
				}()

				handlerJob := Job{
					ID:          j.ID,
					ExecID:      j.ExecID,
					PayloadType: PayloadType(j.PayloadType),
					Payload:     j.Payload,
					CreatedAt:   j.CreatedAt,
					ScheduledAt: j.ScheduledAt,
				}

				s.logger.Debug("starting job execution", "execID", j.ExecID, "type", j.PayloadType, "jobID", j.ID)
				if err := h.Handle(execCtx, handlerJob); err != nil {
					s.logger.Error("handler error", "type", j.PayloadType, "execID", j.ExecID, "error", err)
				}
				s.logger.Debug("completed job execution", "execID", j.ExecID, "type", j.PayloadType, "jobID", j.ID)
			}(done, job, handler)
		}
	}

	return nil
}
