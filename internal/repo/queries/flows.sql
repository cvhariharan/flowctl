-- name: GetFlowBySlug :one
SELECT * FROM flows where slug = $1;

-- name: DeleteAllFlows :exec
DELETE FROM flows;

-- name: CreateFlow :one
INSERT INTO flows (
    slug,
    name,
    description,
    checksum
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: UpdateFlow :one
UPDATE flows SET 
    name = $1,
    description = $2,
    checksum = $3,
    updated_at = NOW()
WHERE slug = $4
RETURNING *;