CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS flows (
    id SERIAL PRIMARY KEY,
    slug VARCHAR(100) NOT NULL,
    name VARCHAR(150) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
)
CREATE UNIQUE INDEX idx_flows_slug ON flows(slug);

CREATE TYPE execution_status AS ENUM (
    'pending',
    'running',
    'completed',
    'error'
);

CREATE TABLE IF NOT EXISTS execution_queue (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    flow_id INTEGER,
    input JSONB DEFAULT '{}'::jsonb NOT NULL,
    status execution_status NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (flow_id) REFERENCES flows(id)
)
CREATE UNIQUE INDEX idx_execution_queue_uuid ON execution_queue(uuid);

CREATE TABLE IF NOT EXISTS results (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    flow_id INTEGER,
    execution_id INTEGER,
    output JSONB DEFAULT '{}'::jsonb NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (flow_id) REFERENCES flows(id)
    FOREIGN KEY (execution_id) REFERENCES execution_queue(id)
)
CREATE UNIQUE INDEX idx_results_uuid ON results(uuid);


CREATE TYPE user_login_type AS ENUM (
    'oidc',
    'standard',
    'token'
);

CREATE TYPE user_role_type AS ENUM (
    'admin',
    'user',
);

CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    username VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    login_type user_login_type NOT NULL DEFAULT 'standard',
    role user_role_type NOT NULL DEFAULT 'user',
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_users_uuid ON users (uuid);