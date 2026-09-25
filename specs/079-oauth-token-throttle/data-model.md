# Data Model: OAuth Token 智能限流

**Branch**: `079-oauth-token-throttle` | **Date**: 2026-03-05

## Entities

### Existing (no changes)

#### AnthropicOAuthRateLimitState
**Location**: `internal/callback/ratelimit_store.go:16`

Already contains all fields needed for throttle decisions:
- `TokenKey` — sha256(token)[:12], cache key
- `Unified5hUtilization` — float64 [0,1], -1 = missing
- `Unified7dUtilization` — float64 [0,1], -1 = missing
- `UnifiedStatus` — "allowed" | "rate_limited" | "overage"
- `Unified5hReset` — unix timestamp string
- `Unified7dReset` — unix timestamp string
- `UnifiedReset` — unix timestamp string

#### RateLimitStore (interface)
**Location**: `internal/callback/ratelimit_store.go:108`

- `Get(key) → (state, bool)` — used by throttle to check each token
- `Set(key, state)` — called by ModifyResponse on every Anthropic response
- `GetAll()` — used by UI only
- `Prune(ttl)` — cleanup stale entries (5min TTL, 1min tick)

### New

#### allTokensThrottledError
**Location**: `internal/proxy/handler/native_upstream.go` (new)

```
Fields:
- resetAt: time.Time — nearest reset time across all throttled tokens

Methods:
- Error() string — implements error interface
```

**Lifecycle**: Created when `selectUpstreamWithThrottle` finds zero available tokens. Consumed by `nativeProxy` to write 429 + Retry-After response. Not persisted.

## State Transitions

```
Token State (per request cycle):

  [No Data]  ─── first response ──→  [Tracked]
                                         │
                                    ┌────┴────┐
                                    │         │
                              utilization  utilization
                               < 80%       >= 80%
                                    │         │
                                    ▼         ▼
                              [Available]  [Throttled]
                                    │         │
                                    │    next response
                                    │    utilization drops
                                    │         │
                                    ◄─────────┘
                                    │
                              status becomes
                              "rate_limited"
                                    │
                                    ▼
                              [Hard Limited]
                                    │
                              reset time passes
                              next response: status="allowed"
                                    │
                                    ▼
                              [Available]
```

## No Schema Changes

This feature operates entirely in-memory. No database migrations required.
