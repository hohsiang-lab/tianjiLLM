-- name: GetUserIdentityByProviderSubject :one
SELECT *
FROM "UserIdentityTable"
WHERE provider = $1 AND provider_user_id = $2;

-- name: ListUserIdentitiesByUser :many
SELECT *
FROM "UserIdentityTable"
WHERE user_id = $1
ORDER BY created_at ASC;

-- name: GetUserIdentity :one
SELECT *
FROM "UserIdentityTable"
WHERE identity_id = $1;

-- name: CountUserIdentitiesByUser :one
SELECT COUNT(*)
FROM "UserIdentityTable"
WHERE user_id = $1;

-- name: CreateUserIdentity :one
INSERT INTO "UserIdentityTable" (
    identity_id,
    user_id,
    provider,
    provider_user_id,
    provider_email,
    email_verified,
    display_name,
    avatar_url,
    last_login_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
RETURNING *;

-- name: UpdateUserIdentityLogin :one
UPDATE "UserIdentityTable"
SET provider_email = $3,
    email_verified = $4,
    display_name = $5,
    avatar_url = $6,
    last_login_at = NOW(),
    updated_at = NOW()
WHERE provider = $1 AND provider_user_id = $2
RETURNING *;

-- name: DeleteUserIdentity :exec
DELETE FROM "UserIdentityTable"
WHERE identity_id = $1;

-- name: DeleteUserIdentityForUser :execrows
DELETE FROM "UserIdentityTable"
WHERE identity_id = $1 AND user_id = $2;
