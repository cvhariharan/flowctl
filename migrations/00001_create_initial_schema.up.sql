CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS flows (
    id SERIAL PRIMARY KEY,
    slug VARCHAR(100) NOT NULL,
    name VARCHAR(150) NOT NULL,
    checksum VARCHAR(128) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_flows_slug ON flows(slug);

CREATE TYPE execution_status AS ENUM (
    'pending',
    'running',
    'successful',
    'failed'
);

CREATE TABLE IF NOT EXISTS execution_queue (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    flow_id INTEGER NOT NULL,
    input JSONB DEFAULT '{}'::jsonb NOT NULL,
    status execution_status NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    FOREIGN KEY (flow_id) REFERENCES flows(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX idx_execution_queue_uuid ON execution_queue(uuid);
CREATE INDEX idx_execution_queue_created_at ON execution_queue(created_at);

CREATE TABLE IF NOT EXISTS results (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    flow_id INTEGER NOT NULL,
    execution_id INTEGER NOT NULL,
    output JSONB DEFAULT '{}'::jsonb NOT NULL,
    error JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    FOREIGN KEY (flow_id) REFERENCES flows(id) ON DELETE CASCADE,
    FOREIGN KEY (execution_id) REFERENCES execution_queue(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX idx_results_uuid ON results(uuid);

CREATE TYPE user_login_type AS ENUM (
    'oidc',
    'standard',
    'token'
);

CREATE TYPE user_role_type AS ENUM (
    'admin',
    'user'
);

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    username VARCHAR(255) NOT NULL,
    password VARCHAR(255),
    login_type user_login_type NOT NULL DEFAULT 'standard',
    role user_role_type NOT NULL DEFAULT 'user',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_users_uuid ON users(uuid);

CREATE OR REPLACE FUNCTION notify_new_execution_entry()
RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('new_flow', NEW.id::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER new_execution_trigger
    AFTER INSERT
    ON execution_queue
    FOR EACH ROW
    EXECUTE FUNCTION notify_new_execution_entry();


CREATE TABLE IF NOT EXISTS sessions (
    id TEXT NOT NULL PRIMARY KEY,
    data JSONB DEFAULT '{}'::jsonb NOT NULL,
    created_at TIMESTAMP DEFAULT now() NOT NULL
);
CREATE INDEX idx_sessions ON sessions (id, created_at);