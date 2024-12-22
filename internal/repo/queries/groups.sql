-- name: CreateGroup :one
INSERT INTO groups (
    name,
    description
) VALUES (
    $1, $2
) RETURNING *;


-- name: GetAllGroupsWithUsers :many
SELECT * FROM group_view;

-- name: GetGroupByUUIDWithUsers :one
SELECT * FROM group_view WHERE uuid = $1;

-- name: GetGroupByUUID :one
SELECT * FROM groups WHERE uuid = $1;

-- name: DeleteGroupByUUID :exec
DELETE FROM groups WHERE uuid = $1;

-- name: SearchGroup :many
SELECT * FROM group_view WHERE lower(name) LIKE '%' || lower($1::text) || '%' OR lower(description) LIKE '%' || lower($1::text) || '%';
