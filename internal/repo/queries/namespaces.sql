-- name: CreateNamespace :one
INSERT INTO namespaces (name)
VALUES ($1)
RETURNING *;

-- name: GetNamespaceByUUID :one
SELECT * FROM namespaces WHERE uuid = $1;

-- name: ListNamespaces :many
WITH filtered AS (
    SELECT DISTINCT n.* FROM namespaces n
    LEFT JOIN namespace_members nm_user ON n.id = nm_user.namespace_id AND nm_user.subject_type = 'user'
    LEFT JOIN namespace_members nm_group ON n.id = nm_group.namespace_id AND nm_group.subject_type = 'group'
    LEFT JOIN group_memberships gm ON nm_group.subject_uuid IN (
        SELECT g.uuid FROM groups g 
        JOIN group_memberships gm2 ON g.id = gm2.group_id 
        WHERE gm2.user_id = (SELECT id FROM users WHERE users.uuid = $1)
    )
    WHERE (
        (SELECT role FROM users WHERE users.uuid = $1) = 'superuser'
        OR nm_user.subject_uuid = $1
        OR nm_group.subject_uuid IN (
            SELECT g.uuid FROM groups g 
            JOIN group_memberships gm3 ON g.id = gm3.group_id 
            WHERE gm3.user_id = (SELECT id FROM users WHERE users.uuid = $1)
        )
        
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

-- name: GetAllNamespaceMembers :many
SELECT
    nm.uuid,
    nm.subject_uuid,
    nm.subject_type,
    nm.role,
    nm.namespace_id,
    n.uuid as namespace_uuid,
    n.name as namespace_name,
    nm.created_at,
    nm.updated_at
FROM namespace_members nm
JOIN namespaces n ON nm.namespace_id = n.id
ORDER BY n.name, nm.role;
