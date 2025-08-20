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
    cron_schedule,
    namespace_id
) VALUES (
    $1, $2, $3, $4, $5, (SELECT id FROM namespaces WHERE namespaces.name = $6)
) RETURNING *;

-- name: UpdateFlow :one
UPDATE flows SET 
    name = $1,
    description = $2,
    checksum = $3,
    cron_schedule = $4,
    updated_at = NOW()
WHERE slug = $5 AND namespace_id = (SELECT id FROM namespaces WHERE namespaces.name = $6)
RETURNING *;

-- name: DeleteFlow :exec
DELETE FROM flows WHERE slug = $1 AND namespace_id = (SELECT id FROM namespaces where namespaces.uuid = $2);

-- name: GetFlowsByNamespace :many
SELECT f.*, n.uuid AS namespace_uuid
FROM flows f
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
    SELECT CEIL(total.total_count::numeric / $2::numeric)::bigint AS page_count FROM total
)
SELECT 
    p.*,
    pc.page_count,
    t.total_count
FROM paged p, page_count pc, total t;

-- name: ListFlowsPaginated :many
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
    SELECT CEIL(total.total_count::numeric / $2::numeric)::bigint AS page_count FROM total
)
SELECT 
    p.*,
    pc.page_count,
    t.total_count
FROM paged p, page_count pc, total t;

-- name: SearchFlowsPaginated :many
WITH filtered AS (
    SELECT f.*, n.uuid AS namespace_uuid FROM flows f
    JOIN namespaces n ON f.namespace_id = n.id
    WHERE n.uuid = $1
      AND (lower(f.name) LIKE '%' || lower($2::text) || '%'
           OR lower(f.description) LIKE '%' || lower($2::text) || '%')
),
total AS (
    SELECT COUNT(*) AS total_count FROM filtered
),
paged AS (
    SELECT * FROM filtered
    ORDER BY created_at DESC
    LIMIT $3 OFFSET $4
),
page_count AS (
    SELECT CEIL(total.total_count::numeric / $3::numeric)::bigint AS page_count FROM total
)
SELECT 
    p.*,
    pc.page_count,
    t.total_count
FROM paged p, page_count pc, total t;

-- name: GetScheduledFlows :many
SELECT f.*, n.uuid AS namespace_uuid
FROM flows f
JOIN namespaces n ON f.namespace_id = n.id
WHERE f.cron_schedule IS NOT NULL AND f.cron_schedule != '';