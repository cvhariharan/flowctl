-- name: CreateNamespace :one
INSERT INTO namespaces (name)
VALUES ($1)
RETURNING *;

-- name: GetNamespaceByUUID :one
SELECT * FROM namespaces WHERE uuid = $1;

-- name: ListNamespaces :many
WITH filtered AS (
    SELECT DISTINCT n.* FROM namespaces n
    LEFT JOIN group_namespace_access gna ON n.id = gna.namespace_id
    LEFT JOIN group_memberships gm ON gna.group_id = gm.group_id
    LEFT JOIN users u ON gm.user_id = u.id
    WHERE (
        (SELECT role FROM users WHERE users.uuid = $1) = 'superuser'
        OR (u.uuid = $1 AND gna.namespace_id IS NOT NULL)
    )
),
total AS (
    SELECT COUNT(*) AS total_count FROM filtered
),
paged AS (
    SELECT * FROM filtered
    LIMIT $2 OFFSET $3
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



-- name: AssignUserNamespaceRole :one
INSERT INTO namespace_members (subject_uuid, subject_type, namespace_id, role)
VALUES (
    $1,
    'user',
    (SELECT id FROM namespaces WHERE namespaces.uuid = $2),
    $3
)
ON CONFLICT ON CONSTRAINT unique_namespace_member
DO UPDATE SET role = EXCLUDED.role, updated_at = NOW()
RETURNING *;

-- name: AssignGroupNamespaceRole :one
INSERT INTO namespace_members (subject_uuid, subject_type, namespace_id, role)
VALUES (
    $1,
    'group',
    (SELECT id FROM namespaces WHERE namespaces.uuid = $2),
    $3
)
ON CONFLICT ON CONSTRAINT unique_namespace_member
DO UPDATE SET role = EXCLUDED.role, updated_at = NOW()
RETURNING *;

-- name: GetUserNamespacesWithRoles :many
WITH user_namespaces AS (
    -- Direct user membership
    SELECT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    WHERE nm.subject_uuid = (SELECT uuid FROM users WHERE users.uuid = $1)
    AND nm.subject_type = 'user'

    UNION

    -- Group membership
    SELECT DISTINCT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN groups gr ON nm.subject_uuid = gr.uuid
    JOIN group_memberships gm ON gr.id = gm.group_id
    WHERE gm.user_id = (SELECT id FROM users WHERE users.uuid = $1)
    AND nm.subject_type = 'group'
)
SELECT * FROM user_namespaces
ORDER BY name;

-- name: GetNamespaceMembers :many
SELECT
    nm.uuid,
    CASE WHEN nm.subject_type = 'user' THEN u.uuid ELSE g.uuid END as subject_uuid,
    CASE WHEN nm.subject_type = 'user' THEN u.name ELSE g.name END as subject_name,
    nm.subject_type,
    nm.role,
    nm.created_at,
    nm.updated_at
FROM namespace_members nm
LEFT JOIN users u ON nm.subject_uuid = u.uuid AND nm.subject_type = 'user'
LEFT JOIN groups g ON nm.subject_uuid = g.uuid AND nm.subject_type = 'group'
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
