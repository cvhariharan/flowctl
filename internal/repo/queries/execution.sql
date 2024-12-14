-- name: AddToQueue :one
INSERT INTO execution_queue (
    flow_id,
    input
) VALUES (
    $1, $2
) RETURNING *;

-- name: UpdateStatusByID :exec
UPDATE execution_queue SET status = $2 WHERE id = $1;

-- name: GetFromQueueByID :one
SELECT * FROM execution_queue WHERE id = $1;

-- name: Dequeue :many
UPDATE execution_queue SET status = 'running' WHERE id IN (
    SELECT id FROM execution_queue WHERE status = 'pending' ORDER BY created_at LIMIT $1 FOR UPDATE SKIP LOCKED
) RETURNING uuid, flow_id, input;


-- name: DequeueByID :one
UPDATE execution_queue SET status = 'running' WHERE id = (
    SELECT id FROM execution_queue WHERE status = 'pending' AND execution_queue.id = $1 FOR UPDATE SKIP LOCKED
) RETURNING uuid, flow_id, input;