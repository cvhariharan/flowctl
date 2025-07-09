-- name: CreateCredential :one
INSERT INTO credentials (name, private_key, password)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetCredentialByUUID :one
SELECT * FROM credentials WHERE uuid = $1;

-- name: GetCredentialByID :one
SELECT * FROM credentials WHERE id = $1;

-- name: ListCredentials :many
WITH filtered AS (
    SELECT * FROM credentials
),
total AS (
    SELECT COUNT(*) AS total_count FROM filtered
),
paged AS (
    SELECT * FROM filtered
    ORDER BY created_at DESC
    LIMIT $1 OFFSET $2
),
page_count AS (
    SELECT COUNT(*) AS page_count FROM paged
)
SELECT
    p.*,
    pc.page_count,
    t.total_count
FROM paged p, page_count pc, total t;

-- name: UpdateCredential :one
UPDATE credentials
SET name = $2, private_key = $3, password = $4, updated_at = NOW()
WHERE uuid = $1
RETURNING *;

-- name: DeleteCredential :exec
DELETE FROM credentials WHERE uuid = $1;
