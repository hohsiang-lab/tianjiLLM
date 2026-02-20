-- name: CreateOrganization :one
INSERT INTO "OrganizationTable" (organization_id, organization_alias, max_budget, models, created_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetOrganization :one
SELECT * FROM "OrganizationTable" WHERE organization_id = $1;

-- name: ListOrganizations :many
SELECT * FROM "OrganizationTable" ORDER BY created_at DESC;

-- name: UpdateOrganization :one
UPDATE "OrganizationTable"
SET organization_alias = COALESCE($2, organization_alias),
    max_budget = COALESCE($3, max_budget),
    updated_at = NOW()
WHERE organization_id = $1
RETURNING *;

-- name: DeleteOrganization :exec
DELETE FROM "OrganizationTable" WHERE organization_id = $1;
