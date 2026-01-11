CREATE TABLE IF NOT EXISTS namespace_webhooks (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    name VARCHAR(150) NOT NULL,
    description TEXT,
    type VARCHAR(32) NOT NULL DEFAULT 'generic',
    encrypted_url TEXT NOT NULL,
    encrypted_headers TEXT,
    content_type VARCHAR(255) NOT NULL DEFAULT 'application/json',
    template_body TEXT NOT NULL,
    template_format VARCHAR(32) NOT NULL DEFAULT 'json',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    namespace_id INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    FOREIGN KEY (namespace_id) REFERENCES namespaces(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX idx_namespace_webhooks_uuid ON namespace_webhooks(uuid);
CREATE UNIQUE INDEX idx_namespace_webhooks_name_namespace ON namespace_webhooks(namespace_id, name);
CREATE INDEX idx_namespace_webhooks_namespace ON namespace_webhooks(namespace_id);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id SERIAL PRIMARY KEY,
    uuid UUID NOT NULL DEFAULT uuid_generate_v4(),
    webhook_id INTEGER NOT NULL,
    flow_id VARCHAR(150) NOT NULL,
    execution_id VARCHAR(36) NOT NULL,
    event VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP WITH TIME ZONE,
    last_status_code INTEGER,
    last_error_message TEXT,
    delivered_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    FOREIGN KEY (webhook_id) REFERENCES namespace_webhooks(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX idx_webhook_deliveries_uuid ON webhook_deliveries(uuid);
CREATE INDEX idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
CREATE INDEX idx_webhook_deliveries_next_attempt_at ON webhook_deliveries(next_attempt_at);

CREATE TABLE IF NOT EXISTS webhook_delivery_attempts (
    id SERIAL PRIMARY KEY,
    delivery_id UUID NOT NULL,
    attempt_number INTEGER NOT NULL,
    status_code INTEGER,
    error_message TEXT,
    duration_ms INTEGER,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    FOREIGN KEY (delivery_id) REFERENCES webhook_deliveries(uuid) ON DELETE CASCADE
);
CREATE INDEX idx_webhook_delivery_attempts_delivery_id ON webhook_delivery_attempts(delivery_id);
