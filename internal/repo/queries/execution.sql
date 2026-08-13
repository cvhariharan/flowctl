-- name: AddExecution :one
WITH user_lookup AS (
    SELECT id FROM users WHERE users.uuid = $4
), namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $5
)
INSERT INTO executions (
    exec_id, flow_id, inputs, trigger_type, triggered_by, namespace_id, scheduled_at
) VALUES (
    $1, $2, $3, $6, (SELECT id FROM user_lookup), (SELECT id FROM namespace_lookup), $7
) RETURNING *;

-- name: InsertExecutionEvent :execrows
INSERT INTO execution_events (exec_id, attempt, action_id, type, error, outputs, created_at)
SELECT sqlc.arg(exec_id)::varchar AS exec_id,
       sqlc.arg(attempt)::integer AS attempt,
       sqlc.narg(action_id)::varchar AS action_id,
       sqlc.arg(event_type)::execution_event_type AS type,
       sqlc.narg(error)::text AS error,
       sqlc.narg(outputs)::jsonb AS outputs,
       COALESCE(sqlc.narg(created_at)::timestamptz, NOW()) AS created_at
WHERE EXISTS (
    SELECT 1 FROM executions
    WHERE exec_id = sqlc.arg(exec_id) AND attempt = sqlc.arg(attempt)
);

-- name: ProjectExecutionEvent :execresult
UPDATE executions SET
    status = CASE sqlc.arg(event_type)::execution_event_type
        WHEN 'queued' THEN 'pending'::execution_status
        WHEN 'started' THEN 'running'::execution_status
        WHEN 'waiting_approval' THEN 'pending_approval'::execution_status
        WHEN 'completed' THEN 'completed'::execution_status
        WHEN 'errored' THEN 'errored'::execution_status
        WHEN 'cancelled' THEN 'cancelled'::execution_status
        ELSE status
    END,
    error = CASE
        WHEN sqlc.arg(event_type)::execution_event_type IN ('queued', 'started') THEN NULL
        WHEN sqlc.arg(event_type)::execution_event_type IN ('completed', 'errored', 'cancelled') THEN sqlc.narg(error)
        ELSE error
    END,
    outputs = CASE WHEN sqlc.narg(outputs)::jsonb IS NOT NULL
        THEN outputs || sqlc.narg(outputs)::jsonb ELSE outputs END,
    started_at = CASE WHEN sqlc.arg(event_type)::execution_event_type = 'started'
        THEN COALESCE(started_at, COALESCE(sqlc.narg(created_at), NOW())) ELSE started_at END,
    completed_at = CASE
        WHEN sqlc.arg(event_type)::execution_event_type IN ('queued', 'started') THEN NULL
        WHEN sqlc.arg(event_type)::execution_event_type IN ('completed', 'errored', 'cancelled')
            THEN COALESCE(sqlc.narg(created_at), NOW())
        ELSE completed_at
    END,
    updated_at = NOW()
WHERE exec_id = sqlc.arg(exec_id) AND attempt = sqlc.arg(attempt);

-- name: LoadExecutionEvents :many
SELECT * FROM execution_events WHERE exec_id = $1 ORDER BY seq;

-- name: BeginAttempt :one
UPDATE executions SET attempt = attempt + 1, updated_at = NOW()
WHERE exec_id = $1 AND status IN ('pending', 'running', 'errored')
RETURNING attempt;

-- name: RequeueExecution :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
UPDATE executions SET status = 'pending', updated_at = NOW()
WHERE exec_id = $1
  AND namespace_id = (SELECT id FROM namespace_lookup)
  AND status IN ('errored', 'cancelled', 'pending_approval')
RETURNING attempt;

-- name: RequeueExecutionForReset :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
UPDATE executions SET status = 'pending', updated_at = NOW()
WHERE exec_id = $1
  AND namespace_id = (SELECT id FROM namespace_lookup)
  AND status IN ('completed', 'errored', 'cancelled')
RETURNING attempt;

-- name: CancelExecution :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
UPDATE executions SET
    status = 'cancelled', completed_at = NOW(), updated_at = NOW(), attempt = attempt + 1
WHERE exec_id = $1
  AND namespace_id = (SELECT id FROM namespace_lookup)
  AND status IN ('pending', 'running', 'pending_approval')
RETURNING *;

-- name: GetExecutionsByFlow :many
WITH user_lookup AS (
    SELECT id FROM users WHERE users.uuid = $2
), namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $3
)
SELECT el.*, u.name, u.username, u.uuid AS triggered_by_uuid,
       CONCAT(u.name, ' <', u.username, '>')::TEXT AS triggered_by_name,
       f.name AS flow_name, f.slug AS flow_slug
FROM executions el
JOIN flows f ON el.flow_id = f.id
JOIN users u ON el.triggered_by = u.id
WHERE f.id = $1
  AND el.triggered_by = (SELECT id FROM user_lookup)
  AND f.namespace_id = (SELECT id FROM namespace_lookup)
  AND f.is_active = TRUE;

-- name: GetExecutionByExecID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT el.*, u.name, u.username, u.uuid AS triggered_by_uuid,
       CONCAT(u.name, ' <', u.username, '>')::TEXT AS triggered_by_name,
       f.name AS flow_name, f.slug AS flow_slug
FROM executions el
JOIN users u ON el.triggered_by = u.id
JOIN flows f ON el.flow_id = f.id
WHERE el.exec_id = $1
  AND el.namespace_id = (SELECT id FROM namespace_lookup)
  AND f.is_active = TRUE;

-- name: GetExecutionByExecIDWithNamespace :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT el.*, u.name, u.username, u.uuid AS triggered_by_uuid,
       CONCAT(u.name, ' <', u.username, '>')::TEXT AS triggered_by_name,
       f.name AS flow_name, f.slug AS flow_slug
FROM executions el
JOIN users u ON el.triggered_by = u.id
JOIN flows f ON el.flow_id = f.id
WHERE el.exec_id = $1
  AND f.namespace_id = (SELECT id FROM namespace_lookup)
  AND f.is_active = TRUE;

-- name: GetFlowFromExecID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT f.* FROM flows f
JOIN executions el ON el.flow_id = f.id
WHERE el.exec_id = $1
  AND f.namespace_id = (SELECT id FROM namespace_lookup)
  AND f.is_active = TRUE;

-- name: GetFlowFromExecIDWithNamespace :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT f.* FROM flows f
JOIN executions el ON el.flow_id = f.id
WHERE el.exec_id = $1
  AND f.namespace_id = (SELECT id FROM namespace_lookup)
  AND f.is_active = TRUE;

-- name: GetExecutionByID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT el.*, u.name, u.username, u.uuid AS triggered_by_uuid,
       CONCAT(u.name, ' <', u.username, '>')::TEXT AS triggered_by_name,
       f.name AS flow_name, f.slug AS flow_slug
FROM executions el
JOIN users u ON el.triggered_by = u.id
JOIN flows f ON el.flow_id = f.id
WHERE el.id = $1
  AND el.namespace_id = (SELECT id FROM namespace_lookup)
  AND f.is_active = TRUE;

-- name: GetExecutionContextByUUID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT jsonb_build_object('inputs', inputs, 'outputs', outputs)::jsonb AS context
FROM executions
WHERE exec_id = $1 AND namespace_id = (SELECT id FROM namespace_lookup);

-- name: GetExecutionsByFlowPaginated :many
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
), user_namespaces AS (
    SELECT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN users u ON nm.user_id = u.id
    WHERE u.uuid = $5 AND n.uuid = $2
    UNION
    SELECT DISTINCT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN groups g ON nm.group_id = g.id
    JOIN group_memberships gm ON g.id = gm.group_id
    WHERE gm.user_id = (SELECT id FROM users WHERE users.uuid = $5) AND n.uuid = $2
), filtered AS (
    SELECT el.*, u.name, u.username, u.uuid AS triggered_by_uuid,
           CONCAT(u.name, ' <', u.username, '>')::TEXT AS triggered_by_name,
           f.name AS flow_name, f.slug AS flow_slug
    FROM executions el
    JOIN flows f ON el.flow_id = f.id
    JOIN users u ON el.triggered_by = u.id
    WHERE f.id = $1
      AND f.namespace_id = (SELECT id FROM namespace_lookup)
      AND f.is_active = TRUE
      AND (el.scheduled_at IS NULL OR el.scheduled_at <= NOW())
      AND (
        el.triggered_by = (SELECT id FROM users WHERE users.uuid = $5)
        OR EXISTS (SELECT id FROM users WHERE users.uuid = $5 AND users.role = 'superuser')
        OR EXISTS (SELECT uuid FROM user_namespaces WHERE role IN ('admin', 'reviewer', 'operator'))
      )
), total AS (
    SELECT COUNT(*) AS total_count FROM filtered
), paged AS (
    SELECT * FROM filtered ORDER BY created_at DESC LIMIT $3 OFFSET $4
), page_count AS (
    SELECT CEIL(total.total_count::numeric / $3::numeric)::bigint AS page_count FROM total
)
SELECT p.*, pc.page_count, t.total_count FROM paged p, page_count pc, total t;

-- name: GetAllExecutionsPaginated :many
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $1
), filtered AS (
    SELECT el.*, u.name, u.username, u.uuid AS triggered_by_uuid,
           CONCAT(u.name, ' <', u.username, '>')::TEXT AS triggered_by_name,
           f.name AS flow_name, f.slug AS flow_slug
    FROM executions el
    JOIN flows f ON el.flow_id = f.id
    JOIN users u ON el.triggered_by = u.id
    WHERE f.namespace_id = (SELECT id FROM namespace_lookup)
      AND f.is_active = TRUE
      AND (el.scheduled_at IS NULL OR el.scheduled_at <= NOW())
), total AS (
    SELECT COUNT(*) AS total_count FROM filtered
), paged AS (
    SELECT * FROM filtered ORDER BY created_at DESC LIMIT $2 OFFSET $3
), page_count AS (
    SELECT CEIL(total.total_count::numeric / $2::numeric)::bigint AS page_count FROM total
)
SELECT p.*, pc.page_count, t.total_count FROM paged p, page_count pc, total t;

-- name: SearchExecutionsPaginated :many
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $1
), user_namespaces AS (
    SELECT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN users u ON nm.user_id = u.id
    WHERE u.uuid = $5 AND n.uuid = $1
    UNION
    SELECT DISTINCT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN groups g ON nm.group_id = g.id
    JOIN group_memberships gm ON g.id = gm.group_id
    WHERE gm.user_id = (SELECT id FROM users WHERE users.uuid = $5) AND n.uuid = $1
), filtered AS (
    SELECT el.*, u.name, u.username, u.uuid AS triggered_by_uuid,
           CONCAT(u.name, ' <', u.username, '>')::TEXT AS triggered_by_name,
           f.name AS flow_name, f.slug AS flow_slug
    FROM executions el
    JOIN flows f ON el.flow_id = f.id
    JOIN users u ON el.triggered_by = u.id
    WHERE f.namespace_id = (SELECT id FROM namespace_lookup)
      AND f.is_active = TRUE
      AND (el.scheduled_at IS NULL OR el.scheduled_at <= NOW())
      AND (
        el.triggered_by = (SELECT id FROM users WHERE users.uuid = $5)
        OR EXISTS (SELECT id FROM users WHERE users.uuid = $5 AND users.role = 'superuser')
        OR EXISTS (SELECT uuid FROM user_namespaces WHERE role IN ('admin', 'reviewer', 'operator'))
      )
      AND ($2 = '' OR f.name ILIKE '%' || $2 || '%' OR f.slug ILIKE '%' || $2 || '%'
        OR el.exec_id ILIKE '%' || $2 || '%' OR u.name ILIKE '%' || $2 || '%'
        OR u.username ILIKE '%' || $2 || '%')
), total AS (
    SELECT COUNT(*) AS total_count FROM filtered
), paged AS (
    SELECT * FROM filtered ORDER BY created_at DESC LIMIT $3 OFFSET $4
), page_count AS (
    SELECT CEIL(total.total_count::numeric / $3::numeric)::bigint AS page_count FROM total
)
SELECT p.*, pc.page_count, t.total_count FROM paged p, page_count pc, total t;

-- name: ExecutionExistsForFlow :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT EXISTS (
    SELECT 1 FROM executions
    WHERE flow_id = (
        SELECT id FROM flows
        WHERE slug = $1 AND namespace_id = (SELECT id FROM namespace_lookup) AND is_active = TRUE
    )
      AND namespace_id = (SELECT id FROM namespace_lookup)
      AND status IN ('running', 'pending_approval', 'pending')
);

-- name: GetScheduledExecutionsByFlow :many
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT el.exec_id, el.scheduled_at
FROM executions el
JOIN flows f ON el.flow_id = f.id
WHERE el.flow_id = $1
  AND f.namespace_id = (SELECT id FROM namespace_lookup)
  AND f.is_active = TRUE
  AND el.scheduled_at IS NOT NULL
  AND el.scheduled_at > NOW()
  AND el.status = 'pending'
ORDER BY el.scheduled_at;

-- name: DeleteExpiredExecutions :many
WITH expired AS (
    SELECT exec_id FROM executions
    WHERE status IN ('completed', 'errored', 'cancelled')
      AND COALESCE(executions.completed_at, executions.updated_at) < sqlc.arg(cutoff)
    ORDER BY COALESCE(executions.completed_at, executions.updated_at)
    LIMIT sqlc.arg(batch_size)
)
DELETE FROM executions
WHERE exec_id IN (SELECT exec_id FROM expired)
  AND status IN ('completed', 'errored', 'cancelled')
RETURNING exec_id;

-- name: DeleteExecutionEventsByExecIDs :exec
DELETE FROM execution_events WHERE exec_id = ANY(sqlc.arg(exec_ids)::varchar[]);

-- name: ListExecutionIDs :many
SELECT exec_id FROM executions ORDER BY exec_id;

-- name: GetExecutionProjection :one
SELECT * FROM executions WHERE exec_id = $1;

-- name: UpdateExecutionProjection :exec
UPDATE executions SET status = $2, error = $3, outputs = $4, started_at = $5,
    completed_at = $6, updated_at = NOW()
WHERE exec_id = $1;
