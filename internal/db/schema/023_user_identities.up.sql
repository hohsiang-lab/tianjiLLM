ALTER TABLE "UserTable"
    ADD COLUMN IF NOT EXISTS auth_version BIGINT NOT NULL DEFAULT 1;

CREATE TABLE IF NOT EXISTS "UserIdentityTable" (
    identity_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES "UserTable"(user_id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    provider_email TEXT,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    display_name TEXT,
    avatar_url TEXT,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_identities_user
    ON "UserIdentityTable" (user_id);

CREATE INDEX IF NOT EXISTS idx_user_identities_email
    ON "UserIdentityTable" (provider_email);
