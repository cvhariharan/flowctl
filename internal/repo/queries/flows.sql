-- name: GetFlowBySlug :one
SELECT * FROM flows where slug = $1;

-- name: DeleteAllFlows :exec
DELETE FROM flows;

-- name: CreateFlow :one
INSERT INTO flows (
    slug,
    name,
    description
) VALUES (
    $1, $2, $3
) RETURNING *;