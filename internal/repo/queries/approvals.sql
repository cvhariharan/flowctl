-- name: AddApprovalRequest :one
WITH inserted_approval AS (
    INSERT INTO approvals (exec_id, action_id, namespace_id)
    VALUES ($1, $2, (SELECT id FROM namespaces WHERE namespaces.uuid = $3))
    RETURNING *
)
SELECT a.*, u.name AS requested_by
FROM inserted_approval a
JOIN executions el ON a.exec_id = el.exec_id
JOIN users u ON el.triggered_by = u.id;

-- name: ApproveRequestByUUID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $3
), updated AS (
    UPDATE approvals SET status = 'approved', decided_by = $2, updated_at = NOW()
    WHERE approvals.uuid = $1
      AND approvals.exec_id IN (
        SELECT el.exec_id FROM executions el JOIN flows f ON el.flow_id = f.id
        WHERE f.namespace_id = (SELECT id FROM namespace_lookup) AND f.is_active = TRUE
      )
    RETURNING *
)
SELECT a.*, u.name AS requested_by
FROM updated a
JOIN executions el ON a.exec_id = el.exec_id
JOIN users u ON el.triggered_by = u.id;

-- name: RejectRequestByUUID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $3
), updated AS (
    UPDATE approvals SET status = 'rejected', decided_by = $2, updated_at = NOW()
    WHERE approvals.uuid = $1
      AND approvals.exec_id IN (
        SELECT el.exec_id FROM executions el JOIN flows f ON el.flow_id = f.id
        WHERE f.namespace_id = (SELECT id FROM namespace_lookup) AND f.is_active = TRUE
      )
    RETURNING *
)
SELECT a.*, u.name AS requested_by
FROM updated a
JOIN executions el ON a.exec_id = el.exec_id
JOIN users u ON el.triggered_by = u.id;

-- name: UpdateApprovalStatusByUUID :one
WITH updated AS (
    UPDATE approvals SET status = $1, decided_by = $2, updated_at = NOW()
    WHERE approvals.uuid = $3 RETURNING *
)
SELECT a.*, u.name AS requested_by
FROM updated a
JOIN executions el ON a.exec_id = el.exec_id
JOIN users u ON el.triggered_by = u.id;

-- name: GetApprovalByUUID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT a.*, u.name AS requested_by
FROM approvals a
JOIN executions el ON a.exec_id = el.exec_id
JOIN flows f ON el.flow_id = f.id
JOIN users u ON el.triggered_by = u.id
WHERE a.uuid = $1 AND f.namespace_id = (SELECT id FROM namespace_lookup) AND f.is_active = TRUE;

-- name: GetApprovalWithInputsByUUID :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT a.*, el.inputs AS exec_inputs, f.name AS flow_name, f.slug AS flow_slug,
       u.name AS requested_by, us.name AS decided_by_name
FROM approvals a
JOIN executions el ON a.exec_id = el.exec_id
JOIN flows f ON el.flow_id = f.id
JOIN users u ON el.triggered_by = u.id
LEFT JOIN users us ON a.decided_by = us.id
WHERE a.uuid = $1 AND f.namespace_id = (SELECT id FROM namespace_lookup) AND f.is_active = TRUE;

-- name: GetApprovalRequestForActionAndExec :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $3
)
SELECT a.* FROM approvals a
JOIN executions el ON a.exec_id = el.exec_id
JOIN flows f ON el.flow_id = f.id
WHERE a.exec_id = $1 AND a.action_id = $2
  AND f.namespace_id = (SELECT id FROM namespace_lookup) AND f.is_active = TRUE;

-- name: GetApprovalRequestForExec :one
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT a.*, u.name AS requested_by
FROM approvals a
JOIN executions el ON a.exec_id = el.exec_id
JOIN flows f ON el.flow_id = f.id
JOIN users u ON el.triggered_by = u.id
WHERE a.exec_id = $1 AND f.namespace_id = (SELECT id FROM namespace_lookup) AND f.is_active = TRUE
ORDER BY a.created_at, a.id LIMIT 1;

-- name: GetApprovalRequestsForExec :many
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $2
)
SELECT a.*, u.name AS requested_by
FROM approvals a
JOIN executions el ON a.exec_id = el.exec_id
JOIN flows f ON el.flow_id = f.id
JOIN users u ON el.triggered_by = u.id
WHERE a.exec_id = $1 AND f.namespace_id = (SELECT id FROM namespace_lookup) AND f.is_active = TRUE
ORDER BY a.created_at, a.id;

-- name: GetApprovalsPaginated :many
WITH namespace_lookup AS (
    SELECT id FROM namespaces WHERE namespaces.uuid = $1
), filtered AS (
    SELECT a.*, u.name AS requested_by, f.name AS flow_name
    FROM approvals a
    JOIN executions el ON a.exec_id = el.exec_id
    JOIN flows f ON el.flow_id = f.id
    JOIN users u ON el.triggered_by = u.id
    WHERE f.namespace_id = (SELECT id FROM namespace_lookup)
      AND f.is_active = TRUE
      AND (CASE WHEN $2::text = '' THEN TRUE ELSE a.status = $2::approval_status END)
      AND ($3 = '' OR a.action_id ILIKE '%' || $3 || '%' OR a.exec_id ILIKE '%' || $3 || '%' OR u.name ILIKE '%' || $3 || '%')
), total AS (
    SELECT COUNT(*) AS total_count FROM filtered
), paged AS (
    SELECT * FROM filtered ORDER BY created_at DESC LIMIT $4 OFFSET $5
), page_count AS (
    SELECT CEIL(total.total_count::numeric / $4::numeric)::bigint AS page_count FROM total
)
SELECT p.*, pc.page_count, t.total_count FROM paged p, page_count pc, total t;
