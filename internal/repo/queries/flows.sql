-- name: GetFlowBySlug :one
SELECT f.* FROM flows f
JOIN namespaces n ON f.namespace_id = n.id
WHERE f.slug = $1 AND n.uuid = $2;

-- name: DeleteAllFlows :exec
DELETE FROM flows;

-- name: CreateFlow :one
INSERT INTO flows (
    slug,
    name,
    description,
    checksum,
    namespace_id
) VALUES (
    $1, $2, $3, $4, (SELECT id FROM namespaces WHERE namespaces.name = $5)
) RETURNING *;

-- name: UpdateFlow :one
UPDATE flows SET 
    name = $1,
    description = $2,
    checksum = $3,
    updated_at = NOW()
WHERE slug = $4 AND namespace_id = (SELECT id FROM namespaces WHERE namespaces.name = $5)
RETURNING *;

-- name: GetFlowsByNamespace :many
SELECT f.*, n.uuid AS namespace_uuid FROM flows f
JOIN namespaces n ON f.namespace_id = n.id
WHERE n.uuid = $1;

-- name: ListFlows :many
WITH filtered AS (
    SELECT f.*, n.uuid AS namespace_uuid FROM flows f
    JOIN namespaces n ON f.namespace_id = n.id
    WHERE n.uuid = $1
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
    SELECT COUNT(*) AS page_count FROM paged
)
SELECT 
    p.*,
    pc.page_count,
    t.total_count
FROM paged p, page_count pc, total t;