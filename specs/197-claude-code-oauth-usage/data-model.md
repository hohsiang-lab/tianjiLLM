# Data Model: Claude Code OAuth Token Usage & Limits

## New Entities

### OAuthUsageState (in-memory)

Per-token usage data fetched from Anthropic's OAuth Usage API. Stored in a new `OAuthUsageStore` (separate from existing `RateLimitStore`).

| Field | Type | Description |
|-------|------|-------------|
| TokenKey | string | SHA256[:12] cache key of the OAuth token |
| SessionUsage | float64 | 5h utilization [0,1]; -1 = unknown |
| SessionResetAt | string | ISO 8601 timestamp for 5h reset |
| WeeklyUsage | float64 | 7d utilization [0,1]; -1 = unknown |
| WeeklyResetAt | string | ISO 8601 timestamp for 7d reset |
| ExtraUsageEnabled | *bool | nil = not available |
| ExtraUsageLimit | *float64 | Monthly limit in USD |
| ExtraUsageUsed | *float64 | Used credits in USD |
| ExtraUsageUtilization | *float64 | [0,1] fraction |
| Error | string | Error type if fetch failed: "", "rate-limited", "api-error", "timeout" |
| FetchedAt | time.Time | When data was last fetched |
| LockedUntil | time.Time | Rate-limit lock expiry (zero = unlocked) |

### OAuthTokenMetadata (database)

Persisted metadata for OAuth tokens. Survives restarts.

| Column | Type | Constraint | Description |
|--------|------|------------|-------------|
| token_key | TEXT | PRIMARY KEY | SHA256[:12] cache key |
| org_id | TEXT | NOT NULL | Organization ID from response headers |
| updated_at | TIMESTAMPTZ | NOT NULL DEFAULT now() | Last update time |

### Migration

File: `internal/db/schema/014_oauth_token_metadata.up.sql` (number determined at implementation time)

```sql
CREATE TABLE IF NOT EXISTS "OAuthTokenMetadata" (
    token_key TEXT PRIMARY KEY,
    org_id TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### sqlc Queries

File: `internal/db/queries/oauth_token_metadata.sql`

```sql
-- name: UpsertOAuthTokenMetadata :exec
INSERT INTO "OAuthTokenMetadata" (token_key, org_id, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (token_key) DO UPDATE SET
    org_id = EXCLUDED.org_id,
    updated_at = now();

-- name: GetOAuthTokenMetadata :one
SELECT token_key, org_id, updated_at
FROM "OAuthTokenMetadata"
WHERE token_key = $1;

-- name: GetAllOAuthTokenMetadata :many
SELECT token_key, org_id, updated_at
FROM "OAuthTokenMetadata";
```

## Modified Entities

### AnthropicRateLimitWidgetData (DELETED)

The `pages.AnthropicRateLimitWidgetData` struct and `RateLimitWidget` templ component are removed. Replaced by new `ClaudeCodeUsageData` struct and `ClaudeCodeTab` templ component.

### RateLimitStore (RETAINED)

`callback.InMemoryRateLimitStore` is retained — it is still used by Discord alerter (`CheckAndAlertOAuth`). The UI rendering path (`buildRateLimitWidgetData`, `toWidgetData`, `handleRateLimitState`) is deleted.

## New Store

### OAuthUsageStore

New interface in `internal/callback/oauth_usage_store.go`:

```go
type OAuthUsageStore interface {
    Get(tokenKey string) (OAuthUsageState, bool)
    Set(tokenKey string, state OAuthUsageState)
    GetAll() map[string]OAuthUsageState
    IsLocked(tokenKey string) bool
    Lock(tokenKey string, until time.Time)
}
```

In-memory implementation with `sync.RWMutex`.

## Relationships

```
ProxyConfig.ModelList
  └─ ModelConfig.TianjiParams.APIKey (OAuth token)
       ├─ RateLimitCacheKey(token) → tokenKey
       ├─ OAuthUsageStore.Get(tokenKey) → OAuthUsageState (in-memory)
       └─ DB.GetOAuthTokenMetadata(tokenKey) → org_id (persistent)
```
