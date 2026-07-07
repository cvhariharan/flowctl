-- name: AddExecutionLog :one
WITH user_lookup AS (
    SELECT id FROM users WHERE users.uuid = $4
), namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $5
), next_version AS (
    SELECT COALESCE(MAX(version), -1) + 1 as version
    FROM execution_log
    WHERE exec_id = $1 AND namespace_id = (SELECT id FROM namespace_lookup)
), prev_action_retries AS (
    SELECT action_retries
    FROM execution_log
    WHERE exec_id = $1 AND namespace_id = (SELECT id FROM namespace_lookup)
    ORDER BY version DESC
    LIMIT 1
)
INSERT INTO execution_log (
    exec_id,
    flow_id,
    version,
    input,
    trigger_type,
    triggered_by,
    namespace_id,
    action_retries,
    scheduled_at
) VALUES (
    $1, $2, (SELECT version FROM next_version), $3, $6, (SELECT id FROM user_lookup), (SELECT id FROM namespace_lookup),
    COALESCE((SELECT action_retries FROM prev_action_retries), '{}'),
    $7
) RETURNING *;

-- name: UpdateExecutionStatus :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $4
), latest_version AS (
    SELECT MAX(version) as version
    FROM execution_log
    WHERE execution_log.exec_id = $3 AND namespace_id = (SELECT id FROM namespace_lookup)
)
UPDATE execution_log SET
    status=$1,
    error=$2,
    updated_at=NOW(),
    completed_at = CASE WHEN $1 IN ('completed'::execution_status, 'errored'::execution_status, 'cancelled'::execution_status) THEN NOW() ELSE completed_at END
WHERE execution_log.exec_id = $3
  AND version = (SELECT version FROM latest_version)
  AND namespace_id = (SELECT id FROM namespace_lookup)
RETURNING *;

-- name: UpdateExecutionActionID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $3
), latest_version AS (
    SELECT MAX(version) as version
    FROM execution_log
    WHERE execution_log.exec_id = $2 AND namespace_id = (SELECT id FROM namespace_lookup)
)
UPDATE execution_log SET current_action_id=$1, updated_at=NOW()
WHERE execution_log.exec_id = $2
  AND version = (SELECT version FROM latest_version)
  AND namespace_id = (SELECT id FROM namespace_lookup)
RETURNING *;

-- name: GetExecutionsByFlow :many
WITH user_lookup AS (
    SELECT id FROM users WHERE users.uuid = $2
), namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $3
)
SELECT el.*, u.name, u.username, u.uuid as triggered_by_uuid,
       CONCAT(u.name, ' <', u.username, '>')::TEXT as triggered_by_name,
       f.name as flow_name,
       f.slug as flow_slug
FROM execution_log el
INNER JOIN flows f ON el.flow_id = f.id
INNER JOIN users u ON el.triggered_by = u.id
WHERE f.id = $1
  AND el.triggered_by = (SELECT id FROM user_lookup)
  AND f.namespace_id = (SELECT id FROM namespace_lookup)
  AND f.is_active = TRUE;

-- name: GetExecutionByExecID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
), latest_version AS (
    SELECT MAX(version) as version
    FROM execution_log
    WHERE exec_id = $1 AND namespace_id = (SELECT id FROM namespace_lookup)
)
SELECT
    el.*,
    u.name,
    u.username,
    u.uuid AS triggered_by_uuid,
    CONCAT(u.name, ' <', u.username, '>')::TEXT as triggered_by_name,
    f.name as flow_name,
    f.slug as flow_slug
FROM
    execution_log el
INNER JOIN
    users u ON el.triggered_by = u.id
INNER JOIN
    flows f ON el.flow_id = f.id
WHERE
    el.exec_id = $1
    AND el.version = (SELECT version FROM latest_version)
    AND el.namespace_id = (SELECT id FROM namespace_lookup)
    AND f.is_active = TRUE;

-- name: GetExecutionByExecIDWithNamespace :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
), latest_version AS (
    SELECT MAX(version) as version
    FROM execution_log el2
    INNER JOIN flows f2 ON el2.flow_id = f2.id
    WHERE el2.exec_id = $1 AND f2.namespace_id = (SELECT id FROM namespace_lookup) AND f2.is_active = TRUE
)
SELECT
    el.*,
    u.name,
    u.username,
    u.uuid AS triggered_by_uuid,
    CONCAT(u.name, ' <', u.username, '>')::TEXT as triggered_by_name,
    f.name as flow_name,
    f.slug as flow_slug
FROM
    execution_log el
INNER JOIN
    users u ON el.triggered_by = u.id
INNER JOIN
    flows f ON el.flow_id = f.id
WHERE
    el.exec_id = $1
    AND el.version = (SELECT version FROM latest_version)
    AND f.namespace_id = (SELECT id FROM namespace_lookup)
    AND f.is_active = TRUE;

-- name: GetFlowFromExecID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
), latest_exec_log AS (
    SELECT flow_id
    FROM execution_log
    WHERE exec_id = $1
    ORDER BY version DESC
    LIMIT 1
)
SELECT f.* FROM flows f
INNER JOIN latest_exec_log el ON el.flow_id = f.id
WHERE f.namespace_id = (SELECT id FROM namespace_lookup) AND f.is_active = TRUE;

-- name: GetFlowFromExecIDWithNamespace :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
), latest_exec_log AS (
    SELECT flow_id
    FROM execution_log el
    INNER JOIN flows f ON el.flow_id = f.id
    WHERE el.exec_id = $1
      AND f.namespace_id = (SELECT id FROM namespace_lookup)
      AND f.is_active = TRUE
    ORDER BY el.version DESC
    LIMIT 1
)
SELECT f.* FROM flows f
INNER JOIN latest_exec_log el ON el.flow_id = f.id
WHERE f.namespace_id = (SELECT id FROM namespace_lookup) AND f.is_active = TRUE;

-- name: GetExecutionByID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT el.*, u.name, u.username, u.uuid as triggered_by_uuid,
       CONCAT(u.name, ' <', u.username, '>')::TEXT as triggered_by_name,
       f.name as flow_name,
       f.slug as flow_slug
FROM execution_log el
INNER JOIN users u ON el.triggered_by = u.id
INNER JOIN flows f ON el.flow_id = f.id
WHERE el.id = $1 AND el.namespace_id = (SELECT id FROM namespace_lookup) AND f.is_active = TRUE;

-- name: GetInputForExecByUUID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
), latest_execution AS (
    SELECT MAX(version) as max_version
    FROM execution_log
    WHERE exec_id = $1 AND namespace_id = (SELECT id FROM namespace_lookup)
)
SELECT input FROM execution_log
WHERE execution_log.exec_id = $1
  AND execution_log.namespace_id = (SELECT id FROM namespace_lookup)
  AND execution_log.version = (SELECT max_version FROM latest_execution);


-- name: GetExecutionsByFlowPaginated :many
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
),
user_namespaces AS (
    -- Direct user membership
    SELECT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN users u ON nm.user_id = u.id
    WHERE u.uuid = $5 AND n.uuid = $2

    UNION

    -- Group membership
    SELECT DISTINCT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN groups g ON nm.group_id = g.id
    JOIN group_memberships gm ON g.id = gm.group_id
    WHERE gm.user_id = (SELECT id FROM users WHERE users.uuid = $5) AND n.uuid = $2
),
latest_versions AS (
    SELECT exec_id, MAX(version) as max_version
    FROM execution_log el
    INNER JOIN flows f ON el.flow_id = f.id
    WHERE f.id = $1
      AND f.namespace_id = (SELECT id FROM namespace_lookup)
      AND f.is_active = TRUE
    GROUP BY exec_id
),
filtered AS (
    SELECT el.*, u.name, u.username, u.uuid as triggered_by_uuid,
           CONCAT(u.name, ' <', u.username, '>')::TEXT as triggered_by_name,
           f.name as flow_name,
           f.slug as flow_slug
    FROM execution_log el
    INNER JOIN flows f ON el.flow_id = f.id
    INNER JOIN users u ON el.triggered_by = u.id
    INNER JOIN latest_versions lv ON el.exec_id = lv.exec_id AND el.version = lv.max_version
    WHERE f.id = $1
      AND f.namespace_id = (SELECT id FROM namespace_lookup)
      AND f.is_active = TRUE
      AND (el.scheduled_at IS NULL OR el.scheduled_at <= NOW())
      AND (
        el.triggered_by = (SELECT id FROM users WHERE users.uuid = $5)
        OR EXISTS (SELECT id FROM users WHERE users.uuid = $5 AND users.role = 'superuser')
        OR EXISTS (SELECT uuid FROM user_namespaces WHERE role IN ('admin', 'reviewer', 'operator'))
      )
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
latest_versions AS (
    SELECT exec_id, MAX(version) as max_version
    FROM execution_log el
    INNER JOIN flows f ON el.flow_id = f.id
    WHERE f.namespace_id = (SELECT id FROM namespace_lookup)
      AND f.is_active = TRUE
    GROUP BY exec_id
),
filtered AS (
    SELECT el.*, u.name, u.username, u.uuid as triggered_by_uuid,
           CONCAT(u.name, ' <', u.username, '>')::TEXT as triggered_by_name,
           f.name as flow_name,
           f.slug as flow_slug
    FROM execution_log el
    INNER JOIN flows f ON el.flow_id = f.id
    INNER JOIN users u ON el.triggered_by = u.id
    INNER JOIN latest_versions lv ON el.exec_id = lv.exec_id AND el.version = lv.max_version
    WHERE f.namespace_id = (SELECT id FROM namespace_lookup)
      AND f.is_active = TRUE
      AND (el.scheduled_at IS NULL OR el.scheduled_at <= NOW())
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

-- name: SearchExecutionsPaginated :many
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $1
),
user_namespaces AS (
    -- Direct user membership
    SELECT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN users u ON nm.user_id = u.id
    WHERE u.uuid = $5 AND n.uuid = $1

    UNION

    -- Group membership
    SELECT DISTINCT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN groups g ON nm.group_id = g.id
    JOIN group_memberships gm ON g.id = gm.group_id
    WHERE gm.user_id = (SELECT id FROM users WHERE users.uuid = $5) AND n.uuid = $1
),
latest_versions AS (
    SELECT exec_id, MAX(version) as max_version
    FROM execution_log el
    INNER JOIN flows f ON el.flow_id = f.id
    WHERE f.namespace_id = (SELECT id FROM namespace_lookup)
      AND f.is_active = TRUE
    GROUP BY exec_id
),
filtered AS (
    SELECT el.*, u.name, u.username, u.uuid as triggered_by_uuid,
           CONCAT(u.name, ' <', u.username, '>')::TEXT as triggered_by_name,
           f.name as flow_name,
           f.slug as flow_slug
    FROM execution_log el
    INNER JOIN flows f ON el.flow_id = f.id
    INNER JOIN users u ON el.triggered_by = u.id
    INNER JOIN latest_versions lv ON el.exec_id = lv.exec_id AND el.version = lv.max_version
    WHERE f.namespace_id = (SELECT id FROM namespace_lookup)
      AND f.is_active = TRUE
      AND (el.scheduled_at IS NULL OR el.scheduled_at <= NOW())
      AND (
        el.triggered_by = (SELECT id FROM users WHERE users.uuid = $5)
        OR EXISTS (SELECT id FROM users WHERE users.uuid = $5 AND users.role = 'superuser')
        OR EXISTS (SELECT uuid FROM user_namespaces WHERE role IN ('admin', 'reviewer', 'operator'))
      )
      AND (
        $2 = '' OR
        f.name ILIKE '%' || $2 || '%' OR
        f.slug ILIKE '%' || $2 || '%' OR
        el.exec_id ILIKE '%' || $2 || '%' OR
        u.name ILIKE '%' || $2 || '%' OR
        u.username ILIKE '%' || $2 || '%'
      )
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


-- name: ExecutionExistsForFlow :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
),
latest_versions AS (
    SELECT exec_id, MAX(version) as max_version
    FROM execution_log el
    INNER JOIN flows f ON el.flow_id = f.id
    WHERE f.namespace_id = (SELECT id FROM namespace_lookup)
    GROUP BY exec_id
)
SELECT exists (SELECT * FROM execution_log el INNER JOIN latest_versions lv on el.exec_id = lv.exec_id
WHERE flow_id = (SELECT id FROM flows WHERE flows.slug = $1 AND flows.namespace_id = (SELECT id FROM namespace_lookup) AND flows.is_active = TRUE) AND
namespace_id = (SELECT id FROM namespace_lookup) AND
(status = 'running' or status = 'pending_approval' or status = 'pending') AND
version = lv.max_version);

-- name: GetExecutionActionRetries :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
), latest_version AS (
    SELECT MAX(version) as version
    FROM execution_log el
    WHERE el.exec_id = $1 AND el.namespace_id = (SELECT id FROM namespace_lookup)
)
SELECT el.action_retries FROM execution_log el
WHERE el.exec_id = $1
  AND el.version = (SELECT version FROM latest_version)
  AND el.namespace_id = (SELECT id FROM namespace_lookup);

-- name: UpdateExecutionActionRetries :exec
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $3
), latest_version AS (
    SELECT MAX(version) as version
    FROM execution_log el
    WHERE el.exec_id = $1 AND el.namespace_id = (SELECT id FROM namespace_lookup)
)
UPDATE execution_log el
SET action_retries = $2, updated_at = NOW()
WHERE el.exec_id = $1
  AND el.version = (SELECT version FROM latest_version)
  AND el.namespace_id = (SELECT id FROM namespace_lookup);

-- name: IncrementActionRetry :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $3
), latest_version AS (
    SELECT MAX(version) as version
    FROM execution_log el
    WHERE el.exec_id = $1 AND el.namespace_id = (SELECT id FROM namespace_lookup)
)
UPDATE execution_log el
SET
    action_retries = jsonb_set(
        COALESCE(action_retries, '{}'::jsonb),
        ARRAY[$2],
        to_jsonb((COALESCE(action_retries->>$2, '0')::int + 1))
    ),
    updated_at = NOW()
WHERE el.exec_id = $1
  AND el.version = (SELECT version FROM latest_version)
  AND el.namespace_id = (SELECT id FROM namespace_lookup)
RETURNING
    action_retries,
    (action_retries->>$2)::int as retry_count;

-- name: GetScheduledExecutionsByFlow :many
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
),
latest_versions AS (
    SELECT exec_id, MAX(version) as max_version
    FROM execution_log el
    WHERE el.flow_id = $1
    GROUP BY exec_id
)
SELECT el.exec_id, el.scheduled_at
FROM execution_log el
INNER JOIN latest_versions lv ON el.exec_id = lv.exec_id AND el.version = lv.max_version
INNER JOIN flows f ON el.flow_id = f.id
WHERE el.flow_id = $1
  AND f.namespace_id = (SELECT id FROM namespace_lookup)
  AND f.is_active = TRUE
  AND el.scheduled_at IS NOT NULL
  AND el.scheduled_at > NOW()
  AND el.status = 'pending'
ORDER BY el.scheduled_at ASC;

-- name: UpdateExecutionStartedAt :exec
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
), latest_version AS (
    SELECT MAX(version) as version
    FROM execution_log
    WHERE execution_log.exec_id = $1 AND namespace_id = (SELECT id FROM namespace_lookup)
)
UPDATE execution_log SET
    started_at = NOW(),
    updated_at = NOW()
WHERE execution_log.exec_id = $1
  AND version = (SELECT version FROM latest_version)
  AND namespace_id = (SELECT id FROM namespace_lookup)
  AND started_at IS NULL;

-- name: GetExecutionStatusByExecID :one
WITH latest_version AS (
    SELECT MAX(version) as version
    FROM execution_log
    WHERE execution_log.exec_id = $1
)
SELECT status
FROM execution_log
WHERE execution_log.exec_id = $1
  AND version = (SELECT version FROM latest_version);

-- name: GetFinishedExecutionsOlderThan :many
WITH latest_version AS (
    SELECT exec_id, MAX(version) as version
    FROM execution_log
    GROUP BY exec_id
)
SELECT el.exec_id, f.slug as flow_slug
FROM execution_log el
INNER JOIN latest_version lv ON el.exec_id = lv.exec_id AND el.version = lv.version
INNER JOIN flows f ON el.flow_id = f.id
WHERE el.status IN ('completed'::execution_status, 'errored'::execution_status, 'cancelled'::execution_status)
  AND el.updated_at < $1;


