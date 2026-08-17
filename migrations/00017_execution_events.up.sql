CREATE TYPE execution_event_type AS ENUM (
    'queued',
    'started',
    'waiting_approval',
    'completed',
    'errored',
    'cancelled',
    'action_started',
    'action_completed',
    'action_failed',
    'action_blocked',
    'action_skipped',
    'action_cancelled'
);

CREATE TABLE executions (
    id SERIAL PRIMARY KEY,
    exec_id VARCHAR(36) NOT NULL UNIQUE,
    flow_id INTEGER NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
    namespace_id INTEGER NOT NULL REFERENCES namespaces(id) ON DELETE CASCADE,
    triggered_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    trigger_type trigger_type NOT NULL DEFAULT 'manual',
    inputs JSONB NOT NULL DEFAULT '{}'::jsonb,
    scheduled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attempt INTEGER NOT NULL DEFAULT 0,
    status execution_status NOT NULL DEFAULT 'pending',
    error TEXT,
    outputs JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_executions_flow ON executions (flow_id, created_at DESC);
CREATE INDEX idx_executions_namespace ON executions (namespace_id, created_at DESC);
CREATE INDEX idx_executions_triggered_by ON executions (triggered_by);
CREATE INDEX idx_executions_scheduled ON executions (scheduled_at) WHERE scheduled_at IS NOT NULL;
CREATE INDEX idx_executions_active ON executions (flow_id)
    WHERE status IN ('pending', 'running', 'pending_approval');

CREATE TABLE execution_events (
    seq BIGSERIAL PRIMARY KEY,
    exec_id VARCHAR(36) NOT NULL REFERENCES executions(exec_id) ON DELETE CASCADE,
    attempt INTEGER NOT NULL,
    action_id VARCHAR(50),
    type execution_event_type NOT NULL,
    error TEXT,
    outputs JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT action_events_have_action_id CHECK (
        (type::text LIKE 'action\_%') = (action_id IS NOT NULL)
    )
);

CREATE INDEX idx_execution_events_fold ON execution_events (exec_id, seq);

INSERT INTO executions (
    exec_id, flow_id, namespace_id, triggered_by, trigger_type, inputs, scheduled_at,
    created_at, attempt, status, error, outputs, started_at, completed_at, updated_at
)
SELECT DISTINCT ON (el.exec_id)
    el.exec_id, el.flow_id, el.namespace_id, el.triggered_by, el.trigger_type,
    COALESCE(el.context -> 'inputs', '{}'::jsonb), el.scheduled_at, el.created_at,
    CASE WHEN el.started_at IS NULL THEN 0 ELSE el.version + 1 END,
    el.status, el.error, COALESCE(el.context -> 'outputs', '{}'::jsonb),
    el.started_at, el.completed_at, el.updated_at
FROM execution_log el
ORDER BY el.exec_id, el.version DESC;

INSERT INTO execution_events (exec_id, attempt, type, outputs, created_at)
SELECT exec_id, attempt, 'queued',
       outputs,
       created_at
FROM executions;

INSERT INTO execution_events (exec_id, attempt, type, outputs, created_at)
SELECT exec_id, attempt, 'started',
       CASE WHEN status = 'running' THEN outputs ELSE NULL END,
       started_at
FROM executions
WHERE started_at IS NOT NULL;

INSERT INTO execution_events (exec_id, attempt, type, error, outputs, created_at)
SELECT exec_id, attempt,
       CASE status
           WHEN 'pending_approval' THEN 'waiting_approval'::execution_event_type
           WHEN 'completed' THEN 'completed'::execution_event_type
           WHEN 'errored' THEN 'errored'::execution_event_type
           WHEN 'cancelled' THEN 'cancelled'::execution_event_type
       END,
       error, outputs, COALESCE(completed_at, updated_at)
FROM executions
WHERE status IN ('pending_approval', 'completed', 'errored', 'cancelled');

WITH latest AS (
    SELECT DISTINCT ON (el.exec_id) el.*
    FROM execution_log el
    ORDER BY el.exec_id, el.version DESC
), states AS (
    SELECT l.exec_id, e.attempt, entry.key AS action_id, entry.value AS state,
           COALESCE(NULLIF(entry.value ->> 'started_at', '')::timestamptz, e.started_at, e.created_at) AS started_at,
           COALESCE(NULLIF(entry.value ->> 'finished_at', '')::timestamptz, e.completed_at, e.updated_at) AS finished_at
    FROM latest l
    JOIN executions e ON e.exec_id = l.exec_id
    CROSS JOIN LATERAL jsonb_each(COALESCE(l.action_states, '{}'::jsonb)) entry
    WHERE entry.value ->> 'status' NOT IN ('pending', 'blocked')
)
INSERT INTO execution_events (exec_id, attempt, action_id, type, created_at)
SELECT exec_id, attempt, action_id, 'action_started', started_at
FROM states;

WITH latest AS (
    SELECT DISTINCT ON (el.exec_id) el.*
    FROM execution_log el
    ORDER BY el.exec_id, el.version DESC
), states AS (
    SELECT l.exec_id, e.attempt, entry.key AS action_id, entry.value AS state,
           COALESCE(NULLIF(entry.value ->> 'finished_at', '')::timestamptz, e.completed_at, e.updated_at) AS finished_at
    FROM latest l
    JOIN executions e ON e.exec_id = l.exec_id
    CROSS JOIN LATERAL jsonb_each(COALESCE(l.action_states, '{}'::jsonb)) entry
    WHERE entry.value ->> 'status' NOT IN ('pending', 'running')
)
INSERT INTO execution_events (exec_id, attempt, action_id, type, error, created_at)
SELECT exec_id, attempt, action_id,
       CASE state ->> 'status'
           WHEN 'completed' THEN 'action_completed'::execution_event_type
           WHEN 'failed' THEN 'action_failed'::execution_event_type
           WHEN 'blocked' THEN 'action_blocked'::execution_event_type
           WHEN 'skipped' THEN 'action_skipped'::execution_event_type
           WHEN 'cancelled' THEN 'action_cancelled'::execution_event_type
       END,
       NULLIF(state ->> 'error', ''), finished_at
FROM states
WHERE state ->> 'status' IN ('completed', 'failed', 'blocked', 'skipped', 'cancelled');

ALTER TABLE approvals ADD COLUMN exec_id VARCHAR(36);
UPDATE approvals a SET exec_id = el.exec_id FROM execution_log el WHERE a.exec_log_id = el.id;
DELETE FROM approvals a USING approvals b
WHERE a.exec_id = b.exec_id AND a.action_id = b.action_id
  AND (
    (a.status = 'pending' AND b.status <> 'pending')
    OR (((a.status = 'pending') = (b.status = 'pending')) AND a.id < b.id)
  );
ALTER TABLE approvals DROP CONSTRAINT approvals_exec_log_id_fkey;
DROP INDEX idx_approvals_exec_action_id;
ALTER TABLE approvals DROP COLUMN exec_log_id;
ALTER TABLE approvals ALTER COLUMN exec_id SET NOT NULL;
ALTER TABLE approvals ADD CONSTRAINT approvals_exec_id_fkey
    FOREIGN KEY (exec_id) REFERENCES executions(exec_id) ON DELETE CASCADE;
CREATE UNIQUE INDEX idx_approvals_exec_action ON approvals (exec_id, action_id);

DROP TABLE execution_log;
