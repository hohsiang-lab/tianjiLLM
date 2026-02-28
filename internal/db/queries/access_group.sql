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

-- name: ListKeysByAccessGroup :many
SELECT token, key_name, key_alias FROM "VerificationToken"
WHERE sqlc.arg(group_id)::text = ANY(access_group_ids);

-- name: AddKeyToAccessGroup :exec
UPDATE "VerificationToken"
SET access_group_ids = array_append(access_group_ids, sqlc.arg(group_id)::text)
WHERE token = sqlc.arg(token)::text AND NOT (sqlc.arg(group_id)::text = ANY(access_group_ids));

-- name: RemoveKeyFromAccessGroup :exec
UPDATE "VerificationToken"
SET access_group_ids = array_remove(access_group_ids, sqlc.arg(group_id)::text)
WHERE token = sqlc.arg(token)::text;

-- name: RemoveAccessGroupFromAllKeys :exec
UPDATE "VerificationToken"
SET access_group_ids = array_remove(access_group_ids, sqlc.arg(group_id)::text)
WHERE sqlc.arg(group_id)::text = ANY(access_group_ids);

-- name: ListKeysNotInAccessGroup :many
SELECT token, key_name, key_alias FROM "VerificationToken"
WHERE NOT (sqlc.arg(group_id)::text = ANY(access_group_ids)) OR access_group_ids IS NULL;

-- name: ListAllKeySummaries :many
SELECT token, key_name, key_alias FROM "VerificationToken"
ORDER BY key_alias, key_name;
