-- name: AddExecutionLog :one
WITH user_lookup AS (
    SELECT id FROM users WHERE users.uuid = $5
), namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $6
)
INSERT INTO execution_log (
    exec_id,
    parent_exec_id,
    flow_id,
    input,
    triggered_by,
    namespace_id
) VALUES (
    $1, $2, $3, $4, (SELECT id FROM user_lookup), (SELECT id FROM namespace_lookup)
) RETURNING *;

-- name: UpdateExecutionStatus :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $5
)
UPDATE execution_log SET status=$1, error=$2, updated_at=$3
WHERE exec_id = $4 AND namespace_id = (SELECT id FROM namespace_lookup)
RETURNING *;

-- name: GetExecutionsByFlow :many
WITH user_lookup AS (
    SELECT id FROM users WHERE users.uuid = $2
), namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $3
)
SELECT el.*, u.name, u.username, u.uuid as triggered_by_uuid,
       CONCAT(u.name, ' <', u.username, '>')::TEXT as triggered_by_name,
       f.name as flow_name
FROM execution_log el
INNER JOIN flows f ON el.flow_id = f.id
INNER JOIN users u ON el.triggered_by = u.id
WHERE f.id = $1
  AND el.triggered_by = (SELECT id FROM user_lookup)
  AND f.namespace_id = (SELECT id FROM namespace_lookup);

-- name: GetExecutionByExecID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT
    el.*,
    u.name,
    u.username,
    u.uuid AS triggered_by_uuid,
    CONCAT(u.name, ' <', u.username, '>')::TEXT as triggered_by_name,
    f.name as flow_name
FROM
    execution_log el
INNER JOIN
    users u ON el.triggered_by = u.id
INNER JOIN
    flows f ON el.flow_id = f.id
WHERE
    el.exec_id = $1
    AND el.namespace_id = (SELECT id FROM namespace_lookup);

-- name: GetExecutionByExecIDWithNamespace :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT
    el.*,
    u.name,
    u.username,
    u.uuid AS triggered_by_uuid,
    CONCAT(u.name, ' <', u.username, '>')::TEXT as triggered_by_name,
    f.name as flow_name
FROM
    execution_log el
INNER JOIN
    users u ON el.triggered_by = u.id
INNER JOIN
    flows f ON el.flow_id = f.id
WHERE
    el.exec_id = $1
    AND f.namespace_id = (SELECT id FROM namespace_lookup);

-- name: GetFlowFromExecID :one
WITH exec_log AS (
    SELECT flow_id FROM execution_log WHERE exec_id = $1
), namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT f.* FROM flows f
INNER JOIN exec_log el ON el.flow_id = f.id
WHERE f.namespace_id = (SELECT id FROM namespace_lookup);

-- name: GetFlowFromExecIDWithNamespace :one
WITH exec_log AS (
    SELECT flow_id FROM execution_log WHERE exec_id = $1
), namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT f.* FROM flows f
INNER JOIN exec_log el ON el.flow_id = f.id
WHERE f.namespace_id = (SELECT id FROM namespace_lookup);

-- name: GetExecutionByID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT el.*, u.name, u.username, u.uuid as triggered_by_uuid,
       CONCAT(u.name, ' <', u.username, '>')::TEXT as triggered_by_name,
       f.name as flow_name
FROM execution_log el
INNER JOIN users u ON el.triggered_by = u.id
INNER JOIN flows f ON el.flow_id = f.id
WHERE el.id = $1 AND el.namespace_id = (SELECT id FROM namespace_lookup);

-- name: GetInputForExecByUUID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT input FROM execution_log
WHERE execution_log.exec_id = $1 AND execution_log.namespace_id = (SELECT id FROM namespace_lookup);

-- name: GetChildrenByParentUUID :many
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT el.*, u.name, u.username, u.uuid as triggered_by_uuid,
       CONCAT(u.name, ' <', u.username, '>')::TEXT as triggered_by_name,
       f.name as flow_name
FROM execution_log el
INNER JOIN users u ON el.triggered_by = u.id
INNER JOIN flows f ON el.flow_id = f.id
WHERE el.parent_exec_id = $1 AND el.namespace_id = (SELECT id FROM namespace_lookup);

-- name: GetExecutionsByFlowPaginated :many
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
),
filtered AS (
    SELECT el.*, u.name, u.username, u.uuid as triggered_by_uuid,
           CONCAT(u.name, ' <', u.username, '>')::TEXT as triggered_by_name,
           f.name as flow_name
    FROM execution_log el
    INNER JOIN flows f ON el.flow_id = f.id
    INNER JOIN users u ON el.triggered_by = u.id
    WHERE f.id = $1
      AND f.namespace_id = (SELECT id FROM namespace_lookup)
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

-- name: GetAllExecutionsPaginated :many
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $1
),
filtered AS (
    SELECT el.*, u.name, u.username, u.uuid as triggered_by_uuid,
           CONCAT(u.name, ' <', u.username, '>')::TEXT as triggered_by_name,
           f.name as flow_name
    FROM execution_log el
    INNER JOIN flows f ON el.flow_id = f.id
    INNER JOIN users u ON el.triggered_by = u.id
    WHERE f.namespace_id = (SELECT id FROM namespace_lookup)
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
