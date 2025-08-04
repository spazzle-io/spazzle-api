-- name: CreateCredential :one
INSERT INTO credentials (
    user_id, wallet_address
) VALUES (
    $1, $2
) RETURNING *;

-- name: GetCredentialByWalletAddress :one
SELECT
    id,
    user_id,
    wallet_address,
    role,
    created_at
FROM credentials
WHERE wallet_address = $1
LIMIT 1;
