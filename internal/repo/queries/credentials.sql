-- name: CreateCredential :one
INSERT INTO credentials (name, key_type, key_data, namespace_id)
VALUES ($1, $2, $3, (SELECT id FROM namespaces WHERE namespaces.uuid = $4))
RETURNING *;

-- name: GetCredentialByUUID :one
SELECT c.*, ns.uuid AS namespace_uuid FROM credentials c
JOIN namespaces ns ON c.namespace_id = ns.id
WHERE c.uuid = $1 AND ns.uuid = $2;

-- name: GetCredentialByID :one
SELECT c.*, ns.uuid AS namespace_uuid FROM credentials c
JOIN namespaces ns ON c.namespace_id = ns.id
WHERE c.id = $1 AND ns.uuid = $2;

-- name: ListCredentials :many
WITH filtered AS (
    SELECT c.*, ns.uuid AS namespace_uuid FROM credentials c
    JOIN namespaces ns ON c.namespace_id = ns.id
    WHERE ns.uuid = $1
),
total AS (
    SELECT COUNT(*) AS total_count FROM filtered
),
paged AS (
    SELECT * FROM filtered
    ORDER BY created_at DESC
    LIMIT $2 OFFSET $3
),
page_count AS (
    SELECT CEIL(total.total_count::numeric / $2::numeric)::bigint AS page_count FROM total
)
SELECT
    p.*,
    pc.page_count,
    t.total_count
FROM paged p, page_count pc, total t;

-- name: UpdateCredential :one
UPDATE credentials
SET name = $2, key_type = $3, key_data = $4, updated_at = NOW()
WHERE credentials.uuid = $1 AND namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $5)
RETURNING *;

-- name: DeleteCredential :exec
DELETE FROM credentials WHERE credentials.uuid = $1 AND namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $2);
