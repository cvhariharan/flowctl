-- name: GetUserByUUID :one
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
    u.uuid = $1
GROUP BY
    u.id, u.uuid, u.username, u.password;

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

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (
    username,
    password,
    login_type,
    role
) VALUES (
    $1, $2, $3, $4
) RETURNING *;
