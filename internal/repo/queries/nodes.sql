-- name: CreateNode :one
INSERT INTO nodes (name, hostname, port, username, os_family, tags, auth_method, credential_id, namespace_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, (SELECT id FROM namespaces WHERE namespaces.uuid = $9))
RETURNING *;

-- name: GetNodeByUUID :one
SELECT n.*, ns.uuid AS namespace_uuid FROM nodes n
JOIN namespaces ns ON n.namespace_id = ns.id
WHERE n.uuid = $1 AND ns.uuid = $2;

-- name: ListNodes :many
WITH filtered AS (
    SELECT n.*, ns.uuid AS namespace_uuid FROM nodes n
    JOIN namespaces ns ON n.namespace_id = ns.id
    WHERE ns.uuid = $1
),
total AS (
    SELECT COUNT(*) AS total_count FROM filtered
),
paged AS (
    SELECT * FROM filtered
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

-- name: UpdateNode :one
UPDATE nodes
SET name = $2, hostname = $3, port = $4, username = $5, os_family = $6, tags = $7, auth_method = $8, credential_id = $9, updated_at = NOW()
WHERE nodes.uuid = $1 AND namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $10)
RETURNING *;

-- name: DeleteNode :exec
DELETE FROM nodes WHERE nodes.uuid = $1 AND namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $2);

-- name: GetNodeByName :one
SELECT n.*, ns.uuid AS namespace_uuid FROM nodes n
JOIN namespaces ns ON n.namespace_id = ns.id
WHERE n.name = $1 AND ns.uuid = $2;

-- name: GetNodesByNames :many
SELECT 
    n.*,
    ns.uuid AS namespace_uuid,
    c.uuid AS credential_uuid, 
    c.name AS credential_name, 
    c.key_type AS credential_key_type, 
    c.key_data AS credential_key_data
FROM nodes n
JOIN namespaces ns ON n.namespace_id = ns.id
LEFT JOIN credentials c ON n.credential_id = c.id
WHERE n.name = ANY($1::text[]) AND ns.uuid = $2
ORDER BY n.name;
