-- name: AddServerAdmin :one
INSERT INTO server_admins (
    server_id,
    user_id
) VALUES (
    $1, $2
) RETURNING *;

-- name: ListServerAdmins :many
SELECT
    server_id,
    user_id,
    added_at
FROM server_admins
WHERE server_id = sqlc.arg(server_id)
AND (
    sqlc.narg(after_added_at)::timestamptz IS NULL
    OR added_at < sqlc.narg(after_added_at)
    OR (added_at = sqlc.narg(after_added_at) AND user_id < sqlc.narg(after_user_id))
) ORDER BY added_at DESC, user_id DESC
LIMIT sqlc.arg(page_size);

-- name: RemoveServerAdmin :execresult
DELETE FROM server_admins
WHERE server_id = $1 AND user_id = $2;
