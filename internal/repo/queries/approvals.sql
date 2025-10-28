-- name: AddApprovalRequest :one
WITH inserted_approval AS (
    INSERT INTO approvals (
        exec_log_id,
        action_id,
        namespace_id
    ) VALUES (
        $1, $2, (SELECT id FROM namespaces where namespaces.uuid = $3)
    ) RETURNING *
)
SELECT
    a.*,
    u.name as requested_by
FROM inserted_approval a
JOIN execution_log el ON a.exec_log_id = el.id
JOIN users u ON el.triggered_by = u.id;

-- name: ApproveRequestByUUID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $3
), updated AS (
    UPDATE approvals SET status = 'approved', decided_by = $2, updated_at = NOW()
    WHERE approvals.uuid = $1
    AND approvals.exec_log_id IN (
        SELECT el.id FROM execution_log el
        JOIN flows f ON el.flow_id = f.id
        WHERE f.namespace_id = (SELECT id FROM namespace_lookup)
    )
    RETURNING *
)
SELECT
    a.*,
    u.name as requested_by
FROM updated a
JOIN execution_log el ON a.exec_log_id = el.id
JOIN users u ON el.triggered_by = u.id;

-- name: RejectRequestByUUID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $3
), updated AS (
    UPDATE approvals SET status = 'rejected', decided_by = $2, updated_at = NOW()
    WHERE approvals.uuid = $1
    AND approvals.exec_log_id IN (
        SELECT el.id FROM execution_log el
        JOIN flows f ON el.flow_id = f.id
        WHERE f.namespace_id = (SELECT id FROM namespace_lookup)
    )
    RETURNING *
)
SELECT
    a.*,
    el.exec_id,
    u.name as requested_by
FROM updated a
JOIN execution_log el ON a.exec_log_id = el.id
JOIN users u ON el.triggered_by = u.id;

-- name: UpdateApprovalStatusByUUID :one
WITH updated AS (
    UPDATE approvals SET status = $1, decided_by = $2, updated_at = NOW()
    WHERE uuid = $1
    RETURNING *
)
SELECT
    a.*,
    u.name as requested_by
FROM updated a
JOIN execution_log el ON a.exec_log_id = el.id
JOIN users u ON el.triggered_by = u.id;

-- name: GetApprovalByUUID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT
    a.*,
    el.exec_id,
    u.name as requested_by
FROM approvals a
JOIN execution_log el ON a.exec_log_id = el.id
JOIN flows f ON el.flow_id = f.id
JOIN users u ON el.triggered_by = u.id
WHERE a.uuid = $1 AND f.namespace_id = (SELECT id FROM namespace_lookup);

-- name: GetApprovalWithInputsByUUID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT
    a.*,
    el.exec_id,
    el.input as exec_inputs,
    f.name as flow_name,
    f.slug as flow_slug,
    u.name as requested_by,
    us.name as decided_by_name
FROM approvals a
JOIN execution_log el ON a.exec_log_id = el.id
JOIN flows f ON el.flow_id = f.id
JOIN users u ON el.triggered_by = u.id
LEFT JOIN users us ON a.decided_by = us.id
WHERE a.uuid = $1 AND f.namespace_id = (SELECT id FROM namespace_lookup);

-- name: GetApprovalRequestForActionAndExec :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $3
)
SELECT a.* FROM approvals a
JOIN execution_log el ON a.exec_log_id = el.id
JOIN flows f ON el.flow_id = f.id
WHERE el.exec_id = $1
  AND a.action_id = $2
  AND f.namespace_id = (SELECT id FROM namespace_lookup);

-- name: GetApprovalRequestForExec :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
), latest_version AS (
    SELECT MAX(version) as max_version
    FROM execution_log
    WHERE exec_id = $1
      AND namespace_id = (SELECT id FROM namespace_lookup)
)
SELECT
    a.*,
    el.exec_id,
    u.name as requested_by
FROM approvals a
JOIN execution_log el ON a.exec_log_id = el.id
JOIN flows f ON el.flow_id = f.id
JOIN users u ON el.triggered_by = u.id
WHERE el.exec_id = $1
  AND f.namespace_id = (SELECT id FROM namespace_lookup)
  AND el.version = (SELECT max_version FROM latest_version);

-- name: GetApprovalsPaginated :many
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $1
),
filtered AS (
    SELECT
        a.*,
        el.exec_id,
        u.name as requested_by,
        f.name as flow_name
    FROM approvals a
    JOIN execution_log el ON a.exec_log_id = el.id
    JOIN flows f ON el.flow_id = f.id
    JOIN users u ON el.triggered_by = u.id
    WHERE f.namespace_id = (SELECT id FROM namespace_lookup)
      AND (CASE WHEN $2::text = '' THEN TRUE ELSE a.status = $2::approval_status END)
      AND (
        $3 = '' OR
        a.action_id ILIKE '%' || $3 || '%' OR
        el.exec_id ILIKE '%' || $3 || '%' OR
        u.name ILIKE '%' || $3 || '%'
      )
),
total AS (
    SELECT COUNT(*) AS total_count
    FROM filtered
),
paged AS (
    SELECT *
    FROM filtered
    ORDER BY created_at DESC
    LIMIT $4 OFFSET $5
),
page_count AS (
    SELECT CEIL(total.total_count::numeric / $4::numeric)::bigint AS page_count
    FROM total
)
SELECT
    p.*,
    pc.page_count,
    t.total_count
FROM paged p, page_count pc, total t;
