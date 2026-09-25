-- 007_audit.sql
-- Audit logging + soft-delete tables for compliance tracking

CREATE TABLE IF NOT EXISTS "AuditLog" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    changed_by TEXT NOT NULL DEFAULT '',
    changed_by_api_key TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL DEFAULT '',
    table_name TEXT NOT NULL DEFAULT '',
    object_id TEXT NOT NULL DEFAULT '',
    before_value JSONB,
    updated_values JSONB
);

CREATE INDEX IF NOT EXISTS idx_audit_log_changed_by ON "AuditLog" (changed_by);
CREATE INDEX IF NOT EXISTS idx_audit_log_table_name ON "AuditLog" (table_name);
CREATE INDEX IF NOT EXISTS idx_audit_log_object_id ON "AuditLog" (object_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_updated_at ON "AuditLog" (updated_at);

-- Soft-delete table for verification tokens (API keys)
CREATE TABLE IF NOT EXISTS "DeletedVerificationToken" (
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
    budget_id TEXT,
    object_permission_id TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by TEXT,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    updated_by TEXT,
    deleted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_by TEXT NOT NULL DEFAULT '',
    deleted_by_api_key TEXT NOT NULL DEFAULT '',
    tianji_changed_by TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_deleted_vt_deleted_at ON "DeletedVerificationToken" (deleted_at);
CREATE INDEX IF NOT EXISTS idx_deleted_vt_user_id ON "DeletedVerificationToken" (user_id);
CREATE INDEX IF NOT EXISTS idx_deleted_vt_team_id ON "DeletedVerificationToken" (team_id);

-- Soft-delete table for teams
CREATE TABLE IF NOT EXISTS "DeletedTeamTable" (
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
    budget_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT NOT NULL DEFAULT '',
    deleted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_by TEXT NOT NULL DEFAULT '',
    deleted_by_api_key TEXT NOT NULL DEFAULT '',
    tianji_changed_by TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_deleted_team_deleted_at ON "DeletedTeamTable" (deleted_at);
CREATE INDEX IF NOT EXISTS idx_deleted_team_org ON "DeletedTeamTable" (organization_id);
