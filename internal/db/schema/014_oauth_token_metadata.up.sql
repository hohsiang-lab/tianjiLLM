CREATE TABLE IF NOT EXISTS "OAuthTokenMetadata" (
    token_key TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
