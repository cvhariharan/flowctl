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