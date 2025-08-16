-- name: CreateServer :one
INSERT INTO servers (
    name,
    owner_id,
    server_address,
    stake_per_game,
    num_rounds_per_game,
    round_duration_secs,
    num_drawing_options
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetServerById :one
SELECT
    id,
    name,
    owner_id,
    num_admins,
    num_custom_words,
    server_address,
    stake_per_game,
    num_rounds_per_game,
    round_duration_secs,
    num_drawing_options,
    is_archived,
    archived_at,
    created_at
FROM servers
WHERE id = sqlc.arg(server_id)
LIMIT 1;

-- name: GetServerByName :one
SELECT
    id,
    name,
    owner_id,
    num_admins,
    num_custom_words,
    server_address,
    stake_per_game,
    num_rounds_per_game,
    round_duration_secs,
    num_drawing_options,
    is_archived,
    archived_at,
    created_at
FROM servers
WHERE name = sqlc.arg(name)
LIMIT 1;

-- name: GetServerUserPermissions :one
SELECT
    s.owner_id = sqlc.arg(user_id) AS is_owner,
    (sa.user_id IS NOT NULL)::bool AS is_admin,
    (s.owner_id = sqlc.arg(user_id) OR (sa.user_id IS NOT NULL)::bool)::bool AS has_elevated_permissions
FROM servers s
    LEFT JOIN server_admins sa
    ON sa.server_id = s.id AND sa.user_id = sqlc.arg(user_id)
WHERE s.id = sqlc.arg(server_id);

-- name: GetTotalUserServersCount :one
SELECT COUNT(*)
FROM servers s
WHERE
    s.owner_id = sqlc.arg(user_id)
    OR EXISTS (
        SELECT 1
        FROM server_admins sa
        WHERE sa.server_id = s.id AND sa.user_id = sqlc.arg(user_id)
        LIMIT 1
    );

-- name: ListUserServers :many
SELECT
    s.id,
    s.name,
    s.owner_id,
    s.num_admins,
    s.num_custom_words,
    s.server_address,
    s.stake_per_game,
    s.num_rounds_per_game,
    s.round_duration_secs,
    s.num_drawing_options,
    s.is_archived,
    s.archived_at,
    s.created_at,
    (s.owner_id = sqlc.arg(user_id)) AS is_owner,
    (sa.user_id IS NOT NULL)::bool AS is_admin
FROM servers s
    LEFT JOIN LATERAL (
        SELECT sa.user_id
        FROM server_admins sa
        WHERE sa.server_id = s.id AND sa.user_id = sqlc.arg(user_id)
        LIMIT 1
    ) sa ON TRUE
WHERE (s.owner_id = sqlc.arg(user_id) OR sa.user_id = sqlc.arg(user_id))
AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL
    OR s.created_at < sqlc.narg(after_created_at)
    OR (s.created_at = sqlc.narg(after_created_at) AND s.id < sqlc.narg(after_id))
) ORDER BY s.created_at DESC, s.id DESC
LIMIT sqlc.arg(page_size);

-- name: GetTotalServerCount :one
SELECT COUNT(*) FROM servers;

-- name: ListServers :many
SELECT
    id,
    name,
    owner_id,
    num_admins,
    num_custom_words,
    server_address,
    stake_per_game,
    num_rounds_per_game,
    round_duration_secs,
    num_drawing_options,
    is_archived,
    archived_at,
    created_at
FROM servers
WHERE (
    sqlc.narg(after_created_at)::timestamptz IS NULL
    OR created_at < sqlc.narg(after_created_at)
    OR (created_at = sqlc.narg(after_created_at) AND id < sqlc.narg(after_id))
) ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: UpdateServer :one
UPDATE servers
SET
    name = COALESCE(sqlc.narg(name), name),
    owner_id = COALESCE(sqlc.narg(owner_id), owner_id),
    num_admins = COALESCE(sqlc.narg(num_admins), num_admins),
    num_custom_words = COALESCE(sqlc.narg(num_custom_words), num_custom_words),
    server_address = COALESCE(sqlc.narg(server_address), server_address),
    stake_per_game = COALESCE(sqlc.narg(stake_per_game), stake_per_game),
    num_rounds_per_game = COALESCE(sqlc.narg(num_rounds_per_game), num_rounds_per_game),
    round_duration_secs = COALESCE(sqlc.narg(round_duration_secs), round_duration_secs),
    num_drawing_options = COALESCE(sqlc.narg(num_drawing_options), num_drawing_options),
    is_archived = COALESCE(sqlc.narg(is_archived), is_archived),
    archived_at = COALESCE(sqlc.narg(archived_at), archived_at)
WHERE
    id = sqlc.arg(server_id)
    RETURNING *;
