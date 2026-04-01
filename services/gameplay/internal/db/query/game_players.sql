-- name: InsertGamePlayers :copyfrom
INSERT INTO game_players (
    game_id,
    user_id,
    score,
    pnl,
    position,
    rounds_played,
    provisional_payout,
    total_stake_lost,
    is_evicted
) VALUES (
   $1, $2, $3, $4, $5, $6, $7, $8, $9
);

-- name: GetGameLeaderboard :many
SELECT
    game_id,
    user_id,
    score,
    pnl,
    position,
    rounds_played,
    provisional_payout,
    total_stake_lost,
    is_evicted
FROM game_players
WHERE game_id = sqlc.arg(game_id)
ORDER BY position ASC, score DESC, user_id DESC
LIMIT sqlc.arg(page_size)
OFFSET sqlc.arg(page_offset);

-- name: GetTotalGamePlayersCount :one
SELECT COUNT(*)
FROM game_players
WHERE game_id = sqlc.arg(game_id);

-- name: ListUserGames :many
SELECT
    gp.game_id,
    gp.user_id,
    gp.score,
    gp.pnl,
    gp.position,
    gp.rounds_played,
    gp.provisional_payout,
    gp.total_stake_lost,
    gp.is_evicted,
    g.server_id,
    g.num_rounds,
    g.num_players,
    g.total_pot,
    g.game_stake,
    g.started_at,
    g.ended_at
FROM game_players gp
    JOIN games g ON g.id = gp.game_id
WHERE gp.user_id = sqlc.arg(user_id)
AND (
    sqlc.narg(after_ended_at)::timestamptz IS NULL
    OR g.ended_at < sqlc.narg(after_ended_at)
    OR (g.ended_at = sqlc.narg(after_ended_at) AND g.id < sqlc.narg(after_id))
)
ORDER BY g.ended_at DESC, g.id DESC
LIMIT sqlc.arg(page_size);

-- name: GetTotalUserGamesCount :one
SELECT COUNT(*)
FROM game_players
WHERE user_id = sqlc.arg(user_id);
