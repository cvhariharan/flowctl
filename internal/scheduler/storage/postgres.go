package storage

import (
	"context"
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// PostgresStorage implements the Storage interface using PostgreSQL
type PostgresStorage struct {
	db *sqlx.DB
}

// NewPostgresStorage creates a new PostgreSQL storage backend
func NewPostgresStorage(db *sqlx.DB) *PostgresStorage {
	return &PostgresStorage{db: db}
}

// Initialize creates the job queue table
func (p *PostgresStorage) Initialize(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS job_queue (
			id SERIAL PRIMARY KEY,
			exec_id TEXT NOT NULL,
			payload JSONB NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			is_locked BOOLEAN NOT NULL DEFAULT FALSE
		);

		-- Index for efficient queue operations
		CREATE INDEX IF NOT EXISTS idx_job_queue_pending ON job_queue(created_at) WHERE is_locked = false;
		CREATE INDEX IF NOT EXISTS idx_job_queue_exec_id ON job_queue(exec_id);
	`

	_, err := p.db.ExecContext(ctx, query)
	return err
}

// Put adds a job to the queue
func (p *PostgresStorage) Put(ctx context.Context, job Job) error {
	query := `
		INSERT INTO job_queue (exec_id, payload, created_at)
		VALUES ($1, $2, $3)
		RETURNING id
	`

	err := p.db.GetContext(ctx, &job.ID, query, job.ExecID, job.Payload, job.CreatedAt)
	return err
}

// Get retrieves and locks a job from the queue for processing
func (p *PostgresStorage) Get(ctx context.Context) (Job, error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback()

	// Lock and get the oldest pending job
	query := `
		UPDATE job_queue 
		SET is_locked = TRUE
		WHERE id = (
			SELECT id FROM job_queue 
			WHERE is_locked = FALSE
			ORDER BY created_at ASC 
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, exec_id, payload, created_at
	`

	var job Job
	err = tx.GetContext(ctx, &job, query)
	if err != nil {
		if err == sql.ErrNoRows {
			return Job{}, ErrNoJobs
		}
		return Job{}, err
	}

	if err = tx.Commit(); err != nil {
		return Job{}, err
	}

	return job, nil
}

// Delete removes a job from the queue
func (p *PostgresStorage) Delete(ctx context.Context, jobID int64) error {
	query := `DELETE FROM job_queue WHERE id = $1`
	_, err := p.db.ExecContext(ctx, query, jobID)
	return err
}

// CancelByExecID removes all jobs with the given execution ID
func (p *PostgresStorage) CancelByExecID(ctx context.Context, execID string) error {
	query := `DELETE FROM job_queue WHERE exec_id = $1`
	_, err := p.db.ExecContext(ctx, query, execID)
	return err
}

// Close closes the storage backend
func (p *PostgresStorage) Close() error {
	// The database connection is managed externally, so we don't close it here
	return nil
}
