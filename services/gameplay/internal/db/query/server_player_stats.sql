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
ORDER BY total_pnl DESC, total_score DESC, user_id DESC
LIMIT sqlc.arg(page_size)
OFFSET sqlc.arg(page_offset);

-- name: GetTotalServerPlayerStatsCount :one
SELECT COUNT(*)
FROM server_player_stats
WHERE server_id = sqlc.arg(server_id);

-- name: GetServerLeaderboardByWindow :many
SELECT
    gp.user_id,
    COUNT(*)::int AS total_games,
    SUM(gp.score)::int AS total_score,
    SUM(gp.pnl)::numeric AS total_pnl,
    SUM(g.game_stake)::numeric AS total_volume
FROM game_players gp
    JOIN games g ON g.id = gp.game_id
WHERE g.server_id = sqlc.arg(server_id)
AND g.ended_at > now() - sqlc.arg(time_window)::interval
GROUP BY gp.user_id
ORDER BY total_pnl DESC, total_score DESC, gp.user_id DESC
LIMIT sqlc.arg(page_size)
OFFSET sqlc.arg(page_offset);

-- name: GetServerLeaderboardByWindowCount :one
SELECT COUNT(DISTINCT gp.user_id)
FROM game_players gp
    JOIN games g ON g.id = gp.game_id
WHERE g.server_id = sqlc.arg(server_id)
AND g.ended_at > now() - sqlc.arg(time_window)::interval;
