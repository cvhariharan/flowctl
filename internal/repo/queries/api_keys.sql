-- name: CreateAPIKey :one
INSERT INTO api_keys (
    name,
    key_hash,
    key_prefix,
    user_id,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetAPIKeyByPrefix :one
SELECT ak.*, u.uuid as user_uuid, u.name as user_name, u.username, u.login_type, u.role
FROM api_keys ak
JOIN users u ON ak.user_id = u.id
WHERE ak.key_prefix = $1;

-- name: GetAPIKeysByUserUUID :many
SELECT ak.*
FROM api_keys ak
JOIN users u ON ak.user_id = u.id
WHERE u.uuid = $1
ORDER BY ak.created_at DESC;

-- name: GetAPIKeyByUUID :one
SELECT * FROM api_keys WHERE uuid = $1;

-- name: DeleteAPIKeyByUUID :exec
DELETE FROM api_keys WHERE uuid = $1;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE api_keys SET last_used_at = NOW(), updated_at = NOW() WHERE id = $1;

-- name: GetUserIDByUUID :one
SELECT id FROM users WHERE uuid = $1;
