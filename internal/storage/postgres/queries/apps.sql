-- name: InsertApp :exec
INSERT INTO apps (id, name, product, api_key_hash, api_key_prefix, created_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetApp :one
SELECT id, name, product, api_key_prefix, created_at, last_used_at, revoked_at, created_by FROM apps WHERE id = $1;

-- name: GetAppByHash :one
SELECT id, name, product, api_key_prefix, created_at, last_used_at, revoked_at, created_by FROM apps WHERE api_key_hash = $1;

-- name: ListApps :many
SELECT id, name, product, api_key_prefix, created_at, last_used_at, revoked_at, created_by FROM apps ORDER BY created_at DESC;

-- name: UpdateAppKeyHash :exec
UPDATE apps SET api_key_hash = $2, api_key_prefix = $3 WHERE id = $1;

-- name: RevokeApp :exec
UPDATE apps SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;
