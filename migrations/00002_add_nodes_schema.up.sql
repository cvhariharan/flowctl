CREATE TYPE authentication_method AS ENUM (
    'ssh_key',
    'password'
);

CREATE TABLE IF NOT EXISTS credentials (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    name VARCHAR(150) NOT NULL,
    private_key TEXT,
    password TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_credentials_uuid ON credentials(uuid);
CREATE UNIQUE INDEX idx_credentials_name ON credentials(name);

CREATE TABLE IF NOT EXISTS nodes (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    name VARCHAR(150) NOT NULL,
    hostname VARCHAR(255) NOT NULL,
    port INTEGER NOT NULL DEFAULT 22,
    username VARCHAR(150) NOT NULL,
    os_family VARCHAR(50) NOT NULL,
    tags TEXT[],
    auth_method authentication_method NOT NULL DEFAULT 'ssh_key',
    credential_id INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    FOREIGN KEY (credential_id) REFERENCES credentials(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX idx_nodes_uuid ON nodes(uuid);
CREATE UNIQUE INDEX idx_nodes_name ON nodes(name);