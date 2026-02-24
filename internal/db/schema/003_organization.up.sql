-- 003_organization.sql
-- Organization management tables

CREATE TABLE IF NOT EXISTS "OrganizationTable" (
    organization_id TEXT PRIMARY KEY,
    organization_alias TEXT,
    budget_id TEXT REFERENCES "BudgetTable"(budget_id),
    metadata JSONB NOT NULL DEFAULT '{}',
    models TEXT[] NOT NULL DEFAULT '{}',
    spend DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    max_budget DOUBLE PRECISION,
    tpm_limit BIGINT,
    rpm_limit BIGINT,
    budget_duration TEXT,
    budget_reset_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT NOT NULL DEFAULT ''
);

-- Model Access Group table
CREATE TABLE IF NOT EXISTS "ModelAccessGroup" (
    group_id TEXT PRIMARY KEY,
    group_alias TEXT,
    models TEXT[] NOT NULL DEFAULT '{}',
    organization_id TEXT REFERENCES "OrganizationTable"(organization_id),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT NOT NULL DEFAULT ''
);

-- Add columns to SpendLogs
-- Note: Using plain ALTER TABLE (without IF NOT EXISTS) for sqlc compatibility.
-- The actual migration uses IF NOT EXISTS for idempotency; this is the sqlc schema view.
ALTER TABLE "SpendLogs" ADD COLUMN organization_id TEXT;
ALTER TABLE "SpendLogs" ADD COLUMN provider TEXT NOT NULL DEFAULT '';

-- Add organization_id index
CREATE INDEX IF NOT EXISTS idx_spend_logs_org ON "SpendLogs" (organization_id);
