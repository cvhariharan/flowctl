CREATE TABLE execution_log (
    id SERIAL PRIMARY KEY,
    exec_id VARCHAR(36) NOT NULL,
    flow_id INTEGER NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
    version INTEGER NOT NULL DEFAULT 0,
    context JSONB NOT NULL DEFAULT '{}'::jsonb,
    error TEXT,
    current_action_id TEXT,
    status execution_status NOT NULL DEFAULT 'pending',
    trigger_type trigger_type NOT NULL DEFAULT 'manual',
    triggered_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    namespace_id INTEGER NOT NULL REFERENCES namespaces(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    action_retries JSONB DEFAULT '{}'::jsonb,
    scheduled_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    action_states JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_execution_log_exec_id ON execution_log(exec_id);
CREATE UNIQUE INDEX idx_execution_log_exec_id_version ON execution_log(exec_id, version);
CREATE INDEX idx_execution_log_triggered_by ON execution_log(triggered_by);

INSERT INTO execution_log (
    exec_id, flow_id, version, context, error, status, trigger_type, triggered_by,
    namespace_id, created_at, updated_at, completed_at, scheduled_at, started_at
)
SELECT exec_id, flow_id, GREATEST(attempt - 1, 0),
       jsonb_build_object('inputs', inputs, 'outputs', outputs), error, status,
       trigger_type, triggered_by, namespace_id, created_at, updated_at,
       completed_at, scheduled_at, started_at
FROM executions;

ALTER TABLE approvals ADD COLUMN exec_log_id INTEGER;
UPDATE approvals a SET exec_log_id = el.id FROM execution_log el WHERE a.exec_id = el.exec_id;
ALTER TABLE approvals DROP CONSTRAINT approvals_exec_id_fkey;
DROP INDEX idx_approvals_exec_action;
ALTER TABLE approvals DROP COLUMN exec_id;
ALTER TABLE approvals ALTER COLUMN exec_log_id SET NOT NULL;
ALTER TABLE approvals ADD CONSTRAINT approvals_exec_log_id_fkey
    FOREIGN KEY (exec_log_id) REFERENCES execution_log(id) ON DELETE CASCADE;
CREATE UNIQUE INDEX idx_approvals_exec_action_id ON approvals(exec_log_id, action_id);

DROP TABLE execution_events;
DROP TABLE executions;
DROP TYPE execution_event_type;
