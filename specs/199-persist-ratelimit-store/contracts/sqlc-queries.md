# Contract: sqlc Queries for OAuthTokenRateLimitState

**Feature**: 199-persist-ratelimit-store
**Date**: 2026-03-26

## Migration: 015_oauth_token_ratelimit_state.up.sql

```sql
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
```

## Queries: oauth_token_ratelimit_state.sql

```sql
-- name: UpsertOAuthTokenRateLimitState :exec
INSERT INTO "OAuthTokenRateLimitState" (
    token_key, unified_status,
    unified_5h_status, unified_5h_utilization, unified_5h_reset,
    unified_7d_status, unified_7d_utilization, unified_7d_reset,
    unified_7d_sonnet_status, unified_7d_sonnet_utilization, unified_7d_sonnet_reset,
    org_id, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
ON CONFLICT (token_key) DO UPDATE SET
    unified_status = EXCLUDED.unified_status,
    unified_5h_status = EXCLUDED.unified_5h_status,
    unified_5h_utilization = EXCLUDED.unified_5h_utilization,
    unified_5h_reset = EXCLUDED.unified_5h_reset,
    unified_7d_status = EXCLUDED.unified_7d_status,
    unified_7d_utilization = EXCLUDED.unified_7d_utilization,
    unified_7d_reset = EXCLUDED.unified_7d_reset,
    unified_7d_sonnet_status = EXCLUDED.unified_7d_sonnet_status,
    unified_7d_sonnet_utilization = EXCLUDED.unified_7d_sonnet_utilization,
    unified_7d_sonnet_reset = EXCLUDED.unified_7d_sonnet_reset,
    org_id = EXCLUDED.org_id,
    updated_at = now();

-- name: GetRecentOAuthTokenRateLimitStates :many
SELECT token_key, unified_status,
    unified_5h_status, unified_5h_utilization, unified_5h_reset,
    unified_7d_status, unified_7d_utilization, unified_7d_reset,
    unified_7d_sonnet_status, unified_7d_sonnet_utilization, unified_7d_sonnet_reset,
    org_id, updated_at
FROM "OAuthTokenRateLimitState"
WHERE updated_at > now() - $1::interval;
```

## Behavior Contract

1. `UpsertOAuthTokenRateLimitState` — called during periodic flush for each dirty token. Idempotent via ON CONFLICT.
2. `GetRecentOAuthTokenRateLimitStates` — called once at startup. `$1` = `'30 minutes'`. Returns only fresh records.
