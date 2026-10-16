-- name: CreateAPIKey :one
INSERT INTO lists.api_keys (user_id, name, key_hash)
VALUES ($1, $2, $3)
RETURNING id, name, created_at;

-- name: ListAPIKeys :many
SELECT id, name, created_at, last_used_at FROM lists.api_keys
WHERE user_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: RevokeAPIKey :execrows
UPDATE lists.api_keys SET revoked_at = now()
WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL;

-- name: UserByKeyHash :one
UPDATE lists.api_keys SET last_used_at = now()
WHERE key_hash = $1 AND revoked_at IS NULL
RETURNING user_id;
