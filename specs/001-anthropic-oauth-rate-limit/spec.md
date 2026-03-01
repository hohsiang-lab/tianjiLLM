# Feature Specification: Usage Page — Anthropic Rate Limit Per OAuth Token

**Feature Branch**: `001-anthropic-oauth-rate-limit`  
**Created**: 2026-03-01  
**Status**: Draft  
**Linear**: HO-74

## User Scenarios & Testing *(mandatory)*

### User Story 1 — View Rate Limit Status Per Token (Priority: P1)

As an admin viewing the Usage page, I want to see the current Anthropic rate limit status for each OAuth token so I can identify which token is close to being throttled.

**Why this priority**: Core requirement of HO-74. Without per-token visibility, operators cannot manage multiple Anthropic OAuth tokens effectively.

**Independent Test**: Navigate to Usage page → confirm an "Anthropic Rate Limits" widget appears → confirm each configured OAuth token has its own row/card with input token quota, output token quota, and request quota displayed.

**Acceptance Scenarios**:

1. **Given** the system has 2 Anthropic OAuth tokens configured, **When** a user visits the Usage page, **Then** the widget shows 2 separate sections, each labelled with the token alias (or key hash prefix if no alias), showing input/output/requests limits and remaining values.
2. **Given** a token has recently made Anthropic API calls, **When** the page loads, **Then** the rate limit data reflects the latest parsed headers (not stale empty data).
3. **Given** a token's remaining capacity is below 20%, **When** displayed in the widget, **Then** the relevant quota value is highlighted in a warning colour.

---

### User Story 2 — Single-Token Degraded Compatibility (Priority: P2)

As an operator who only has one Anthropic OAuth token, I want the widget to still render correctly without breaking the existing Usage page layout.

**Why this priority**: Backward compatibility. Must not regress single-token deployments.

**Independent Test**: Configure exactly one token → visit Usage page → widget shows one section and page layout is unchanged.

**Acceptance Scenarios**:

1. **Given** the system has exactly 1 Anthropic OAuth token, **When** the widget renders, **Then** it displays a single token section (no multi-token header confusion).
2. **Given** no Anthropic OAuth tokens are configured, **When** the widget renders, **Then** it shows a graceful empty state (e.g., "No Anthropic tokens configured").

---

### User Story 3 — Rate Limit Data Auto-Refresh (Priority: P3)

As an admin, I want the rate limit widget to show reasonably fresh data without requiring a full page reload.

**Why this priority**: Nice-to-have; reduces operator need to manually refresh. Acceptable to defer to v2 if scope is tight.

**Independent Test**: Trigger an Anthropic API call → wait ≤ 30 s → observe widget update without full page reload.

**Acceptance Scenarios**:

1. **Given** the widget is loaded, **When** 30 seconds pass, **Then** the widget auto-refreshes via HTMX polling.
2. **Given** the API endpoint returns an error, **When** the widget polls, **Then** it shows the last known values rather than blanking out.

---

### Edge Cases

- Token with no recorded rate limit headers yet → show "No data" per quota field (displayed as "–").
- Token alias is empty → display truncated key hash (first 8 chars).
- `Remaining` header missing or sentinel (-1) → display "–" instead of a number.
- Multiple tokens with identical aliases → distinguish by appending key hash suffix.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST store rate limit state per token, keyed by `ratelimit:{token_key_hash}` (not a single global key).
- **FR-002**: The system MUST update the stored rate limit state whenever `ParseAnthropicRateLimitHeaders` is called after an Anthropic API response.
- **FR-003**: A new API endpoint MUST return a list of all tokens' latest rate limit states (alias, key hash, input/output/requests: limit, remaining, reset).
- **FR-004**: The Usage page MUST display an "Anthropic Rate Limits" widget that renders one card/section per OAuth token.
- **FR-005**: Each token section MUST show: token alias (or hash prefix if no alias), input token quota, output token quota, and request quota (limit, remaining, reset time).
- **FR-006**: Values of -1 (sentinel for missing/unparseable headers) MUST be displayed as "–" in the UI.
- **FR-007**: When remaining / limit ratio < 20% for any quota, the UI MUST visually highlight that quota (warning colour).
- **FR-008**: The widget MUST degrade gracefully when 0 tokens exist (show empty state message).
- **FR-009**: The widget SHOULD auto-refresh every 30 seconds via HTMX polling without full page reload.

### Key Entities

- **TokenRateLimitState**: Represents the latest parsed Anthropic rate limit headers for one OAuth token. Fields: `TokenKeyHash`, `TokenAlias`, `InputTokensLimit`, `InputTokensRemaining`, `InputTokensReset`, `OutputTokensLimit`, `OutputTokensRemaining`, `OutputTokensReset`, `RequestsLimit`, `RequestsRemaining`, `RequestsReset`, `UpdatedAt`.
- **RateLimitStore**: Key-value store keyed by `ratelimit:{token_key_hash}`; supports Set(state) and ListAll() operations.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Admins can identify which Anthropic token is rate-limited within 5 seconds of visiting the Usage page.
- **SC-002**: Widget correctly renders per-token data for deployments with 1–10 OAuth tokens without layout breakage.
- **SC-003**: Rate limit data displayed is never more than 60 seconds stale (with auto-refresh enabled).
- **SC-004**: Zero regression on existing Usage page tabs (Cost, Model Activity, Key Activity, Endpoint Activity).
- **SC-005**: When a token's quota drops below 20%, operators notice the visual warning without reading every number.

## Assumptions

- `ParseAnthropicRateLimitHeaders` in `internal/callback/discord_ratelimit.go` is the canonical parser; no new parsing logic needed.
- Token identity (`token_key_hash`) is available at the call site when `ParseAnthropicRateLimitHeaders` is invoked.
- Token aliases are retrievable from the existing token/key store by hash.
- In-memory store is acceptable for v1 (rate limit state is ephemeral; repopulates on next Anthropic API call after restart).
- HTMX is already in the frontend (consistent with existing Usage page pattern using `hx-get`, `hx-target`, etc.).
- The Usage page follows the Go templ pattern in `internal/ui/pages/usage.templ`.
