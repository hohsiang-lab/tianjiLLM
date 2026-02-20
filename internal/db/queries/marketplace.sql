-- name: CreatePlugin :one
INSERT INTO "ClaudeCodePluginTable" (
    name, version, description, manifest_json, files_json,
    enabled, source, source_url, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetPlugin :one
SELECT * FROM "ClaudeCodePluginTable"
WHERE name = $1;

-- name: ListPlugins :many
SELECT * FROM "ClaudeCodePluginTable"
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListEnabledPlugins :many
SELECT * FROM "ClaudeCodePluginTable"
WHERE enabled = TRUE
ORDER BY name ASC;

-- name: EnablePlugin :exec
UPDATE "ClaudeCodePluginTable"
SET enabled = TRUE, updated_at = NOW()
WHERE name = $1;

-- name: DisablePlugin :exec
UPDATE "ClaudeCodePluginTable"
SET enabled = FALSE, updated_at = NOW()
WHERE name = $1;

-- name: DeletePlugin :exec
DELETE FROM "ClaudeCodePluginTable"
WHERE name = $1;
