-- name: CreateAccessGroup :one
INSERT INTO "ModelAccessGroup" (group_id, group_alias, models, organization_id, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAccessGroup :one
SELECT * FROM "ModelAccessGroup" WHERE group_id = $1;

-- name: ListAccessGroups :many
SELECT * FROM "ModelAccessGroup" ORDER BY created_at DESC;

-- name: UpdateAccessGroup :exec
UPDATE "ModelAccessGroup"
SET group_alias = $2, models = $3, updated_at = NOW()
WHERE group_id = $1;

-- name: DeleteAccessGroup :exec
DELETE FROM "ModelAccessGroup" WHERE group_id = $1;
