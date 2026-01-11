-- name: CreateNamespaceWebhook :one
INSERT INTO namespace_webhooks (
    name,
    description,
    type,
    encrypted_url,
    encrypted_headers,
    content_type,
    template_body,
    template_format,
    is_active,
    namespace_id
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    (SELECT id FROM namespaces WHERE namespaces.uuid = $10)
)
RETURNING *;

-- name: GetNamespaceWebhookByUUID :one
SELECT w.*, ns.uuid AS namespace_uuid
FROM namespace_webhooks w
JOIN namespaces ns ON w.namespace_id = ns.id
WHERE w.uuid = $1 AND ns.uuid = $2;

-- name: GetNamespaceWebhookByID :one
SELECT w.*, ns.uuid AS namespace_uuid
FROM namespace_webhooks w
JOIN namespaces ns ON w.namespace_id = ns.id
WHERE w.id = $1;

-- name: GetNamespaceWebhookByName :one
SELECT w.*, ns.uuid AS namespace_uuid
FROM namespace_webhooks w
JOIN namespaces ns ON w.namespace_id = ns.id
WHERE ns.uuid = $1 AND w.name = $2;

-- name: ListNamespaceWebhooks :many
SELECT w.*, ns.uuid AS namespace_uuid
FROM namespace_webhooks w
JOIN namespaces ns ON w.namespace_id = ns.id
WHERE ns.uuid = $1
ORDER BY w.updated_at DESC;

-- name: UpdateNamespaceWebhook :one
UPDATE namespace_webhooks
SET
    name = $2,
    description = $3,
    type = $4,
    encrypted_url = $5,
    encrypted_headers = $6,
    content_type = $7,
    template_body = $8,
    template_format = $9,
    is_active = $10,
    updated_at = NOW()
WHERE namespace_webhooks.uuid = $1
  AND namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $11)
RETURNING *;

-- name: DeleteNamespaceWebhook :exec
DELETE FROM namespace_webhooks
WHERE namespace_webhooks.uuid = $1
  AND namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $2);

-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (
    webhook_id,
    flow_id,
    execution_id,
    event
)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetWebhookDeliveryByUUID :one
SELECT * FROM webhook_deliveries
WHERE uuid = $1;

-- name: UpdateWebhookDelivery :one
UPDATE webhook_deliveries
SET
    status = $2,
    attempt_count = $3,
    next_attempt_at = $4,
    last_status_code = $5,
    last_error_message = $6,
    delivered_at = $7,
    updated_at = NOW()
WHERE uuid = $1
RETURNING *;

-- name: AddWebhookDeliveryAttempt :one
INSERT INTO webhook_delivery_attempts (
    delivery_id,
    attempt_number,
    status_code,
    error_message,
    duration_ms
)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;
