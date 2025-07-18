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
SELECT el.* FROM execution_log el
INNER JOIN flows f ON el.flow_id = f.id
WHERE f.id = $1 
  AND el.triggered_by = (SELECT id FROM user_lookup)
  AND f.namespace_id = (SELECT id FROM namespace_lookup);

-- name: GetExecutionByExecID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT
    el.*,
    u.uuid AS triggered_by_uuid
FROM
    execution_log el
INNER JOIN
    users u ON el.triggered_by = u.id
WHERE
    el.exec_id = $1
    AND el.namespace_id = (SELECT id FROM namespace_lookup);

-- name: GetExecutionByExecIDWithNamespace :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT
    el.*,
    u.uuid AS triggered_by_uuid
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
SELECT * FROM execution_log 
WHERE execution_log.id = $1 AND execution_log.namespace_id = (SELECT id FROM namespace_lookup);

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
SELECT * FROM execution_log 
WHERE execution_log.parent_exec_id = $1 AND execution_log.namespace_id = (SELECT id FROM namespace_lookup);
