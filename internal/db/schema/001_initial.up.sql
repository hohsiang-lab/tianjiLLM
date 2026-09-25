-- 001_initial.sql
-- Core tables for P1: VerificationToken (API keys), ProxyModelTable, BudgetTable

CREATE TABLE IF NOT EXISTS "BudgetTable" (
    budget_id TEXT PRIMARY KEY,
    max_budget DOUBLE PRECISION,
    soft_budget DOUBLE PRECISION,
    max_parallel_requests INTEGER,
    tpm_limit BIGINT,
    rpm_limit BIGINT,
    model_max_budget JSONB,
    budget_duration TEXT,
    budget_reset_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS "VerificationToken" (
    token TEXT PRIMARY KEY,
    key_name TEXT,
    key_alias TEXT,
    spend DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    max_budget DOUBLE PRECISION,
    expires TIMESTAMPTZ,
    models TEXT[] NOT NULL DEFAULT '{}',
    aliases JSONB NOT NULL DEFAULT '{}',
    config JSONB NOT NULL DEFAULT '{}',
    user_id TEXT,
    team_id TEXT,
    organization_id TEXT,
    permissions JSONB NOT NULL DEFAULT '{}',
    metadata JSONB NOT NULL DEFAULT '{}',
    blocked BOOLEAN,
    tpm_limit BIGINT,
    rpm_limit BIGINT,
    budget_duration TEXT,
    budget_reset_at TIMESTAMPTZ,
    allowed_cache_controls TEXT[] NOT NULL DEFAULT '{}',
    allowed_routes TEXT[] NOT NULL DEFAULT '{}',
    policies TEXT[] NOT NULL DEFAULT '{}',
    access_group_ids TEXT[] NOT NULL DEFAULT '{}',
    model_spend JSONB NOT NULL DEFAULT '{}',
    model_max_budget JSONB NOT NULL DEFAULT '{}',
    soft_budget_cooldown BOOLEAN NOT NULL DEFAULT FALSE,
    budget_id TEXT REFERENCES "BudgetTable"(budget_id),
    object_permission_id TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by TEXT,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    updated_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_verification_token_user_team ON "VerificationToken" (user_id, team_id);
CREATE INDEX IF NOT EXISTS idx_verification_token_team ON "VerificationToken" (team_id);
CREATE INDEX IF NOT EXISTS idx_verification_token_budget_expires ON "VerificationToken" (budget_reset_at, expires);

CREATE TABLE IF NOT EXISTS "ProxyModelTable" (
    model_id TEXT PRIMARY KEY,
    model_name TEXT NOT NULL,
    tianji_params JSONB NOT NULL,
    model_info JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT NOT NULL DEFAULT ''
);
