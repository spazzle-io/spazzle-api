-- name: CreateTreasury :one
INSERT INTO server_treasuries (
    address,
    server_id,
    owner
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: GetTreasury :one
SELECT
    address,
    server_id,
    owner,
    status,
    tx_hash,
    block_number,
    gas_used,
    deployed_at,
    created_at,
    updated_at
FROM server_treasuries
WHERE
    address = sqlc.arg(address)
LIMIT 1;

-- name: MarkTreasuryDeploying :one
UPDATE server_treasuries
SET
    status = 'deploying',
    updated_at = now()
WHERE
    address = sqlc.arg(address)
RETURNING *;

-- name: MarkTreasuryDeployed :one
UPDATE server_treasuries
SET
    status = 'deployed',
    tx_hash = sqlc.arg(tx_hash),
    block_number = sqlc.arg(block_number),
    gas_used = sqlc.arg(gas_used),
    deployed_at = now(),
    updated_at = now()
WHERE
    address = sqlc.arg(address)
RETURNING *;

-- name: RecoverDeployedTreasury :one
UPDATE server_treasuries
SET
    status = 'deployed',
    updated_at = now()
WHERE
    address = sqlc.arg(address)
RETURNING *;

-- name: MarkTreasuryFailed :one
UPDATE server_treasuries
SET
    status = 'failed',
    updated_at = now()
WHERE
    address = sqlc.arg(address)
RETURNING *;
