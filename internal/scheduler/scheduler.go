package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/cvhariharan/flowctl/internal/scheduler/storage"
	"github.com/cvhariharan/flowctl/internal/streamlogger"
	"github.com/google/uuid"
)

type TaskScheduler interface {
	QueueTask(ctx context.Context, payload FlowExecutionPayload) (string, error)
	CancelTask(ctx context.Context, execID string) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Scheduler implements TaskScheduler
type Scheduler struct {
	store           repo.Store      // For flow metadata and cron schedules
	jobStore        storage.Storage // For job queue
	secretsProvider SecretsProviderFn
	logmanager      streamlogger.LogManager
	cancelFuncs     map[string]context.CancelFunc
	mu              sync.RWMutex
	taskTicker      *time.Ticker
	periodicTicker  *time.Ticker
	cronSyncTicker  *time.Ticker
	stopCh          chan struct{}
	stopped         bool
	workerCount     int
	logger          *slog.Logger
}

// SchedulerBuilder provides a fluent interface for building schedulers
type SchedulerBuilder struct {
	store           repo.Store
	jobStore        storage.Storage
	secretsProvider SecretsProviderFn
	logmanager      streamlogger.LogManager
	workerCount     int
	logger          *slog.Logger
}

// NewSchedulerBuilder creates a new scheduler builder
func NewSchedulerBuilder(logger *slog.Logger) *SchedulerBuilder {
	return &SchedulerBuilder{
		logger: logger,
	}
}

// WithStore sets the store
func (b *SchedulerBuilder) WithStore(store repo.Store) *SchedulerBuilder {
	b.store = store
	return b
}

// WithJobStore sets the job store
func (b *SchedulerBuilder) WithJobStore(jobStore storage.Storage) *SchedulerBuilder {
	b.jobStore = jobStore
	return b
}

// WithSecretsProvider sets the secrets provider
func (b *SchedulerBuilder) WithSecretsProvider(sp SecretsProviderFn) *SchedulerBuilder {
	b.secretsProvider = sp
	return b
}

// WithLogManager sets the log manager
func (b *SchedulerBuilder) WithLogManager(lm streamlogger.LogManager) *SchedulerBuilder {
	b.logmanager = lm
	return b
}

func (b *SchedulerBuilder) WithWorkerCount(c int) *SchedulerBuilder {
	b.workerCount = c
	return b
}

// Build creates the scheduler instance
func (b *SchedulerBuilder) Build() (*Scheduler, error) {
	if b.workerCount == 0 {
		b.workerCount = runtime.NumCPU()
	}

	if b.logmanager == nil {
		return nil, fmt.Errorf("logmanager cannot be nil")
	}

	return &Scheduler{
		store:           b.store,
		jobStore:        b.jobStore,
		secretsProvider: b.secretsProvider,
		logmanager:      b.logmanager,
		workerCount:     b.workerCount,
		logger:          b.logger,
		cancelFuncs:     make(map[string]context.CancelFunc),
		stopCh:          make(chan struct{}),
	}, nil
}

// SetSecretsProvider allows updating secrets provider after build
func (s *Scheduler) SetSecretsProvider(sp SecretsProviderFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secretsProvider = sp
}

// Start begins the scheduler's task processing loops
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return nil
	}

	s.logger.Debug("starting scheduler task processing", "workers", s.workerCount)

	// Initialize the job store
	if err := s.jobStore.Initialize(ctx); err != nil {
		return err
	}

	// Check for immediate tasks
	s.taskTicker = time.NewTicker(2 * time.Second)

	// Check periodic tasks every minute
	s.periodicTicker = time.NewTicker(1 * time.Minute)

	// Sync crons from DB every
	s.cronSyncTicker = time.NewTicker(5 * time.Minute)

	go s.processLoop(ctx)

	return nil
}

// Stop shuts down the scheduler
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

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

	// Cancel all running executions
	for _, cancel := range s.cancelFuncs {
		cancel()
	}

	return nil
}

// QueueTask queues an immediate task for execution
func (s *Scheduler) QueueTask(ctx context.Context, payload FlowExecutionPayload) (string, error) {
	// Create job using the generic storage
	job, err := storage.NewJob(payload.ExecID, payload)
	if err != nil {
		return "", err
	}

	err = s.jobStore.Put(ctx, job)
	if err != nil {
		return "", err
	}

	return payload.ExecID, nil
}

// CancelTask cancels a running or pending execution
func (s *Scheduler) CancelTask(ctx context.Context, execID string) error {
	s.mu.Lock()
	if cancel, exists := s.cancelFuncs[execID]; exists {
		cancel()
		delete(s.cancelFuncs, execID)
	}
	s.mu.Unlock()

	// remove pending queue items
	return s.jobStore.CancelByExecID(ctx, execID)
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
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// processPendingTasks gets pending tasks and executes them
func (s *Scheduler) processPendingTasks(ctx context.Context) error {
	for i := 0; i < s.workerCount; i++ {
		job, err := s.jobStore.Get(ctx)
		if err != nil {
			if errors.Is(err, storage.ErrNoJobs) {
				break
			}
			return err
		}

		go func() {
			if err := s.executeJob(ctx, job); err != nil {
				s.logger.Error("error executing flow", "execID", job.ExecID, "error", err)
			}
		}()
	}

	return nil
}

// executeJob executes a single job
func (s *Scheduler) executeJob(ctx context.Context, job storage.Job) error {
	var payload FlowExecutionPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	execCtx, cancel := context.WithCancel(ctx)

	// Track cancellation function
	s.mu.Lock()
	s.cancelFuncs[job.ExecID] = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.cancelFuncs, job.ExecID)
		s.mu.Unlock()
		// Remove job from queue when done
		if err := s.jobStore.Delete(ctx, job.ID); err != nil {
			s.logger.Error("failed to delete job", "jobID", job.ID, "error", err)
		}
	}()

	// Create execution log for scheduled executions only (manual ones are created in core)
	if payload.TriggerType == TriggerTypeScheduled {
		if err := s.createExecutionLog(ctx, payload); err != nil {
			s.logger.Error("failed to create execution log for scheduled task", "error", err)
		}
	}

	if err := s.setStatus(ctx, payload.ExecID, repo.ExecutionStatusRunning, payload.NamespaceID, nil); err != nil {
		return fmt.Errorf("could not update execution_log status: %w", err)
	}

	if err := s.executeFlow(execCtx, payload); err != nil {
		if errors.Is(err, ErrPendingApproval) {
			return s.setStatus(ctx, payload.ExecID, repo.ExecutionStatusPendingApproval, payload.NamespaceID, nil)
		}
		if errors.Is(err, ErrExecutionCancelled) {
			return s.setStatus(ctx, payload.ExecID, repo.ExecutionStatusCancelled, payload.NamespaceID, nil)
		}
		return s.setStatus(ctx, payload.ExecID, repo.ExecutionStatusErrored, payload.NamespaceID, err)
	}

	return s.setStatus(ctx, payload.ExecID, repo.ExecutionStatusCompleted, payload.NamespaceID, nil)
}

// setStatus updates the execution status in the execution_log table
func (s *Scheduler) setStatus(ctx context.Context, execID string, status repo.ExecutionStatus, namespaceID string, err error) error {
	var errMsg sql.NullString
	if err != nil {
		errMsg = sql.NullString{String: err.Error(), Valid: true}
	}
	namespaceUUID, parseErr := uuid.Parse(namespaceID)
	if parseErr != nil {
		return fmt.Errorf("invalid namespace ID: %w", parseErr)
	}
	_, err = s.store.UpdateExecutionStatus(ctx, repo.UpdateExecutionStatusParams{
		Status: status,
		Error:  errMsg,
		ExecID: execID,
		Uuid:   namespaceUUID,
	})
	if err != nil {
		return fmt.Errorf("could not update error execution status: %w", err)
	}

	return nil
}

// createExecutionLog creates an execution log entry for executions
func (s *Scheduler) createExecutionLog(ctx context.Context, payload FlowExecutionPayload) error {
	namespaceUUID, err := uuid.Parse(payload.NamespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	userUUID, err := uuid.Parse(payload.UserUUID)
	if err != nil {
		return fmt.Errorf("invalid user UUID: %w", err)
	}

	inputB, err := json.Marshal(payload.Input)
	if err != nil {
		return fmt.Errorf("could not marshal input to json: %w", err)
	}

	// Convert trigger type
	triggerType := repo.TriggerTypeManual
	if payload.TriggerType == TriggerTypeScheduled {
		triggerType = repo.TriggerTypeScheduled
	}

	_, err = s.store.AddExecutionLog(ctx, repo.AddExecutionLogParams{
		ExecID:      payload.ExecID,
		FlowID:      payload.Workflow.Meta.DBID,
		Input:       inputB,
		TriggerType: triggerType,
		Uuid:        userUUID,
		Uuid_2:      namespaceUUID,
	})
	if err != nil {
		return fmt.Errorf("could not add execution log entry: %w", err)
	}

	return nil
}
