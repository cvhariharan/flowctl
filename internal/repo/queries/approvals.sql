-- name: AddApprovalRequest :one
INSERT INTO approvals (
    exec_log_id,
    approvers,
    action_id
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: ApproveRequestByUUID :one
UPDATE approvals SET status = 'approved', updated_at = NOW() WHERE uuid = $1 RETURNING *;

-- name: RejectRequestByUUID :one
UPDATE approvals SET status = 'rejected', updated_at = NOW() WHERE uuid = $1 RETURNING *;

-- name: UpdateApprovalStatusByUUID :one
UPDATE approvals SET status = $1, updated_at = NOW() WHERE uuid = $1 RETURNING *;

-- name: GetApprovalByUUID :one
SELECT * FROM approvals WHERE uuid = $1;

-- name: GetApprovalRequestsForActionAndExec :one
WITH exec_lookup AS (
    SELECT id FROM execution_log WHERE exec_id = $1
)
SELECT * FROM approvals WHERE exec_log_id = (SELECT id FROM exec_lookup) AND action_id = $2;
