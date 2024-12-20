-- name: GetUserByUUID :one
SELECT * from users WHERE uuid = $1;

-- name: GetUserByID :one
SELECT * from users WHERE id = $1;

-- name: GetUserPassword :one
SELECT password FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT * from users WHERE username = $1;

-- name: CreateUser :one
INSERT INTO users (
    username,
    password,
    login_type,
    role
) VALUES (
    $1, $2, $3, $4
) RETURNING *;