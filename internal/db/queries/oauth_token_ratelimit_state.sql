-- name: UpsertOAuthTokenRateLimitState :exec
INSERT INTO "OAuthTokenRateLimitState" (
    token_key, unified_status,
    unified_5h_status, unified_5h_utilization, unified_5h_reset,
    unified_7d_status, unified_7d_utilization, unified_7d_reset,
    unified_7d_sonnet_status, unified_7d_sonnet_utilization, unified_7d_sonnet_reset,
    org_id, unified_reset, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, now())
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
    unified_reset = EXCLUDED.unified_reset,
    updated_at = now();

-- name: GetActiveOAuthTokenRateLimitStates :many
-- Returns tokens whose 7d window has not yet reset (utilization data is still valid).
-- Used by PreloadRateLimitStore at startup. Tokens with expired 7d windows are omitted
-- because Anthropic has already zeroed their counters — the default (1, 0) score is correct.
SELECT token_key, unified_status,
    unified_5h_status, unified_5h_utilization, unified_5h_reset,
    unified_7d_status, unified_7d_utilization, unified_7d_reset,
    unified_7d_sonnet_status, unified_7d_sonnet_utilization, unified_7d_sonnet_reset,
    org_id, updated_at, unified_reset
FROM "OAuthTokenRateLimitState"
WHERE unified_7d_reset ~ '^\d+$' AND unified_7d_reset::bigint > extract(epoch from now())::bigint;

-- name: GetOAuthTokenRateLimitState :one
SELECT token_key, unified_status,
    unified_5h_status, unified_5h_utilization, unified_5h_reset,
    unified_7d_status, unified_7d_utilization, unified_7d_reset,
    unified_7d_sonnet_status, unified_7d_sonnet_utilization, unified_7d_sonnet_reset,
    org_id, updated_at, unified_reset
FROM "OAuthTokenRateLimitState"
WHERE token_key = $1;
