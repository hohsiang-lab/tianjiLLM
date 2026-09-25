# Research: Claude Code OAuth Token Usage & Limits

## R-001: Data Source Strategy — Hybrid (Response Headers + Usage API Fallback)

**Decision**: Use Anthropic response headers as the **primary** data source, with the Usage API as a **cold-start fallback only**.

**Rationale**: The Usage API (`GET https://api.anthropic.com/api/oauth/usage`) has aggressive 429 rate limiting — ~5 requests per token per session before permanent block, with no `Retry-After` header. Per GitHub issues [#31021](https://github.com/anthropics/claude-code/issues/31021) and [#31637](https://github.com/anthropics/claude-code/issues/31637), this makes it unsuitable as a primary data source. Response headers are always available on every proxy response with no rate limiting.

**Primary path**: Extend existing `ParseAnthropicOAuthRateLimitHeaders()` to parse `7d_sonnet` headers and `anthropic-organization-id`. Data flows into a new `OAuthUsageStore` on every proxy response.

**Fallback path**: When store is empty (cold start, no proxy traffic yet) and user enters the Claude Code tab, attempt one Usage API call. On 429, show "No data yet" — data populates naturally once proxy traffic starts.

**Alternatives considered**:
- Usage API as primary source (ccstatusline approach): Rejected due to aggressive 429 rate limiting making it unreliable for server-side polling.
- Response headers only (no Usage API): Would leave cold-start with "—" placeholders until first proxy request. Usage API fallback provides better UX.
- Claude Code Analytics Admin API (`/v1/organizations/usage_report/claude_code`): Requires separate Admin API key, only provides per-user aggregates, not plan limits.

**Sources**:
- [sirmalloc/ccstatusline](https://github.com/sirmalloc/ccstatusline/blob/main/src/utils/usage-fetch.ts) — Usage API client reference
- [GitHub #31637](https://github.com/anthropics/claude-code/issues/31637) — 429 rate limiting documentation
- HTTP Toolkit traffic capture — response header schema verification

## R-002: API Response Schema

**Decision**: Parse the following response structure:

```json
{
  "five_hour": {
    "utilization": 0.06,
    "resets_at": "2026-03-26T05:37:00Z"
  },
  "seven_day": {
    "utilization": 0.44,
    "resets_at": "2026-03-28T22:00:00Z"
  },
  "extra_usage": {
    "is_enabled": true,
    "monthly_limit": 100.0,
    "used_credits": 6.20,
    "utilization": 0.062
  }
}
```

**Rationale**: Confirmed by ccstatusline's Zod schema and HTTP Toolkit traffic inspection. Fields are nullable/optional — free tier tokens may lack `extra_usage`.

## R-003: Authentication

**Decision**: Use `Authorization: Bearer <oauth-token>` with `anthropic-beta: oauth-2025-04-20` header.

**Rationale**: Same auth as Anthropic's native API. OAuth tokens from proxy config (`TianjiParams.APIKey`) work directly.

## R-004: Caching & Rate Limiting

**Decision**: In-memory cache with 180s TTL and 30s per-token rate-limit lock.

**Rationale**: Matches ccstatusline's battle-tested defaults. 180s is appropriate for 5-hour and 7-day utilization windows. 30s lock prevents API hammering during burst proxy traffic.

## R-005: Organization ID Persistence

**Decision**: New DB table `oauth_token_metadata` with columns `(token_key TEXT PRIMARY KEY, org_id TEXT, updated_at TIMESTAMPTZ)`. Migration file `014_oauth_token_metadata.up.sql`.

**Rationale**: Org ID comes from proxy response headers (`anthropic-organization-id`), not the Usage API. Must survive restarts. Simple key-value store — no complex schema needed. Latest existing migration is 013.

**Alternatives considered**:
- In-memory only: Lost on restart, requires proxy traffic to re-populate.
- Config file on disk: Adds file I/O complexity, TianjiLLM already has DB.

## R-006: Go HTTP Client for Usage API

**Decision**: Use `net/http` stdlib with 5-second timeout.

**Rationale**: No need for external HTTP client library. Single GET request with simple JSON response. stdlib is sufficient and avoids dependencies.
