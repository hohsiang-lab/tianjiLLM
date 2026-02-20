-- 002_management.sql
-- Management tables: TeamTable, UserTable, SpendLogs, EndUserTable

CREATE TABLE IF NOT EXISTS "TeamTable" (
    team_id TEXT PRIMARY KEY,
    team_alias TEXT,
    organization_id TEXT,
    admins TEXT[] NOT NULL DEFAULT '{}',
    members TEXT[] NOT NULL DEFAULT '{}',
    members_with_roles JSONB NOT NULL DEFAULT '[]',
    metadata JSONB NOT NULL DEFAULT '{}',
    max_budget DOUBLE PRECISION,
    spend DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    models TEXT[] NOT NULL DEFAULT '{}',
    blocked BOOLEAN NOT NULL DEFAULT FALSE,
    tpm_limit BIGINT,
    rpm_limit BIGINT,
    budget_duration TEXT,
    budget_reset_at TIMESTAMPTZ,
    budget_id TEXT REFERENCES "BudgetTable"(budget_id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS "UserTable" (
    user_id TEXT PRIMARY KEY,
    user_alias TEXT,
    user_email TEXT,
    user_role TEXT NOT NULL DEFAULT 'internal_user',
    teams TEXT[] NOT NULL DEFAULT '{}',
    max_budget DOUBLE PRECISION,
    spend DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    models TEXT[] NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    tpm_limit BIGINT,
    rpm_limit BIGINT,
    budget_duration TEXT,
    budget_reset_at TIMESTAMPTZ,
    budget_id TEXT REFERENCES "BudgetTable"(budget_id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS "SpendLogs" (
    request_id TEXT PRIMARY KEY,
    call_type TEXT NOT NULL DEFAULT 'completion',
    api_key TEXT NOT NULL DEFAULT '',
    spend DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    starttime TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    endtime TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completionstartime TIMESTAMPTZ,
    model TEXT NOT NULL DEFAULT '',
    model_id TEXT NOT NULL DEFAULT '',
    model_group TEXT NOT NULL DEFAULT '',
    api_base TEXT NOT NULL DEFAULT '',
    "user" TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    cache_hit TEXT NOT NULL DEFAULT '',
    cache_key TEXT NOT NULL DEFAULT '',
    request_tags TEXT[] NOT NULL DEFAULT '{}',
    team_id TEXT,
    end_user TEXT,
    requester_ip_address TEXT
);

CREATE INDEX IF NOT EXISTS idx_spend_logs_starttime ON "SpendLogs" (starttime);
CREATE INDEX IF NOT EXISTS idx_spend_logs_api_key ON "SpendLogs" (api_key);
CREATE INDEX IF NOT EXISTS idx_spend_logs_team ON "SpendLogs" (team_id);
CREATE INDEX IF NOT EXISTS idx_spend_logs_user ON "SpendLogs" (user);
CREATE INDEX IF NOT EXISTS idx_spend_logs_tags ON "SpendLogs" USING gin (request_tags);

CREATE TABLE IF NOT EXISTS "EndUserTable" (
    user_id TEXT PRIMARY KEY,
    alias TEXT,
    spend DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    allowed_model_region TEXT,
    default_model TEXT,
    tianji_budget_table TEXT REFERENCES "BudgetTable"(budget_id),
    blocked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
