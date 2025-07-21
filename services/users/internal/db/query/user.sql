-- name: CreateUser :one
INSERT INTO users (
    wallet_address, gamer_tag
) VALUES (
    $1, $2
) RETURNING *;

-- name: GetUserById :one
SELECT * FROM users
WHERE id = sqlc.arg(user_id)
LIMIT 1;

-- name: GetTotalUserCount :one
SELECT COUNT(*) as total_users FROM users;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY created_at DESC
LIMIT $1
OFFSET $2;

-- name: UpdateUser :one
UPDATE users
SET
    gamer_tag = COALESCE(sqlc.narg(gamer_tag), gamer_tag),
    ens_name = COALESCE(sqlc.narg(ens_name), ens_name),
    ens_avatar_uri = COALESCE(sqlc.narg(ens_avatar_uri), ens_avatar_uri),
    ens_image_url = COALESCE(sqlc.narg(ens_image_url), ens_image_url),
    ens_last_resolved_at = COALESCE(sqlc.narg(ens_last_resolved_at), ens_last_resolved_at)
WHERE
    id = sqlc.arg(user_id)
RETURNING *;
