-- name: CreateCronSchedule :one
INSERT INTO cron_schedules (flow_id, cron, timezone)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetCronSchedulesByFlowID :many
SELECT * FROM cron_schedules
WHERE flow_id = $1
ORDER BY id;

-- name: DeleteSystemCronsByFlowID :exec
DELETE FROM cron_schedules
WHERE flow_id = $1 AND is_user_created = FALSE;

-- name: GetAllCronSchedules :many
SELECT cs.*, f.slug AS flow_slug, f.name AS flow_name, n.uuid AS namespace_uuid
FROM cron_schedules cs
JOIN flows f ON cs.flow_id = f.id
JOIN namespaces n ON f.namespace_id = n.id
WHERE f.is_active = TRUE
ORDER BY cs.flow_id, cs.id;

-- name: GetActiveCronSchedulesByFlowSlugs :many
SELECT f.slug AS flow_slug, cs.cron, cs.timezone
FROM cron_schedules cs
JOIN flows f ON cs.flow_id = f.id
JOIN namespaces n ON f.namespace_id = n.id
WHERE n.uuid = $1
  AND f.slug = ANY($2::text[])
  AND f.is_active = TRUE
  AND cs.is_active = TRUE;

-- name: CreateUserSchedule :one
INSERT INTO cron_schedules (flow_id, cron, timezone, inputs, created_by, is_user_created, is_active)
VALUES ($1, $2, $3, $4, (SELECT id FROM users WHERE users.uuid = $5), TRUE, TRUE)
RETURNING *;

-- SELECT
--     cs.*,
--     f.slug as flow_slug,
--     f.name as flow_name,
--     u.uuid as created_by_uuid,
--     u.name as created_by_name
-- FROM cron_schedules cs
-- JOIN flows f ON cs.flow_id = f.id
-- LEFT JOIN users u ON cs.created_by = u.id
-- WHERE cs.id = $1
--   AND f.namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $3)
--   AND (cs.created_by = (SELECT id FROM users WHERE users.uuid = $2) OR cs.is_user_created = FALSE);

-- name: GetUserScheduleByUUID :one
WITH user_namespaces AS (
    -- Direct user membership
    SELECT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN users u ON nm.user_id = u.id
    WHERE u.uuid = $2

    UNION

    -- Group membership
    SELECT DISTINCT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN groups g ON nm.group_id = g.id
    JOIN group_memberships gm ON g.id = gm.group_id
    WHERE gm.user_id = (SELECT id FROM users WHERE users.uuid = $2)
)
SELECT
    cs.*,
    f.slug as flow_slug,
    f.name as flow_name,
    u.uuid as created_by_uuid,
    u.name as created_by_name
FROM cron_schedules cs
JOIN flows f ON cs.flow_id = f.id
INNER JOIN users u ON cs.created_by = u.id
WHERE cs.uuid = $1
  AND f.namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $3)
  AND (cs.created_by = (SELECT id FROM users WHERE users.uuid = $2)
        OR EXISTS (SELECT id FROM users WHERE  users.uuid = $2 AND users.role='superuser')
        OR EXISTS (SELECT user_namespaces.uuid FROM user_namespaces WHERE user_namespaces.role='admin')
  );

-- name: ListSchedules :many
WITH user_namespaces AS (
    -- Direct user membership
    SELECT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN users u ON nm.user_id = u.id
    WHERE u.uuid = $2

    UNION

    -- Group membership
    SELECT DISTINCT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN groups g ON nm.group_id = g.id
    JOIN group_memberships gm ON g.id = gm.group_id
    WHERE gm.user_id = (SELECT id FROM users WHERE users.uuid = $2)
),
filtered AS (
    SELECT
        cs.*,
        f.slug as flow_slug,
        f.name as flow_name,
        u.uuid as created_by_uuid,
        u.name as created_by_name
    FROM cron_schedules cs
    JOIN flows f ON cs.flow_id = f.id
    INNER JOIN users u ON cs.created_by = u.id
    WHERE f.namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $3)
      -- AND (cs.created_by = (SELECT id FROM users WHERE users.uuid = $2)
      --       OR EXISTS (SELECT id FROM users WHERE  users.uuid = $2 AND users.role='superuser')
      --       OR EXISTS (SELECT user_namespaces.uuid FROM user_namespaces WHERE user_namespaces.role='admin')
      --       OR cs.is_user_created = FALSE
      -- )
      AND ($1 = 0 OR f.id = $1)
),
total AS (
    SELECT COUNT(*) AS total_count FROM filtered
),
paged AS (
    SELECT * FROM filtered
    ORDER BY created_at DESC
    LIMIT $4 OFFSET $5
),
page_count AS (
    SELECT CEIL(total.total_count::numeric / $4::numeric)::bigint AS page_count FROM total
)
SELECT
    p.*,
    pc.page_count,
    t.total_count
FROM paged p, page_count pc, total t;

-- UPDATE cron_schedules cs
-- SET
--     cron = $2,
--     timezone = $3,
--     inputs = $4,
--     is_active = $5,
--     updated_at = NOW()
-- FROM flows f
-- WHERE cs.id = $1
--   AND cs.flow_id = f.id
--   AND cs.is_user_created = TRUE
--   AND f.namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $7)
--   AND cs.created_by = (SELECT id FROM users WHERE users.uuid = $6)
-- RETURNING cs.*;

-- name: UpdateUserScheduleByUUID :one
WITH user_namespaces AS (
    -- Direct user membership
    SELECT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN users u ON nm.user_id = u.id
    WHERE u.uuid = $6

    UNION

    -- Group membership
    SELECT DISTINCT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN groups g ON nm.group_id = g.id
    JOIN group_memberships gm ON g.id = gm.group_id
    WHERE gm.user_id = (SELECT id FROM users WHERE users.uuid = $6)
)
UPDATE cron_schedules cs
SET
    cron = $2,
    timezone = $3,
    inputs = $4,
    is_active = $5,
    updated_at = NOW()
FROM flows f
WHERE cs.uuid = $1
  AND cs.flow_id = f.id
  AND cs.is_user_created = TRUE
  AND f.namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $7)
  AND (cs.created_by = (SELECT id FROM users WHERE users.uuid = $6)
        OR EXISTS (SELECT id FROM users WHERE  users.uuid = $6 AND users.role='superuser')
        OR EXISTS (SELECT user_namespaces.uuid FROM user_namespaces WHERE user_namespaces.role='admin')
  )
RETURNING cs.*;

-- DELETE FROM cron_schedules cs
-- USING flows f
-- WHERE cs.id = $1
--   AND cs.flow_id = f.id
--   AND cs.is_user_created = TRUE
--   AND f.namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $3)
--   AND cs.created_by = (SELECT id FROM users WHERE users.uuid = $2);

-- name: DeleteUserScheduleByUUID :execrows
WITH user_namespaces AS (
    -- Direct user membership
    SELECT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN users u ON nm.user_id = u.id
    WHERE u.uuid = $2

    UNION

    -- Group membership
    SELECT DISTINCT n.uuid, n.name, nm.role
    FROM namespaces n
    JOIN namespace_members nm ON n.id = nm.namespace_id
    JOIN groups g ON nm.group_id = g.id
    JOIN group_memberships gm ON g.id = gm.group_id
    WHERE gm.user_id = (SELECT id FROM users WHERE users.uuid = $2)
)
DELETE FROM cron_schedules cs
USING flows f
WHERE cs.uuid = $1
  AND cs.flow_id = f.id
  AND cs.is_user_created = TRUE
  AND f.namespace_id = (SELECT id FROM namespaces WHERE namespaces.uuid = $3)
  AND (cs.created_by = (SELECT id FROM users WHERE users.uuid = $2)
        OR EXISTS (SELECT id FROM users WHERE  users.uuid = $2 AND users.role='superuser')
        OR EXISTS (SELECT user_namespaces.uuid FROM user_namespaces WHERE user_namespaces.role='admin')
  );

-- name: DisableUserSchedulesForFlow :exec
UPDATE cron_schedules
SET is_active = FALSE, updated_at = NOW()
WHERE flow_id = $1 AND is_user_created = TRUE;

-- name: GetScheduleByFlowAndCron :one
SELECT * FROM cron_schedules
WHERE flow_id = $1
  AND cron = $2
  AND timezone = $3
  AND is_user_created = $4
  AND is_active = TRUE;
