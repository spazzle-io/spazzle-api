-- name: UpsertUserStats :exec
INSERT INTO user_stats (
    user_id,
    total_games,
    total_score,
    total_pnl,
    total_volume
) VALUES (
   sqlc.arg(user_id),
   1,
   sqlc.arg(score),
   sqlc.arg(pnl),
   sqlc.arg(volume)
) ON CONFLICT (user_id) DO UPDATE
SET
    total_games = user_stats.total_games + 1,
    total_score = user_stats.total_score + sqlc.arg(score),
    total_pnl = user_stats.total_pnl + sqlc.arg(pnl),
    total_volume = user_stats.total_volume + sqlc.arg(volume),
    updated_at = now();

-- name: GetUserStats :one
SELECT
    user_id,
    total_games,
    total_score,
    total_pnl,
    total_volume,
    updated_at
FROM user_stats
WHERE user_id = sqlc.arg(user_id)
LIMIT 1;

-- name: GetGlobalLeaderboard :many
SELECT
    user_id,
    total_games,
    total_score,
    total_pnl,
    total_volume,
    updated_at
FROM user_stats
ORDER BY total_pnl DESC, total_score DESC, user_id DESC
LIMIT sqlc.arg(page_size)
OFFSET sqlc.arg(page_offset);

-- name: GetTotalUserStatsCount :one
SELECT COUNT(*) FROM user_stats;

-- name: GetGlobalLeaderboardByWindow :many
SELECT
    gp.user_id,
    COUNT(*)::int AS total_games,
    SUM(gp.score)::int AS total_score,
    SUM(gp.pnl)::numeric AS total_pnl,
    SUM(g.game_stake)::numeric AS total_volume
FROM game_players gp
    JOIN games g ON g.id = gp.game_id
WHERE g.ended_at > now() - sqlc.arg(time_window)::interval
GROUP BY gp.user_id
ORDER BY total_pnl DESC, total_score DESC, gp.user_id DESC
LIMIT sqlc.arg(page_size)
OFFSET sqlc.arg(page_offset);

-- name: GetGlobalLeaderboardByWindowCount :one
SELECT COUNT(DISTINCT gp.user_id)
FROM game_players gp
    JOIN games g ON g.id = gp.game_id
WHERE g.ended_at > now() - sqlc.arg(time_window)::interval;
