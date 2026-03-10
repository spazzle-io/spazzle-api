-- name: CreateGame :one
INSERT INTO games (
    id,
    server_id,
    num_rounds,
    num_players,
    total_pot,
    stake_per_game,
    started_at,
    ended_at
) VALUES (
   $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: GetGameById :one
SELECT
    id,
    server_id,
    num_rounds,
    num_players,
    total_pot,
    stake_per_game,
    started_at,
    ended_at,
    created_at
FROM games
WHERE id = sqlc.arg(game_id)
LIMIT 1;

-- name: ListServerGames :many
SELECT
    id,
    server_id,
    num_rounds,
    num_players,
    total_pot,
    stake_per_game,
    started_at,
    ended_at,
    created_at
FROM games
WHERE server_id = sqlc.arg(server_id)
AND (
    sqlc.narg(after_ended_at)::timestamptz IS NULL
    OR ended_at < sqlc.narg(after_ended_at)
    OR (ended_at = sqlc.narg(after_ended_at) AND id < sqlc.narg(after_id))
) ORDER BY ended_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: GetTotalServerGamesCount :one
SELECT COUNT(*)
FROM games
WHERE server_id = sqlc.arg(server_id);
