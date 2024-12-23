-- name: GetUserByUUID :one
SELECT * FROM users WHERE uuid = $1;

-- name: DeleteUserByUUID :exec
DELETE FROM users WHERE uuid = $1;

-- name: GetUserByUsername :one
SELECT
    u.id AS user_id,
    u.uuid,
    u.username,
    u.password,
    array_agg(g.name) AS group_names,
    array_agg(g.description) AS group_descriptions
FROM
    users u
LEFT JOIN
    group_memberships gm ON u.id = gm.user_id
LEFT JOIN
    groups g ON gm.group_id = g.id
WHERE
    u.username = $1
GROUP BY
    u.id, u.uuid, u.username, u.password;

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

-- name: SearchUser :many
SELECT * FROM user_view WHERE lower(name) LIKE '%' || lower($1::text) || '%' OR lower(username) LIKE '%' || lower($1::text) || '%';
