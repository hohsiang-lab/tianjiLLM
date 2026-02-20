-- 008_skills_agents.sql
-- A2A agents, skills, and Claude Code marketplace tables

CREATE TABLE IF NOT EXISTS "AgentsTable" (
    agent_id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    agent_name TEXT UNIQUE NOT NULL,
    tianji_params JSONB NOT NULL DEFAULT '{}',
    agent_card_params JSONB NOT NULL DEFAULT '{}',
    agent_access_groups TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_agents_name ON "AgentsTable" (agent_name);

CREATE TABLE IF NOT EXISTS "DailyAgentSpend" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    agent_id TEXT NOT NULL,
    date DATE NOT NULL DEFAULT CURRENT_DATE,
    api_key TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    model_group TEXT NOT NULL DEFAULT '',
    custom_llm_provider TEXT NOT NULL DEFAULT '',
    mcp_namespaced_tool_name TEXT NOT NULL DEFAULT '',
    endpoint TEXT NOT NULL DEFAULT '',
    prompt_tokens BIGINT NOT NULL DEFAULT 0,
    completion_tokens BIGINT NOT NULL DEFAULT 0,
    cache_read_input_tokens BIGINT NOT NULL DEFAULT 0,
    cache_creation_input_tokens BIGINT NOT NULL DEFAULT 0,
    spend DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    api_requests BIGINT NOT NULL DEFAULT 0,
    successful_requests BIGINT NOT NULL DEFAULT 0,
    failed_requests BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (agent_id, date, api_key, model, custom_llm_provider, mcp_namespaced_tool_name, endpoint)
);

CREATE INDEX IF NOT EXISTS idx_daily_agent_spend_agent ON "DailyAgentSpend" (agent_id);
CREATE INDEX IF NOT EXISTS idx_daily_agent_spend_date ON "DailyAgentSpend" (date);

CREATE TABLE IF NOT EXISTS "SkillsTable" (
    skill_id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    display_title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    instructions TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'custom',
    latest_version TEXT NOT NULL DEFAULT '',
    file_content BYTEA,
    file_name TEXT NOT NULL DEFAULT '',
    file_type TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS "ClaudeCodePluginTable" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    name TEXT UNIQUE NOT NULL,
    version TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    manifest_json JSONB NOT NULL DEFAULT '{}',
    files_json JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    source TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_claude_plugin_name ON "ClaudeCodePluginTable" (name);
CREATE INDEX IF NOT EXISTS idx_claude_plugin_enabled ON "ClaudeCodePluginTable" (enabled);
