-- name: GetUserByUUID :one
SELECT * FROM users WHERE uuid = $1;

-- name: DeleteUserByUUID :exec
DELETE FROM users WHERE uuid = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: GetUserByUsernameWithGroups :one
SELECT * FROM user_view WHERE username = $1;

-- name: GetUserByUUIDWithGroups :one
SELECT * FROM user_view WHERE uuid = $1;

-- name: GetAllUsersWithGroups :many
SELECT * FROM user_view;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (
    username,
    password,
    login_type,
    role,
    name
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: SearchUsersWithGroups :many
SELECT * FROM user_view WHERE lower(name) LIKE '%' || lower($1::text) || '%' OR lower(username) LIKE '%' || lower($1::text) || '%';

-- name: UpdateUserByUUID :one
UPDATE users SET name = $1, username = $2 WHERE uuid = $3 RETURNING *;

-- name: AddGroupToUserByUUID :exec
WITH
user_lookup AS (
    SELECT id FROM users WHERE users.uuid = sqlc.arg(user_uuid)
),
group_lookup AS (
    SELECT id FROM groups WHERE groups.uuid = sqlc.arg(group_uuid)
)
INSERT INTO group_memberships (user_id, group_id) VALUES (
    ( SELECT id FROM user_lookup ),
    ( SELECT id FROM group_lookup )
);
