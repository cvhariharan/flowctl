-- Immediate task operations
-- name: CreateSchedulerTask :one
INSERT INTO scheduler_tasks (uuid, exec_id, payload, status) 
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetPendingTasks :many
SELECT * FROM scheduler_tasks 
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1;

-- name: UpdateTaskStatus :exec
UPDATE scheduler_tasks SET status = $1 WHERE id = $2;

-- name: CancelTasksByExecID :exec
UPDATE scheduler_tasks SET status = 'cancelled' WHERE exec_id = $1 AND status = 'pending';

