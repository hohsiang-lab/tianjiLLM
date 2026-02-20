-- name: CreateGuardrailConfig :one
INSERT INTO "GuardrailConfigTable" (guardrail_name, guardrail_type, config, failure_policy, enabled)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetGuardrailConfig :one
SELECT * FROM "GuardrailConfigTable" WHERE id = $1;

-- name: GetGuardrailConfigByName :one
SELECT * FROM "GuardrailConfigTable" WHERE guardrail_name = $1;

-- name: ListGuardrailConfigs :many
SELECT * FROM "GuardrailConfigTable" ORDER BY guardrail_name ASC;

-- name: UpdateGuardrailConfig :one
UPDATE "GuardrailConfigTable"
SET guardrail_name = COALESCE($2, guardrail_name),
    guardrail_type = COALESCE($3, guardrail_type),
    config = COALESCE($4, config),
    failure_policy = COALESCE($5, failure_policy),
    enabled = COALESCE($6, enabled),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteGuardrailConfig :exec
DELETE FROM "GuardrailConfigTable" WHERE id = $1;
