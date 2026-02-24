-- 004_credentials.sql
-- Encrypted credential storage using NaCl SecretBox

CREATE TABLE IF NOT EXISTS "CredentialTable" (
    credential_id TEXT PRIMARY KEY,
    credential_name TEXT NOT NULL,
    credential_type TEXT NOT NULL DEFAULT 'api_key',
    credential_value TEXT NOT NULL,  -- NaCl SecretBox encrypted, base64url encoded
    credential_info JSONB NOT NULL DEFAULT '{}',
    organization_id TEXT REFERENCES "OrganizationTable"(organization_id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_credentials_org ON "CredentialTable" (organization_id);
