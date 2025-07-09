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
    name VARCHAR(150) NOT NULL,
    username VARCHAR(255) NOT NULL,
    password VARCHAR(255),
    login_type user_login_type NOT NULL DEFAULT 'standard',
    role user_role_type NOT NULL DEFAULT 'user',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_users_uuid ON users(uuid);
CREATE UNIQUE INDEX idx_users_username ON users(username);

CREATE TABLE IF NOT EXISTS groups (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_groups_uuid ON groups(uuid);
CREATE UNIQUE INDEX idx_groups_name ON groups(name);

CREATE TABLE IF NOT EXISTS group_memberships (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
    UNIQUE(user_id, group_id)
);
CREATE INDEX idx_group_memberships_user_id ON group_memberships(user_id);
CREATE INDEX idx_group_memberships_group_id ON group_memberships(group_id);

CREATE OR REPLACE VIEW group_view AS
SELECT
    g.*,
    CASE
        WHEN COUNT(u.id) > 0 THEN JSON_AGG(u.*)
        ELSE NULL
    END AS users
FROM
    groups g
LEFT JOIN
    group_memberships gm ON g.id = gm.group_id
LEFT JOIN
    users u ON gm.user_id = u.id
GROUP BY
    g.id, g.uuid, g.name, g.description, g.created_at, g.updated_at;


CREATE OR REPLACE VIEW user_view AS
SELECT
    u.*,
    CASE
        WHEN COUNT(g.id) > 0 THEN JSON_AGG(g.*)
        ELSE NULL
    END AS groups
FROM
    users u
LEFT JOIN
    group_memberships gm ON u.id = gm.user_id
LEFT JOIN
    groups g ON gm.group_id = g.id
GROUP BY
    u.id, u.uuid, u.name, u.username, u.password, u.login_type, u.role, u.created_at, u.updated_at;

CREATE TYPE execution_status AS ENUM (
    'completed',
    'errored',
    'pending',
    'pending_approval',
    'running'
);

CREATE TABLE IF NOT EXISTS execution_log (
    id SERIAL PRIMARY KEY,
    exec_id VARCHAR(36) NOT NULL,
    flow_id INTEGER NOT NULL,
    parent_exec_id VARCHAR(36),
    input JSONB DEFAULT '{}'::jsonb NOT NULL,
    error TEXT,
    status execution_status NOT NULL DEFAULT 'pending',
    triggered_by INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    FOREIGN KEY (flow_id) REFERENCES flows(id) ON DELETE CASCADE,
    FOREIGN KEY (triggered_by) REFERENCES users(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX idx_execution_log_exec_id ON execution_log(exec_id);
CREATE INDEX idx_execution_log_triggered_by ON execution_log(triggered_by);

CREATE TYPE approval_status AS ENUM (
    'pending',
    'approved',
    'rejected'
);

CREATE TABLE IF NOT EXISTS approvals (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    exec_log_id INTEGER NOT NULL,
    action_id VARCHAR(50) NOT NULL,
    status approval_status NOT NULL DEFAULT 'pending',
    approvers JSONB DEFAULT '{}'::jsonb NOT NULL,
    decided_by INTEGER,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    FOREIGN KEY (exec_log_id) REFERENCES execution_log(id) ON DELETE CASCADE,
    FOREIGN KEY (decided_by) REFERENCES users(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX idx_approvals_uuid ON approvals(uuid);
CREATE UNIQUE INDEX idx_approvals_exec_action_id ON approvals(exec_log_id, action_id);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT NOT NULL PRIMARY KEY,
    data JSONB DEFAULT '{}'::jsonb NOT NULL,
    created_at TIMESTAMP DEFAULT now() NOT NULL
);
CREATE INDEX idx_sessions ON sessions (id, created_at);
