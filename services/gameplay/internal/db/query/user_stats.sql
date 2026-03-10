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

-- name: GetGlobalLeaderboard :many
SELECT
    user_id,
    total_games,
    total_score,
    total_pnl,
    total_volume,
    updated_at
FROM user_stats
WHERE (
    sqlc.narg(after_total_pnl)::numeric IS NULL
    OR total_pnl < sqlc.narg(after_total_pnl)
    OR (total_pnl = sqlc.narg(after_total_pnl) AND user_id < sqlc.narg(after_id))
)
ORDER BY total_pnl DESC, user_id DESC
LIMIT sqlc.arg(page_size);

-- name: GetTotalUserStatsCount :one
SELECT COUNT(*) FROM user_stats;

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
