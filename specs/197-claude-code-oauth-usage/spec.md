# Feature Specification: Claude Code OAuth Token Usage & Limits

**Feature Branch**: `197-claude-code-oauth-usage`
**Created**: 2026-03-26
**Status**: Draft
**Input**: New "Claude Code" tab in Usage page showing per-token plan usage limits for multiple OAuth tokens.

## Clarifications

### Session 2026-03-26

- Q: Org ID persistence across restarts? → A: Store in database (survives restarts, requires DB and migration).
- Q: UI placement for Claude Code usage? → A: New dedicated "Claude Code" tab in the Usage page, showing per-token plan usage limits as progress bars (matching Anthropic's Plan Usage Limits UI).
- Q: Per-user usage via Analytics API? → A: Not needed. Scope is limited to per-token plan usage limits only.
- Q: Data source strategy? → A: Hybrid — response headers as primary source (parsed on every proxy response), Usage API as cold-start fallback only (when store is empty and user enters tab). The Usage API (`/api/oauth/usage`) has aggressive 429 rate limiting (~5 requests per token before permanent block per session, no Retry-After header) per GitHub issues #31021 and #31637, so it cannot be used as the primary data path.
- Q: Polling interval? → A: No auto-polling. Client fetches once on entering the Claude Code tab. Server updates store passively from response headers on every proxy response. User can manually refresh.
- Q: Old widget? → A: Remove entirely. New Claude Code tab replaces it.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Per-Token Plan Usage Limits (Priority: P1)

As a TianjiLLM admin with multiple OAuth tokens, I want to see each token's plan usage limits (5h session, 7d all models, 7d sonnet-only, extra usage) displayed as progress bars — matching what Anthropic shows in their "Plan usage limits" UI — so I can monitor capacity across all my tokens at a glance.

**Why this priority**: This is the core value proposition. Admins currently see incomplete rate limit data (missing Sonnet-only limits) and "unknown" states when no proxy traffic has flowed.

**Independent Test**: Can be tested by making one proxied request per OAuth token, then navigating to Usage → Claude Code tab to see progress bars with real data. For cold start (no traffic yet), the on-demand Usage API fallback populates the cards.

**Acceptance Scenarios**:

1. **Given** TianjiLLM has proxied requests through 2 OAuth tokens, **When** the admin visits Usage → Claude Code tab, **Then** they see a card per token, each showing progress bars for "Current session" (5h), "Weekly — All models" (7d), and "Sonnet only" (7d_sonnet), with percentage and human-readable reset countdown.
2. **Given** proxy traffic has been flowing, **When** the admin visits the tab, **Then** the data is already in the store (populated from response headers) and loads instantly.
3. **Given** no proxy traffic has occurred (cold start), **When** the admin visits the tab, **Then** the backend attempts a one-time fetch from the Usage API, stores the result, and returns it. If the API returns 429, the card shows "No data yet — send a request to populate."

---

### User Story 2 - Passive Update from Response Headers (Priority: P1)

As a TianjiLLM operator, I want the usage store to be updated from Anthropic response headers every time a proxy response comes back, so the data stays fresh as long as there is proxy traffic — without any external API calls.

**Why this priority**: This is the primary and most reliable data update path. Response headers are always available on every proxy response and have no rate limiting.

**Independent Test**: Can be tested by sending a proxy request through TianjiLLM, then checking the store has fresh usage data including 5h, 7d, 7d_sonnet utilization and org ID.

**Acceptance Scenarios**:

1. **Given** a proxy request to Anthropic completes (any status: 200, 429, etc.), **When** the response is processed, **Then** the system parses `anthropic-ratelimit-unified-*` headers (including `7d_sonnet`) and `anthropic-organization-id`, storing the result in the usage store.
2. **Given** the response headers include all utilization fields, **When** the data is stored, **Then** the store contains session (5h), weekly (7d), and sonnet-only (7d_sonnet) utilization with reset timestamps.
3. **Given** the response does not include `anthropic-organization-id`, **When** the data is stored, **Then** the org ID field remains empty — no error.

---

### User Story 3 - Organization ID Capture & Persistence (Priority: P2)

As a TianjiLLM admin, I want each token card to show a meaningful identifier (organization ID) instead of a SHA256 hash prefix, so I can tell which token belongs to which org.

**Why this priority**: Nice-to-have identification. The feature works without it. Org ID comes from proxy response headers, persisted in DB to survive restarts.

**Independent Test**: Can be tested by making one proxied request through a token, then verifying the token card shows the org ID. After restart, the org ID is still displayed.

**Acceptance Scenarios**:

1. **Given** a proxied Anthropic response includes `anthropic-organization-id`, **When** the response is processed, **Then** the org ID is stored in the database associated with the token's cache key.
2. **Given** an org ID has been captured for a token, **When** the Claude Code tab displays its card, **Then** the card title shows the org ID instead of the hash prefix.
3. **Given** TianjiLLM restarts, **When** the Claude Code tab is loaded, **Then** org IDs from the database are displayed immediately (no proxy traffic needed).
4. **Given** no org ID has been captured yet, **When** the card is displayed, **Then** it falls back to showing the SHA256 hash prefix.

---

### Edge Cases

- What happens when the OAuth token is expired or revoked? Response headers won't contain rate limit data; card shows stale data or "—". Usage API fallback may return auth error — card shows error state.
- What happens when the Usage API returns 429 on cold-start fetch? Card shows "No data yet — send a request to populate" with no retry. Data populates naturally once proxy traffic starts.
- What happens when response headers are missing (non-Anthropic provider)? System only processes Anthropic responses — no impact on other providers.
- What happens when multiple tokens share the same org ID? Displayed as separate cards (different token, same org label).
- What happens when `7d_sonnet` headers are absent? The Sonnet-only progress bar shows "—" placeholder.
- What happens when TianjiLLM runs without a database? Org ID capture (P2) is skipped; usage data from headers (P1) works fully from in-memory store.

## Requirements *(mandatory)*

### Functional Requirements

**Response Header Parsing (primary data source)**:
- **FR-001**: System MUST parse `anthropic-ratelimit-unified-5h-status/utilization/reset`, `anthropic-ratelimit-unified-7d-status/utilization/reset`, and `anthropic-ratelimit-unified-7d_sonnet-status/utilization/reset` from every Anthropic proxy response and store the result in the usage store.
- **FR-002**: System MUST parse `anthropic-organization-id` from Anthropic proxy response headers and persist it in the database (FR-006).
- **FR-003**: Response header parsing MUST work on all response statuses (200, 429, etc.) — 429 responses carry the most important rate limit signals.

**Usage API Fallback (cold-start only)**:
- **FR-004**: When the client requests usage data and the store is empty for a token, the backend MUST attempt a single fetch from `GET https://api.anthropic.com/api/oauth/usage` as a fallback. On success, store the result. On 429 or error, return a "no data yet" state — do NOT retry automatically.
- **FR-005**: System MUST implement in-memory caching (180s TTL) for Usage API responses. The cache is also populated by response header parsing, so in practice the API is rarely called.

**Org ID Persistence**:
- **FR-006**: System MUST persist organization IDs in a database table, associated with the OAuth token's cache key. This data survives restarts.

**UI — Claude Code Tab**:
- **FR-007**: System MUST add a "Claude Code" tab to the existing Usage page.
- **FR-008**: The Claude Code tab MUST display one card per configured OAuth token, stacked vertically, each showing:
  - "Current session" (5h utilization) with human-readable reset countdown (e.g. "Resets in 4 hr 37 min")
  - "Weekly — All models" (7d utilization) with reset day/time (e.g. "Resets Fri 6:00 PM")
  - "Sonnet only" (7d_sonnet utilization) with reset day/time
  - Extra usage info when available (only populated via Usage API cold-start fallback; not available from response headers)
  - Percentage label and color-coded progress bar (green <60%, amber 60-80%, red >80%)
- **FR-009**: Each token card MUST display organization ID as card title when available, falling back to SHA256 hash prefix.
- **FR-010**: Each token card MUST show "Last updated" timestamp.
- **FR-011**: Error states MUST be displayed gracefully with appropriate indicators on the affected card, not as a full-page error.
- **FR-012**: Client MUST fetch data once on entering the Claude Code tab. No auto-polling. User can manually refresh via a refresh button.

**Migration — Remove Old Widget**:
- **FR-013**: The old "Anthropic OAuth Rate Limits" widget on the Usage/Spend tab MUST be removed. The new Claude Code tab fully replaces it.
- **FR-014**: All code related to the old widget (templ component `RateLimitWidget`, JS polling script, `/ui/api/rate-limit-state` endpoint, `buildRateLimitWidgetData()`) MUST be deleted. The passive response header parsing (`ParseAnthropicOAuthRateLimitHeaders`) and `InMemoryRateLimitStore` are retained for Discord alerts and upstream throttle logic.

### Key Entities

- **OAuth Usage State**: Per-token data primarily from response headers, optionally enriched by Usage API. Contains: `sessionUsage` (float 0-1), `sessionResetAt` (string), `weeklyUsage` (float 0-1), `weeklyResetAt` (string), `sonnetUsage` (float 0-1), `sonnetResetAt` (string), `extraUsageEnabled` (*bool), `extraUsageLimit`, `extraUsageUsed`, `extraUsageUtilization`, `overageStatus` (string), `overageDisabledReason` (string), `error` (string), `fetchedAt` (timestamp).
- **Organization ID**: UUID from `anthropic-organization-id` response header. Stored in DB per OAuth token cache key. Survives restarts.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All three progress bars (5h, 7d, 7d_sonnet) display correctly for each OAuth token after one proxied request.
- **SC-002**: When proxy traffic is flowing, usage data is kept up-to-date from response headers without any external API calls.
- **SC-003**: On cold start with no proxy traffic, visiting the Claude Code tab shows data within 5 seconds (via Usage API fallback) or a clear "no data yet" message if API is rate-limited.
- **SC-004**: Proxy response latency is completely unaffected by header parsing (synchronous, no external calls needed).
- **SC-005**: The old "Anthropic OAuth Rate Limits" widget is fully removed — no "unknown" states anywhere.

## Assumptions

- Response headers `anthropic-ratelimit-unified-5h-*`, `anthropic-ratelimit-unified-7d-*`, `anthropic-ratelimit-unified-7d_sonnet-*` are consistently present in Anthropic API responses for OAuth tokens. Confirmed via HTTP Toolkit.
- `anthropic-organization-id` header is present in all Anthropic responses for org-level OAuth tokens. Confirmed via HTTP Toolkit.
- The Usage API (`/api/oauth/usage`) has aggressive 429 rate limiting (~5 requests per token per session) and should only be used as a cold-start fallback, not as a primary data source. Per GitHub issues #31021 and #31637.
- The existing `ParseAnthropicOAuthRateLimitHeaders` function can be extended to parse the additional `7d_sonnet` headers and `organization-id` without breaking existing consumers.
