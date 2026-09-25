-- name: UpsertOAuthTokenMetadata :exec
INSERT INTO "OAuthTokenMetadata" (token_key, org_id, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (token_key) DO UPDATE SET
    org_id = EXCLUDED.org_id,
    updated_at = now();

-- name: GetOAuthTokenMetadata :one
SELECT token_key, org_id, disabled, updated_at
FROM "OAuthTokenMetadata"
WHERE token_key = $1;

-- name: GetAllOAuthTokenMetadata :many
SELECT token_key, org_id, disabled, updated_at
FROM "OAuthTokenMetadata";

-- name: DisableOAuthToken :exec
INSERT INTO "OAuthTokenMetadata" (token_key, org_id, disabled, updated_at)
VALUES ($1, '', true, now())
ON CONFLICT (token_key) DO UPDATE SET disabled = true, updated_at = now();

-- name: EnableOAuthToken :exec
INSERT INTO "OAuthTokenMetadata" (token_key, org_id, disabled, updated_at)
VALUES ($1, '', false, now())
ON CONFLICT (token_key) DO UPDATE SET disabled = false, updated_at = now();
