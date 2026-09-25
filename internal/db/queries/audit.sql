-- name: InsertAuditLog :one
INSERT INTO "AuditLog" (
    changed_by, changed_by_api_key, action, table_name, object_id,
    before_value, updated_values
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAuditLog :one
SELECT * FROM "AuditLog"
WHERE id = $1;

-- name: ListAuditLogs :many
SELECT * FROM "AuditLog"
WHERE
    ($1::text = '' OR changed_by = $1) AND
    ($2::text = '' OR action = $2) AND
    ($3::text = '' OR table_name = $3) AND
    ($4::text = '' OR object_id = $4) AND
    ($5::timestamptz IS NULL OR updated_at >= $5) AND
    ($6::timestamptz IS NULL OR updated_at <= $6)
ORDER BY updated_at DESC
LIMIT $7 OFFSET $8;

-- name: InsertDeletedVerificationToken :exec
INSERT INTO "DeletedVerificationToken" (
    token, key_name, key_alias, spend, max_budget, expires,
    models, aliases, config, user_id, team_id, organization_id,
    permissions, metadata, blocked, tpm_limit, rpm_limit,
    budget_duration, budget_reset_at, allowed_cache_controls, allowed_routes,
    policies, access_group_ids, model_spend, model_max_budget,
    soft_budget_cooldown, budget_id, object_permission_id,
    created_at, created_by, updated_at, updated_by,
    deleted_by, deleted_by_api_key, tianji_changed_by
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12,
    $13, $14, $15, $16, $17,
    $18, $19, $20, $21,
    $22, $23, $24, $25,
    $26, $27, $28,
    $29, $30, $31, $32,
    $33, $34, $35
);

-- name: InsertDeletedTeam :exec
INSERT INTO "DeletedTeamTable" (
    team_id, team_alias, organization_id, admins, members,
    members_with_roles, metadata, max_budget, spend, models,
    blocked, tpm_limit, rpm_limit, budget_duration, budget_reset_at,
    budget_id, created_at, created_by, updated_at, updated_by,
    deleted_by, deleted_by_api_key, tianji_changed_by
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14, $15,
    $16, $17, $18, $19, $20,
    $21, $22, $23
);
