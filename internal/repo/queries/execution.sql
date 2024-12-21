-- name: AddExecutionLog :one
WITH user_lookup AS (
    SELECT id FROM users WHERE uuid = $4
)
INSERT INTO execution_log (
    exec_id,
    flow_id,
    input,
    triggered_by
) VALUES (
    $1, $2, $3, (SELECT id FROM user_lookup)
) RETURNING *;

-- name: UpdateExecutionStatus :one
UPDATE execution_log SET status=$1, error=$2, updated_at=$3 WHERE exec_id = $4 RETURNING *;

-- name: GetExecutionsByFlow :many
WITH user_lookup AS (
    SELECT id FROM users WHERE uuid = $2
)
SELECT * FROM execution_log WHERE flow_id = $1 and triggered_by = (SELECT id FROM user_lookup);

-- name: GetExecutionByExecID :one
SELECT
    el.*,
    u.uuid AS triggered_by_uuid
FROM
    execution_log el
INNER JOIN
    users u ON el.triggered_by = u.id
WHERE
    el.exec_id = $1;

-- name: GetFlowFromExecID :one
WITH exec_log AS (
    SELECT flow_id FROM execution_log WHERE exec_id = $1
)
SELECT * FROM flows inner join exec_log on exec_log.flow_id = flows.id;
