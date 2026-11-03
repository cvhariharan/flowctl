CREATE TABLE flow_versions (
    id SERIAL PRIMARY KEY,
    flow_id INTEGER NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    checksum VARCHAR(128) NOT NULL,
    format VARCHAR(10) NOT NULL,
    source TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (flow_id, version),
    UNIQUE (flow_id, checksum)
);

ALTER TABLE executions ADD COLUMN flow_version_id INTEGER REFERENCES flow_versions(id);
CREATE INDEX idx_executions_flow_version ON executions (flow_version_id);
