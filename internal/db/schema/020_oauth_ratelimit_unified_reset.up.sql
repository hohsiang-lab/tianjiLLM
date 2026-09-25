ALTER TABLE "OAuthTokenRateLimitState"
    ADD COLUMN IF NOT EXISTS unified_reset TEXT NOT NULL DEFAULT '';
