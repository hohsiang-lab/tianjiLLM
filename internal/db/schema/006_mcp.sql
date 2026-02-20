-- 006_mcp.sql
-- MCP server configurations (API-managed)

CREATE TABLE IF NOT EXISTS "MCPServerTable" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    server_id TEXT UNIQUE NOT NULL,
    alias TEXT NOT NULL DEFAULT '',
    transport TEXT NOT NULL,
    url TEXT NOT NULL DEFAULT '',
    command TEXT NOT NULL DEFAULT '',
    args TEXT[] NOT NULL DEFAULT '{}',
    auth_type TEXT NOT NULL DEFAULT '',
    auth_token TEXT NOT NULL DEFAULT '',
    static_headers JSONB NOT NULL DEFAULT '{}',
    allowed_tools TEXT[] NOT NULL DEFAULT '{}',
    disallowed_tools TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mcp_server_server_id ON "MCPServerTable" (server_id);
