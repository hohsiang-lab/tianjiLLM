-- name: CreateEndUser :one
INSERT INTO "EndUserTable2" (end_user_id, alias, allowed_model_region, default_model, budget, blocked, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetEndUser :one
SELECT * FROM "EndUserTable2" WHERE id = $1;

-- name: GetEndUserByExternalID :one
SELECT * FROM "EndUserTable2" WHERE end_user_id = $1;

-- name: ListEndUsers :many
SELECT * FROM "EndUserTable2" ORDER BY created_at DESC;

-- name: UpdateEndUser :one
UPDATE "EndUserTable2"
SET alias = COALESCE($2, alias),
    allowed_model_region = COALESCE($3, allowed_model_region),
    default_model = COALESCE($4, default_model),
    budget = COALESCE($5, budget),
    metadata = COALESCE($6, metadata),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteEndUser :exec
DELETE FROM "EndUserTable2" WHERE id = $1;

-- name: BlockEndUser :one
UPDATE "EndUserTable2" SET blocked = TRUE, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UnblockEndUser :one
UPDATE "EndUserTable2" SET blocked = FALSE, updated_at = NOW()
WHERE id = $1
RETURNING *;
