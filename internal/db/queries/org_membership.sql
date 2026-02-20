-- name: AddOrgMember :one
INSERT INTO "OrganizationMembership" (user_id, organization_id, user_role, budget_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateOrgMember :one
UPDATE "OrganizationMembership"
SET user_role = COALESCE($3, user_role),
    budget_id = COALESCE($4, budget_id),
    updated_at = NOW()
WHERE user_id = $1 AND organization_id = $2
RETURNING *;

-- name: DeleteOrgMember :exec
DELETE FROM "OrganizationMembership"
WHERE user_id = $1 AND organization_id = $2;

-- name: ListOrgMembers :many
SELECT * FROM "OrganizationMembership"
WHERE organization_id = $1
ORDER BY created_at DESC;

-- name: GetOrgMember :one
SELECT * FROM "OrganizationMembership"
WHERE user_id = $1 AND organization_id = $2;
