-- name: GetTeam :one
SELECT * FROM "TeamTable" WHERE team_id = $1;

-- name: ListTeams :many
SELECT * FROM "TeamTable" ORDER BY created_at DESC;

-- name: CreateTeam :one
INSERT INTO "TeamTable" (team_id, team_alias, organization_id, admins, members, max_budget, models, tpm_limit, rpm_limit, budget_duration, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: UpdateTeam :one
UPDATE "TeamTable"
SET team_alias = COALESCE($2, team_alias),
    max_budget = COALESCE($3, max_budget),
    models = COALESCE($4, models),
    blocked = COALESCE($5, blocked),
    tpm_limit = COALESCE($6, tpm_limit),
    rpm_limit = COALESCE($7, rpm_limit),
    updated_at = NOW(),
    updated_by = $8
WHERE team_id = $1
RETURNING *;

-- name: DeleteTeam :exec
DELETE FROM "TeamTable" WHERE team_id = $1;

-- name: AddTeamMember :exec
UPDATE "TeamTable"
SET members = array_append(members, $2), updated_at = NOW()
WHERE team_id = $1;

-- name: RemoveTeamMember :exec
UPDATE "TeamTable"
SET members = array_remove(members, $2), updated_at = NOW()
WHERE team_id = $1;

-- name: BlockTeam :exec
UPDATE "TeamTable"
SET blocked = TRUE, updated_at = NOW()
WHERE team_id = $1;

-- name: UnblockTeam :exec
UPDATE "TeamTable"
SET blocked = FALSE, updated_at = NOW()
WHERE team_id = $1;

-- name: AddTeamModel :exec
UPDATE "TeamTable"
SET models = array_append(models, $2), updated_at = NOW()
WHERE team_id = $1;

-- name: RemoveTeamModel :exec
UPDATE "TeamTable"
SET models = array_remove(models, $2), updated_at = NOW()
WHERE team_id = $1;

-- name: GetTeamByAlias :one
SELECT * FROM "TeamTable" WHERE team_alias = $1;

-- name: UpdateTeamMetadata :exec
UPDATE "TeamTable"
SET metadata = $2, updated_at = NOW()
WHERE team_id = $1;

-- name: UpdateTeamMemberRole :exec
UPDATE "TeamTable"
SET members_with_roles = $2, updated_at = NOW()
WHERE team_id = $1;

-- name: ListAvailableTeams :many
SELECT * FROM "TeamTable"
WHERE blocked = FALSE
ORDER BY created_at DESC;

-- name: GetTeamPermissions :one
SELECT metadata FROM "TeamTable"
WHERE team_id = $1;

-- name: SetTeamPermissions :exec
UPDATE "TeamTable"
SET metadata = jsonb_set(COALESCE(metadata, '{}'), '{permissions}', $2::jsonb), updated_at = NOW()
WHERE team_id = $1;

-- name: SetTeamCallback :exec
UPDATE "TeamTable"
SET metadata = jsonb_set(COALESCE(metadata, '{}'), '{callback_settings}', $2::jsonb), updated_at = NOW()
WHERE team_id = $1;

-- name: GetTeamCallback :one
SELECT metadata->'callback_settings' as callback_settings FROM "TeamTable"
WHERE team_id = $1;

-- name: ResetTeamSpend :exec
UPDATE "TeamTable"
SET spend = 0, updated_at = NOW()
WHERE team_id = $1;

-- name: ListTeamAliases :many
SELECT team_id, team_alias FROM "TeamTable"
WHERE team_id = ANY(sqlc.arg(team_ids)::text[]);

-- name: ListTeamsByOrganization :many
SELECT * FROM "TeamTable"
WHERE organization_id = $1
ORDER BY created_at DESC;
