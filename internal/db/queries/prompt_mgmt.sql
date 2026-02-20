-- name: CreatePromptTemplate :one
INSERT INTO "PromptTemplateTable" (name, version, template, variables, model, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetPromptTemplate :one
SELECT * FROM "PromptTemplateTable" WHERE id = $1;

-- name: GetLatestPromptByName :one
SELECT * FROM "PromptTemplateTable"
WHERE name = $1
ORDER BY version DESC
LIMIT 1;

-- name: GetPromptTemplateByNameVersion :one
SELECT * FROM "PromptTemplateTable"
WHERE name = $1 AND version = $2;

-- name: ListPromptTemplates :many
SELECT DISTINCT ON (name) *
FROM "PromptTemplateTable"
ORDER BY name, version DESC;

-- name: GetPromptVersions :many
SELECT * FROM "PromptTemplateTable"
WHERE name = $1
ORDER BY version DESC;

-- name: GetNextPromptVersion :one
SELECT COALESCE(MAX(version), 0) + 1 AS next_version
FROM "PromptTemplateTable"
WHERE name = $1;

-- name: DeletePromptTemplate :exec
DELETE FROM "PromptTemplateTable" WHERE id = $1;
