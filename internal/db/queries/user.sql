-- name: GetUser :one
SELECT * FROM "UserTable" WHERE user_id = $1;

-- name: ListUsers :many
SELECT * FROM "UserTable" ORDER BY created_at DESC;

-- name: CreateUser :one
INSERT INTO "UserTable" (user_id, user_alias, user_email, user_role, teams, max_budget, models, tpm_limit, rpm_limit, budget_duration, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: UpdateUser :one
UPDATE "UserTable"
SET user_alias = COALESCE($2, user_alias),
    user_email = COALESCE($3, user_email),
    user_role = COALESCE($4, user_role),
    max_budget = COALESCE($5, max_budget),
    models = COALESCE($6, models),
    tpm_limit = COALESCE($7, tpm_limit),
    rpm_limit = COALESCE($8, rpm_limit),
    updated_at = NOW(),
    updated_by = $9
WHERE user_id = $1
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM "UserTable" WHERE user_email = $1;

-- name: UpdateUserMetadata :exec
UPDATE "UserTable"
SET metadata = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: DeleteUser :exec
DELETE FROM "UserTable" WHERE user_id = $1;
