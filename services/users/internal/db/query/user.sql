-- name: CreateUser :one
INSERT INTO users (
    wallet_address
) VALUES (
    $1
) RETURNING *;

-- name: GetUserById :one
SELECT
    id, wallet_address, gamer_tag, created_at
FROM users
WHERE id = sqlc.arg(user_id)
LIMIT 1;

-- name: GetUserByWalletAddress :one
SELECT
    id, wallet_address, gamer_tag, created_at
FROM users
WHERE wallet_address = sqlc.arg(wallet_address)
    LIMIT 1;

-- name: GetTotalUserCount :one
SELECT COUNT(*) as total_users FROM users;

-- name: ListUsers :many
SELECT
    id, wallet_address, gamer_tag, created_at
FROM users
ORDER BY created_at DESC
LIMIT $1
OFFSET $2;

-- name: UpdateUser :one
UPDATE users
SET
    gamer_tag = COALESCE(sqlc.narg(gamer_tag), gamer_tag)
WHERE
    id = sqlc.arg(user_id)
RETURNING *;
