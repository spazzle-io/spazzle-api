-- name: CreateServer :one
INSERT INTO servers (
    name,
    owner_id,
    server_address,
    is_publicly_visible,
    stake_per_game,
    num_rounds_per_game,
    round_duration_secs,
    num_drawing_options
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: GetServerById :one
SELECT
    id,
    name,
    owner_id,
    num_admins,
    num_custom_words,
    is_publicly_visible,
    server_address,
    stake_per_game,
    num_rounds_per_game,
    round_duration_secs,
    num_drawing_options,
    total_games,
    total_volume,
    total_players,
    trending_score,
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
    is_publicly_visible,
    server_address,
    stake_per_game,
    num_rounds_per_game,
    round_duration_secs,
    num_drawing_options,
    total_games,
    total_volume,
    total_players,
    trending_score,
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
    s.is_publicly_visible,
    s.server_address,
    s.stake_per_game,
    s.num_rounds_per_game,
    s.round_duration_secs,
    s.num_drawing_options,
    s.total_games,
    s.total_volume,
    s.total_players,
    s.trending_score,
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
    is_publicly_visible,
    server_address,
    stake_per_game,
    num_rounds_per_game,
    round_duration_secs,
    num_drawing_options,
    total_games,
    total_volume,
    total_players,
    trending_score,
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
    is_publicly_visible = COALESCE(sqlc.narg(is_publicly_visible), is_publicly_visible),
    server_address = COALESCE(sqlc.narg(server_address), server_address),
    stake_per_game = COALESCE(sqlc.narg(stake_per_game), stake_per_game),
    num_rounds_per_game = COALESCE(sqlc.narg(num_rounds_per_game), num_rounds_per_game),
    round_duration_secs = COALESCE(sqlc.narg(round_duration_secs), round_duration_secs),
    num_drawing_options = COALESCE(sqlc.narg(num_drawing_options), num_drawing_options),
    trending_score = COALESCE(sqlc.narg(trending_score), trending_score),
    is_archived = COALESCE(sqlc.narg(is_archived), is_archived),
    archived_at = COALESCE(sqlc.narg(archived_at), archived_at)
WHERE
    id = sqlc.arg(server_id)
    RETURNING *;

-- name: UpdateServerGameStats :exec
UPDATE servers
SET
    total_games = total_games + 1,
    total_volume = total_volume + sqlc.arg(volume),
    total_players = total_players + sqlc.arg(num_players)
WHERE id = sqlc.arg(server_id);

-- name: RecomputeTrendingScores :exec
UPDATE servers s
SET trending_score = COALESCE(sub.score, 0)
FROM (
    SELECT
        g.server_id,
        COUNT(*)::float8 * 0.7 + COUNT(DISTINCT gp.user_id)::float8 * 0.3 AS score
    FROM games g
        JOIN game_players gp ON g.id = gp.game_id
    WHERE g.ended_at > now() - sqlc.arg(trending_window)::interval
    GROUP BY g.server_id
) sub
WHERE s.id = sub.server_id;

-- name: ResetTrendingScores :exec
UPDATE servers
SET trending_score = 0
WHERE trending_score > 0
AND id NOT IN (
    SELECT DISTINCT server_id
    FROM games
    WHERE ended_at > now() - sqlc.arg(trending_window)::interval
);

-- name: ListServersByTrending :many
SELECT
    id,
    name,
    owner_id,
    num_admins,
    num_custom_words,
    is_publicly_visible,
    server_address,
    stake_per_game,
    num_rounds_per_game,
    round_duration_secs,
    num_drawing_options,
    total_games,
    total_volume,
    total_players,
    trending_score,
    is_archived,
    archived_at,
    created_at
FROM servers
WHERE (
    sqlc.narg(after_trending_score)::float8 IS NULL
    OR trending_score < sqlc.narg(after_trending_score)
    OR (trending_score = sqlc.narg(after_trending_score) AND id < sqlc.narg(after_id))
) ORDER BY trending_score DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: ListServersByPopular :many
SELECT
    id,
    name,
    owner_id,
    num_admins,
    num_custom_words,
    is_publicly_visible,
    server_address,
    stake_per_game,
    num_rounds_per_game,
    round_duration_secs,
    num_drawing_options,
    total_games,
    total_volume,
    total_players,
    trending_score,
    is_archived,
    archived_at,
    created_at
FROM servers
WHERE (
    sqlc.narg(after_total_games)::int IS NULL
    OR total_games < sqlc.narg(after_total_games)
    OR (total_games = sqlc.narg(after_total_games) AND id < sqlc.narg(after_id))
) ORDER BY total_games DESC, id DESC
LIMIT sqlc.arg(page_size);
