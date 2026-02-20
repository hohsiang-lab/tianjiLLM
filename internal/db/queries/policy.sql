-- name: CreatePolicy :one
INSERT INTO "PolicyTable" (name, parent_id, conditions, guardrails_add, guardrails_remove, pipeline, description, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetPolicy :one
SELECT * FROM "PolicyTable" WHERE id = $1;

-- name: GetPolicyByName :one
SELECT * FROM "PolicyTable" WHERE name = $1;

-- name: ListPolicies :many
SELECT * FROM "PolicyTable" ORDER BY created_at DESC;

-- name: UpdatePolicy :one
UPDATE "PolicyTable"
SET name = COALESCE($2, name),
    parent_id = $3,
    conditions = COALESCE($4, conditions),
    guardrails_add = COALESCE($5, guardrails_add),
    guardrails_remove = COALESCE($6, guardrails_remove),
    pipeline = $7,
    description = COALESCE($8, description),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeletePolicy :exec
DELETE FROM "PolicyTable" WHERE id = $1;

-- name: GetPolicyChain :many
WITH RECURSIVE chain AS (
    SELECT pt.*, 1 AS depth FROM "PolicyTable" pt WHERE pt.name = $1
    UNION ALL
    SELECT p.*, c.depth + 1 FROM "PolicyTable" p
    JOIN chain c ON p.id = c.parent_id
    WHERE c.depth < 50
)
SELECT id, name, parent_id, conditions, guardrails_add, guardrails_remove, pipeline, description, created_by, created_at, updated_at FROM chain;

-- name: CreatePolicyAttachment :one
INSERT INTO "PolicyAttachmentTable" (policy_name, scope, teams, keys, models, tags, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetPolicyAttachment :one
SELECT * FROM "PolicyAttachmentTable" WHERE id = $1;

-- name: ListPolicyAttachments :many
SELECT * FROM "PolicyAttachmentTable" ORDER BY created_at DESC;

-- name: ListPolicyAttachmentsByPolicy :many
SELECT * FROM "PolicyAttachmentTable" WHERE policy_name = $1;

-- name: DeletePolicyAttachment :exec
DELETE FROM "PolicyAttachmentTable" WHERE id = $1;
