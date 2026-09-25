-- 005_phase3.sql
-- Phase 3 tables: Policy Engine, Tags, EndUsers, GuardrailConfigs, PromptTemplates, SpendArchives, IPWhitelist

-- Policy Engine: policies table
CREATE TABLE IF NOT EXISTS "PolicyTable" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    name TEXT UNIQUE NOT NULL,
    parent_id TEXT REFERENCES "PolicyTable"(id),
    conditions JSONB,
    guardrails_add TEXT[] NOT NULL DEFAULT '{}',
    guardrails_remove TEXT[] NOT NULL DEFAULT '{}',
    pipeline JSONB,
    description TEXT,
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_policy_name ON "PolicyTable" (name);
CREATE INDEX IF NOT EXISTS idx_policy_parent ON "PolicyTable" (parent_id);

-- Policy Engine: policy_attachments table
CREATE TABLE IF NOT EXISTS "PolicyAttachmentTable" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    policy_name TEXT NOT NULL REFERENCES "PolicyTable"(name) ON DELETE CASCADE,
    scope TEXT,
    teams TEXT[] NOT NULL DEFAULT '{}',
    keys TEXT[] NOT NULL DEFAULT '{}',
    models TEXT[] NOT NULL DEFAULT '{}',
    tags TEXT[] NOT NULL DEFAULT '{}',
    created_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_policy_attachment_policy ON "PolicyAttachmentTable" (policy_name);

-- Tags for spend attribution
CREATE TABLE IF NOT EXISTS "TagTable" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    name TEXT UNIQUE NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- End users / customers
CREATE TABLE IF NOT EXISTS "EndUserTable2" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    end_user_id TEXT UNIQUE NOT NULL,
    alias TEXT,
    allowed_model_region TEXT,
    default_model TEXT,
    budget DOUBLE PRECISION,
    blocked BOOLEAN NOT NULL DEFAULT FALSE,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_end_user2_end_user_id ON "EndUserTable2" (end_user_id);

-- Guardrail configurations (API-managed)
CREATE TABLE IF NOT EXISTS "GuardrailConfigTable" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    guardrail_name TEXT UNIQUE NOT NULL,
    guardrail_type TEXT NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    failure_policy TEXT NOT NULL DEFAULT 'fail_closed',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_guardrail_config_name ON "GuardrailConfigTable" (guardrail_name);

-- Prompt templates with versioning
CREATE TABLE IF NOT EXISTS "PromptTemplateTable" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    name TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    template TEXT NOT NULL,
    variables TEXT[] NOT NULL DEFAULT '{}',
    model TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (name, version)
);

CREATE INDEX IF NOT EXISTS idx_prompt_template_name ON "PromptTemplateTable" (name);

-- Spend archives for cold storage tracking
CREATE TABLE IF NOT EXISTS "SpendArchiveTable" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    date_from DATE NOT NULL,
    date_to DATE NOT NULL,
    storage_type TEXT NOT NULL,
    storage_location TEXT NOT NULL,
    entry_count BIGINT NOT NULL,
    exported_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_spend_archive_dates ON "SpendArchiveTable" (date_from, date_to);

-- IP whitelist for access control
CREATE TABLE IF NOT EXISTS "IPWhitelistTable" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    ip_address TEXT NOT NULL,
    description TEXT,
    created_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ip_whitelist_address ON "IPWhitelistTable" (ip_address);
