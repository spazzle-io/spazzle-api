-- name: UpsertServerPlayerStats :exec
INSERT INTO server_player_stats (
    server_id,
    user_id,
    total_games,
    total_score,
    total_pnl,
    total_volume
) VALUES (
   sqlc.arg(server_id),
   sqlc.arg(user_id),
   1,
   sqlc.arg(score),
   sqlc.arg(pnl),
   sqlc.arg(volume)
) ON CONFLICT (server_id, user_id) DO UPDATE
SET
    total_games = server_player_stats.total_games + 1,
    total_score = server_player_stats.total_score + sqlc.arg(score),
    total_pnl = server_player_stats.total_pnl + sqlc.arg(pnl),
    total_volume = server_player_stats.total_volume + sqlc.arg(volume),
    updated_at = now();

-- name: GetServerLeaderboard :many
SELECT
    server_id,
    user_id,
    total_games,
    total_score,
    total_pnl,
    total_volume,
    updated_at
FROM server_player_stats
WHERE server_id = sqlc.arg(server_id)
AND (
    sqlc.narg(after_total_pnl)::numeric IS NULL
    OR total_pnl < sqlc.narg(after_total_pnl)
    OR (total_pnl = sqlc.narg(after_total_pnl) AND user_id < sqlc.narg(after_id))
)
ORDER BY total_pnl DESC, user_id DESC
LIMIT sqlc.arg(page_size);

-- name: GetTotalServerPlayerStatsCount :one
SELECT COUNT(*)
FROM server_player_stats
WHERE server_id = sqlc.arg(server_id);
