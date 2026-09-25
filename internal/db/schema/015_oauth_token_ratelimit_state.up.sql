CREATE TABLE IF NOT EXISTS "OAuthTokenRateLimitState" (
    token_key                    TEXT PRIMARY KEY,
    unified_status               TEXT NOT NULL DEFAULT '',
    unified_5h_status            TEXT NOT NULL DEFAULT '',
    unified_5h_utilization       DOUBLE PRECISION NOT NULL DEFAULT -1,
    unified_5h_reset             TEXT NOT NULL DEFAULT '',
    unified_7d_status            TEXT NOT NULL DEFAULT '',
    unified_7d_utilization       DOUBLE PRECISION NOT NULL DEFAULT -1,
    unified_7d_reset             TEXT NOT NULL DEFAULT '',
    unified_7d_sonnet_status     TEXT NOT NULL DEFAULT '',
    unified_7d_sonnet_utilization DOUBLE PRECISION NOT NULL DEFAULT -1,
    unified_7d_sonnet_reset      TEXT NOT NULL DEFAULT '',
    org_id                       TEXT NOT NULL DEFAULT '',
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_oauth_ratelimit_state_updated_at
    ON "OAuthTokenRateLimitState" (updated_at);
