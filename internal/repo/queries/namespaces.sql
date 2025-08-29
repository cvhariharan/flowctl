-- name: CreateNamespace :one
INSERT INTO namespaces (name)
VALUES ($1)
RETURNING *;

-- name: GetNamespaceByUUID :one
SELECT * FROM namespaces WHERE uuid = $1;

-- name: ListNamespaces :many
WITH filtered AS (
    SELECT DISTINCT n.* FROM namespaces n
    LEFT JOIN namespace_members nm ON n.id = nm.namespace_id
    LEFT JOIN users u ON nm.user_id = u.id
    LEFT JOIN groups g ON nm.group_id = g.id
    LEFT JOIN group_memberships gm ON g.id = gm.group_id
    WHERE (
        (SELECT role FROM users WHERE users.uuid = $1) = 'superuser'
        OR u.uuid = $1
        OR gm.user_id = (SELECT id FROM users WHERE users.uuid = $1)
    ) AND lower(n.name) LIKE '%' || lower($4::text) || '%'
),
total AS (
    SELECT COUNT(*) AS total_count FROM filtered
),
paged AS (
    SELECT * FROM filtered
    LIMIT $2 OFFSET $3
),
page_count AS (
    SELECT CEIL(total.total_count::numeric / $2::numeric)::bigint AS page_count FROM total
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

-- name: AssignUserNamespaceRole :one
INSERT INTO namespace_members (user_id, namespace_id, role)
VALUES (
    (SELECT id FROM users WHERE users.uuid = $1),
    (SELECT id FROM namespaces WHERE namespaces.uuid = $2),
    $3
)
ON CONFLICT ON CONSTRAINT unique_user_namespace
DO UPDATE SET role = EXCLUDED.role, updated_at = NOW()
RETURNING *;

-- name: AssignGroupNamespaceRole :one
INSERT INTO namespace_members (group_id, namespace_id, role)
VALUES (
    (SELECT id FROM groups WHERE groups.uuid = $1),
    (SELECT id FROM namespaces WHERE namespaces.uuid = $2),
    $3
)
ON CONFLICT ON CONSTRAINT unique_group_namespace
DO UPDATE SET role = EXCLUDED.role, updated_at = NOW()
RETURNING *;

-- name: GetUserNamespacesWithRoles :many
WITH user_namespaces AS (
    -- Direct user membership
    SELECT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN users u ON nm.user_id = u.id
    WHERE u.uuid = $1

    UNION

    -- Group membership
    SELECT DISTINCT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN groups g ON nm.group_id = g.id
    JOIN group_memberships gm ON g.id = gm.group_id
    WHERE gm.user_id = (SELECT id FROM users WHERE users.uuid = $1)
)
SELECT * FROM user_namespaces
ORDER BY name;

-- name: GetNamespaceMembers :many
SELECT
    nm.uuid,
    COALESCE(u.uuid, g.uuid) as subject_uuid,
    COALESCE(u.name, g.name) as subject_name,
    CASE WHEN nm.user_id IS NOT NULL THEN 'user' ELSE 'group' END as subject_type,
    nm.role,
    nm.created_at,
    nm.updated_at
FROM namespace_members nm
LEFT JOIN users u ON nm.user_id = u.id
LEFT JOIN groups g ON nm.group_id = g.id
WHERE nm.namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $1)
ORDER BY nm.role, subject_name;

-- name: RemoveNamespaceMember :one
DELETE FROM namespace_members
WHERE namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $1)
AND namespace_members.uuid = $2
RETURNING *;

-- name: GetUserGroups :many
SELECT g.* FROM groups g
JOIN group_memberships gm ON g.id = gm.group_id
WHERE gm.user_id = (SELECT id FROM users WHERE users.uuid = $1);

-- name: GetAllNamespaces :many
SELECT * FROM namespaces ORDER BY name;

-- name: GetAllNamespaceMembers :many
SELECT
    nm.uuid,
    COALESCE(u.uuid, g.uuid) as subject_uuid,
    CASE WHEN nm.user_id IS NOT NULL THEN 'user' ELSE 'group' END as subject_type,
    nm.role,
    nm.namespace_id,
    n.uuid as namespace_uuid,
    n.name as namespace_name,
    nm.created_at,
    nm.updated_at
FROM namespace_members nm
JOIN namespaces n ON nm.namespace_id = n.id
LEFT JOIN users u ON nm.user_id = u.id
LEFT JOIN groups g ON nm.group_id = g.id
ORDER BY n.name, nm.role;
