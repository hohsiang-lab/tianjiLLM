-- name: GetVerificationToken :one
SELECT * FROM "VerificationToken"
WHERE token = $1;

-- name: ListVerificationTokens :many
SELECT * FROM "VerificationToken"
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListVerificationTokensByTeam :many
SELECT * FROM "VerificationToken"
WHERE team_id = $1
ORDER BY created_at DESC;

-- name: CreateVerificationToken :one
INSERT INTO "VerificationToken" (
    token, key_name, key_alias, spend, max_budget, expires,
    models, user_id, team_id, organization_id,
    permissions, metadata, tpm_limit, rpm_limit,
    budget_duration, budget_id, created_by
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, $13, $14,
    $15, $16, $17
)
RETURNING *;

-- name: UpdateVerificationTokenSpend :exec
UPDATE "VerificationToken"
SET spend = spend + $2, updated_at = NOW()
WHERE token = $1;

-- name: BlockVerificationToken :exec
UPDATE "VerificationToken"
SET blocked = TRUE, updated_at = NOW()
WHERE token = $1;

-- name: UnblockVerificationToken :exec
UPDATE "VerificationToken"
SET blocked = FALSE, updated_at = NOW()
WHERE token = $1;

-- name: DeleteVerificationToken :exec
DELETE FROM "VerificationToken"
WHERE token = $1;

-- name: UpdateVerificationToken :one
UPDATE "VerificationToken"
SET
    key_name = COALESCE($2, key_name),
    key_alias = COALESCE($3, key_alias),
    max_budget = COALESCE($4, max_budget),
    models = COALESCE($5, models),
    metadata = COALESCE($6, metadata),
    tpm_limit = COALESCE($7, tpm_limit),
    rpm_limit = COALESCE($8, rpm_limit),
    budget_duration = COALESCE($9, budget_duration),
    updated_at = NOW()
WHERE token = $1
RETURNING *;

-- name: ResetBudgetForExpiredTokens :exec
UPDATE "VerificationToken"
SET spend = 0, budget_reset_at = NOW() + (budget_duration || ' seconds')::INTERVAL, updated_at = NOW()
WHERE budget_reset_at IS NOT NULL AND budget_reset_at <= NOW();

-- name: ResetVerificationTokenSpend :exec
UPDATE "VerificationToken"
SET spend = 0, updated_at = NOW()
WHERE token = $1;

-- name: RegenerateVerificationToken :one
UPDATE "VerificationToken"
SET token = $2, spend = 0, updated_at = NOW()
WHERE token = $1
RETURNING *;

-- name: BulkUpdateVerificationTokens :exec
UPDATE "VerificationToken"
SET max_budget = COALESCE($2, max_budget),
    tpm_limit = COALESCE($3, tpm_limit),
    rpm_limit = COALESCE($4, rpm_limit),
    updated_at = NOW()
WHERE token = ANY($1::text[]);

-- name: ListVerificationTokensByUser :many
SELECT * FROM "VerificationToken"
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: ListExpiredTokens :many
SELECT token, key_name, expires FROM "VerificationToken"
WHERE expires IS NOT NULL AND expires <= NOW()
ORDER BY expires
LIMIT 100;

-- name: ListVerificationTokensFiltered :many
SELECT * FROM "VerificationToken"
WHERE
  (sqlc.narg(filter_team_id)::text IS NULL OR team_id = sqlc.narg(filter_team_id)) AND
  (sqlc.narg(filter_key_alias)::text IS NULL OR key_alias = sqlc.narg(filter_key_alias)) AND
  (sqlc.narg(filter_user_id)::text IS NULL OR user_id = sqlc.narg(filter_user_id)) AND
  (sqlc.narg(filter_token)::text IS NULL OR token = sqlc.narg(filter_token))
ORDER BY created_at DESC
LIMIT sqlc.arg(query_limit) OFFSET sqlc.arg(query_offset);

-- name: CountVerificationTokensFiltered :one
SELECT COUNT(*) FROM "VerificationToken"
WHERE
  (sqlc.narg(filter_team_id)::text IS NULL OR team_id = sqlc.narg(filter_team_id)) AND
  (sqlc.narg(filter_key_alias)::text IS NULL OR key_alias = sqlc.narg(filter_key_alias)) AND
  (sqlc.narg(filter_user_id)::text IS NULL OR user_id = sqlc.narg(filter_user_id)) AND
  (sqlc.narg(filter_token)::text IS NULL OR token = sqlc.narg(filter_token));

-- name: GetVerificationTokenByAlias :one
SELECT token FROM "VerificationToken"
WHERE key_alias = sqlc.arg(alias) AND (sqlc.narg(filter_team_id)::text IS NULL OR team_id = sqlc.narg(filter_team_id))
LIMIT 1;

-- name: RegenerateVerificationTokenWithParams :one
UPDATE "VerificationToken"
SET
    token = sqlc.arg(new_token),
    spend = 0,
    max_budget = COALESCE(sqlc.narg(new_max_budget), max_budget),
    tpm_limit = COALESCE(sqlc.narg(new_tpm_limit), tpm_limit),
    rpm_limit = COALESCE(sqlc.narg(new_rpm_limit), rpm_limit),
    budget_duration = COALESCE(sqlc.narg(new_budget_duration), budget_duration),
    updated_at = NOW()
WHERE token = sqlc.arg(old_token)
RETURNING *;
