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
