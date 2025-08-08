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
WITH filtered AS (
    SELECT *
    FROM user_view
    WHERE lower(name) LIKE '%' || lower($1::text) || '%'
       OR lower(username) LIKE '%' || lower($1::text) || '%'
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
    SELECT CEIL(total.total_count::numeric / $2::numeric)::bigint AS page_count
    FROM total
)
SELECT
    p.*,
    pc.page_count,
    t.total_count
FROM paged p, page_count pc, total t;

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

-- name: RemoveAllGroupsForUserByUUID :exec
WITH
user_lookup AS (
    SELECT id FROM users WHERE users.uuid = sqlc.arg(user_uuid)
)
DELETE FROM group_memberships WHERE user_id = ( SELECT id FROM user_lookup );

-- name: GetUsersByRole :many
SELECT * FROM users WHERE role = $1;
