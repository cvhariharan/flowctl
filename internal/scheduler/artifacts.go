package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/cvhariharan/flowctl/internal/storage"
)

type ArtifactManager struct {
	store         repo.Store
	storage       storage.Storage
	retentionTime time.Duration
	scanInterval  time.Duration
	logger        *slog.Logger
}

func NewArtifactManager(store repo.Store, storage storage.Storage, retentionTime time.Duration, scanInterval time.Duration, logger *slog.Logger) *ArtifactManager {
	if scanInterval <= 0 {
		scanInterval = time.Hour
	}
	return &ArtifactManager{
		store:         store,
		storage:       storage,
		retentionTime: retentionTime,
		scanInterval:  scanInterval,
		logger:        logger,
	}
}

func (m *ArtifactManager) Run(ctx context.Context) {
	if m.retentionTime <= 0 || m.storage == nil {
		m.logger.Info("artifact retention manager is disabled or storage is not configured")
		return
	}

	m.logger.Info("starting artifact retention manager", "retentionTime", m.retentionTime, "scanInterval", m.scanInterval)

	ticker := time.NewTicker(m.scanInterval)
	defer ticker.Stop()

	// Run initially on startup
	if err := m.cleanOldArtifacts(ctx); err != nil {
		m.logger.Error("failed to clean old artifacts", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("stopping artifact retention manager")
			return
		case <-ticker.C:
			if err := m.cleanOldArtifacts(ctx); err != nil {
				m.logger.Error("failed to clean old artifacts", "error", err)
			}
		}
	}
}

func (m *ArtifactManager) cleanOldArtifacts(ctx context.Context) error {
	cutoffTime := time.Now().Add(-m.retentionTime)
	m.logger.Debug("running artifact retention scan", "cutoffTime", cutoffTime)

	expiredExecs, err := m.store.GetFinishedExecutionsOlderThan(ctx, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to query expired executions: %w", err)
	}

	if len(expiredExecs) == 0 {
		return nil
	}

	m.logger.Info("found expired executions to clean up artifacts", "count", len(expiredExecs))

	for _, exec := range expiredExecs {
		// prefix: <flow_slug>/<exec_id>/
		prefix := fmt.Sprintf("%s/%s/", exec.FlowSlug, exec.ExecID)
		m.logger.Info("deleting expired artifacts", "execID", exec.ExecID, "prefix", prefix)

		if err := m.storage.Delete(ctx, prefix); err != nil {
			m.logger.Error("failed to delete expired artifacts", "execID", exec.ExecID, "prefix", prefix, "error", err)
		}
	}

	return nil
}
