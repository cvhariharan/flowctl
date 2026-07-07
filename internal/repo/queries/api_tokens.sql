-- name: CreateAPIToken :one
INSERT INTO api_tokens (user_id, label, token_hash)
VALUES (
    (SELECT id FROM users WHERE users.uuid = sqlc.arg(user_uuid)),
    sqlc.arg(label),
    sqlc.arg(token_hash)
)
RETURNING *;

-- name: GetAPITokenByHash :one
SELECT t.id, t.uuid, t.user_id, t.label, t.token_hash, t.last_used_at, t.created_at, t.updated_at,
       u.uuid AS user_uuid
FROM api_tokens t
JOIN users u ON u.id = t.user_id
WHERE t.token_hash = $1;

-- name: ListAPITokensByUser :many
SELECT * FROM api_tokens
WHERE user_id = (SELECT id FROM users WHERE users.uuid = sqlc.arg(user_uuid))
ORDER BY created_at DESC;

-- name: DeleteAPIToken :exec
DELETE FROM api_tokens
WHERE api_tokens.uuid = sqlc.arg(token_uuid)
  AND user_id = (SELECT id FROM users WHERE users.uuid = sqlc.arg(user_uuid));

-- name: TouchAPITokenLastUsed :exec
UPDATE api_tokens SET last_used_at = NOW() WHERE id = $1;

