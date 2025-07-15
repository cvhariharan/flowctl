-- name: CreateNamespace :one
INSERT INTO namespaces (name)
VALUES ($1)
RETURNING *;

-- name: GetNamespaceByUUID :one
SELECT * FROM namespaces WHERE uuid = $1;

-- name: ListNamespaces :many
WITH filtered AS (
    SELECT * FROM namespaces
),
total AS (
    SELECT COUNT(*) AS total_count FROM filtered
),
paged AS (
    SELECT * FROM filtered
    LIMIT $1 OFFSET $2
),
page_count AS (
    SELECT COUNT(*) AS page_count FROM paged
)
SELECT
    p.*,
    pc.page_count,
    t.total_count
FROM paged p, page_count pc, total t;

-- name: UpdateNamespace :one
UPDATE namespaces
SET name = $2, updated_at = NOW()
WHERE uuid = $1
RETURNING *;

-- name: DeleteNamespace :exec
DELETE FROM namespaces WHERE uuid = $1;

-- name: GetNamespaceByName :one
SELECT * FROM namespaces WHERE name = $1;

-- name: GrantGroupNamespaceAccess :one
INSERT INTO group_namespace_access (group_id, namespace_id)
VALUES (
    (SELECT id FROM groups WHERE groups.uuid = $1),
    (SELECT id FROM namespaces WHERE namespaces.uuid = $2)
)
ON CONFLICT (group_id, namespace_id) DO NOTHING
RETURNING *;

-- name: RevokeGroupNamespaceAccess :exec
DELETE FROM group_namespace_access
WHERE group_id = (SELECT id FROM groups WHERE groups.uuid = $1)
AND namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $2);

-- name: GetGroupsWithNamespaceAccess :many
SELECT g.*, gna.created_at AS access_granted_at
FROM groups g
JOIN group_namespace_access gna ON g.id = gna.group_id
WHERE gna.namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $1);

-- name: GetNamespacesForGroup :many
SELECT n.*, gna.created_at AS access_granted_at
FROM namespaces n
JOIN group_namespace_access gna ON n.id = gna.namespace_id
WHERE gna.group_id = (SELECT id FROM groups WHERE groups.uuid = $1);

-- name: CheckUserNamespaceAccess :one
SELECT EXISTS (
    SELECT 1 FROM group_namespace_access gna
    JOIN group_memberships gm ON gna.group_id = gm.group_id
    WHERE gm.user_id = $1 AND gna.namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $2)
) AS has_access;
