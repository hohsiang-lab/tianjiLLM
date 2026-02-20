-- name: CreateAgent :one
INSERT INTO "AgentsTable" (
    agent_name, tianji_params, agent_card_params,
    agent_access_groups, created_by
) VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAgent :one
SELECT * FROM "AgentsTable"
WHERE agent_id = $1;

-- name: GetAgentByName :one
SELECT * FROM "AgentsTable"
WHERE agent_name = $1;

-- name: ListAgents :many
SELECT * FROM "AgentsTable"
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateAgent :one
UPDATE "AgentsTable"
SET
    agent_name = $2,
    tianji_params = $3,
    agent_card_params = $4,
    agent_access_groups = $5,
    updated_by = $6,
    updated_at = NOW()
WHERE agent_id = $1
RETURNING *;

-- name: PatchAgent :one
UPDATE "AgentsTable"
SET
    agent_name = COALESCE($2, agent_name),
    tianji_params = COALESCE($3, tianji_params),
    agent_card_params = COALESCE($4, agent_card_params),
    agent_access_groups = COALESCE($5, agent_access_groups),
    updated_by = COALESCE($6, updated_by),
    updated_at = NOW()
WHERE agent_id = $1
RETURNING *;

-- name: DeleteAgent :exec
DELETE FROM "AgentsTable"
WHERE agent_id = $1;

-- name: ListAgentsByAccessGroups :many
SELECT * FROM "AgentsTable"
WHERE agent_access_groups && $1::text[]
ORDER BY created_at DESC;
