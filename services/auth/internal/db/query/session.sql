-- name: CreateSession :one
INSERT INTO sessions (
    id,
    user_id,
    wallet_address,
    refresh_token,
    user_agent,
    client_ip,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetSessionById :one
SELECT * FROM sessions
WHERE id = $1 LIMIT 1;

-- name: RevokeAccountSessions :execresult
UPDATE sessions
SET is_revoked = true
WHERE user_id = $1 AND is_revoked = false;
