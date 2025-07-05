-- name: CreateGroup :one
INSERT INTO groups (
    name,
    description
) VALUES (
    $1, $2
) RETURNING *;


-- name: GetAllGroupsWithUsers :many
SELECT * FROM group_view;

-- name: GetAllGroups :many
SELECT * FROM groups;

-- name: GetGroupByUUIDWithUsers :one
SELECT * FROM group_view WHERE uuid = $1;

-- name: GetGroupByUUID :one
SELECT * FROM groups WHERE uuid = $1;

-- name: DeleteGroupByUUID :exec
DELETE FROM groups WHERE uuid = $1;

-- name: SearchGroup :many
WITH filtered AS (
    SELECT *
    FROM group_view
    WHERE lower(name) LIKE '%' || lower($1::text) || '%'
       OR lower(description) LIKE '%' || lower($1::text) || '%'
),
total AS (
    SELECT COUNT(*) AS total_count
    FROM filtered
),
paged AS (
    SELECT *
    FROM filtered
    LIMIT $2 OFFSET $3
),
page_count AS (
    SELECT COUNT(*) AS page_count
    FROM paged
)
SELECT
    p.*,
    pc.page_count,
    t.total_count
FROM paged p, page_count pc, total t;

-- name: GetGroupByName :one
SELECT * FROM groups WHERE name = $1;
