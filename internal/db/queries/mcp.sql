-- name: CreateMCPServer :one
INSERT INTO "MCPServerTable" (server_id, alias, transport, url, command, args, auth_type, auth_token, static_headers, allowed_tools, disallowed_tools)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetMCPServer :one
SELECT * FROM "MCPServerTable" WHERE id = $1;

-- name: GetMCPServerByServerID :one
SELECT * FROM "MCPServerTable" WHERE server_id = $1;

-- name: ListMCPServers :many
SELECT * FROM "MCPServerTable" ORDER BY server_id ASC;

-- name: UpdateMCPServer :one
UPDATE "MCPServerTable"
SET alias = COALESCE($2, alias),
    transport = COALESCE($3, transport),
    url = COALESCE($4, url),
    command = COALESCE($5, command),
    args = COALESCE($6, args),
    auth_type = COALESCE($7, auth_type),
    auth_token = COALESCE($8, auth_token),
    static_headers = COALESCE($9, static_headers),
    allowed_tools = COALESCE($10, allowed_tools),
    disallowed_tools = COALESCE($11, disallowed_tools),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteMCPServer :exec
DELETE FROM "MCPServerTable" WHERE id = $1;
