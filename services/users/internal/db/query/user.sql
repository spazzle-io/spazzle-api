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
FROM USERS
WHERE (
    sqlc.narg(after_created_at)::timestamptz IS NULL
    OR created_at < sqlc.narg(after_created_at)
    OR (created_at = sqlc.narg(after_created_at) AND id < sqlc.narg(after_id))
) ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: UpdateUser :one
UPDATE users
SET
    gamer_tag = COALESCE(sqlc.narg(gamer_tag), gamer_tag)
WHERE
    id = sqlc.arg(user_id)
RETURNING *;
