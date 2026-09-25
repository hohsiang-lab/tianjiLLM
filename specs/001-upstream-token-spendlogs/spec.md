# Feature Specification: Record Upstream OAuth Token in SpendLogs

**Feature Branch**: `001-upstream-token-spendlogs`  
**Created**: 2026-04-02  
**Status**: Draft  
**Input**: User description: "Record upstream OAuth token in SpendLogs for request tracing (HO-483)"

## Problem Statement

A specific OAuth token (`b545c01499c2`) is showing 90% utilization in a 5-hour session window, but there is no way to determine **why** — which users, which virtual keys, which models, and how many requests are consuming this token's quota. SpendLogs currently only records the virtual key hash, not which upstream OAuth token fulfilled the request. Pod logs are ephemeral and useless for historical investigation.

The operator needs to answer: "Token X is at 90% — who is using it, what are they requesting, and how much is each consumer contributing?"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Investigate high token utilization (Priority: P1)

As a platform operator, when I see a token at 90% utilization on the rate limit dashboard, I need to drill down and see a breakdown of which virtual keys and users are consuming that token's quota, so I can take action (redistribute load, contact heavy users, or adjust throttle thresholds).

**Why this priority**: This is the direct motivation — without this, the operator sees the symptom (90% used) but cannot diagnose the cause. Every other story depends on this data being recorded first.

**Independent Test**: Can be verified by sending requests through the native proxy with different virtual keys, then querying SpendLogs to see per-key breakdown for a specific upstream token.

**Acceptance Scenarios**:

1. **Given** a request is routed through the Anthropic native proxy using OAuth token A, **When** the request succeeds (200), **Then** SpendLogs contains a record with the token's identifier in the upstream token field, alongside the existing virtual key hash, user ID, model, and token count. **When** the request fails (non-200), **Then** ErrorLogs contains a record with the same token identifier, model, end_user, and organization_id (per FR-009/FR-011).
2. **Given** a request is routed through a standard provider path (not native proxy), **When** the request completes, **Then** SpendLogs records an empty upstream token field (no false data).
3. **Given** multiple OAuth tokens are configured, **When** 10 requests from 3 different virtual keys are sent, **Then** each SpendLog entry reflects the actual upstream token used, enabling a per-key-per-token usage breakdown.

---

### User Story 2 - Show key alias in request log table (Priority: P2)

As a platform operator looking at the request log table, I need to see a human-readable key alias (e.g. "sean", "hoh lobster", "dev Claude Code") next to the key hash, so I can immediately identify which user/application is generating each request without memorizing hash prefixes.

**Why this priority**: The key hash alone (`7f655fa37831...`) is meaningless at a glance. The alias is already stored in VerificationToken but not displayed in the request log. This is essential for the investigation workflow — without it, even after filtering by upstream token, the operator still can't tell "who" at a glance.

**Independent Test**: Can be verified by viewing the request log table and confirming that each row with a known key hash also displays the corresponding alias from the keys page.

**Acceptance Scenarios**:

1. **Given** a request log entry has a key hash that matches a VerificationToken with alias "sean", **When** viewing the request log table, **Then** the "Key Alias" column displays "sean" for that row.
2. **Given** a request log entry has a key hash with no matching VerificationToken (e.g. deleted key or master key), **When** viewing the request log table, **Then** the "Key Alias" column shows "–" (dash).
3. **Given** a request log entry has no key hash (e.g. auth failure), **When** viewing the request log table, **Then** both Key Hash and Key Alias columns show "–".

---

### User Story 3 - Filter and aggregate spend by upstream token (Priority: P3)

As a platform operator, I want to filter the spend logs by a specific upstream token and see aggregated usage (total requests, total tokens consumed, top consumers) so I can answer "who is eating token X's quota?".

**Why this priority**: The raw data (P1) is necessary but not sufficient — operators need to filter and see patterns without writing SQL queries.

**Independent Test**: Can be verified by filtering the spend logs list by token `b545c01499c2` and confirming the results show only that token's requests, with the existing per-entry details (user, key alias, model, token count).

**Acceptance Scenarios**:

1. **Given** SpendLogs contain entries from multiple OAuth tokens, **When** filtering by a specific token identifier, **Then** only entries matching that token are displayed with all existing columns (user, key alias, model, tokens, cost).
2. **Given** SpendLogs contain entries with no upstream token (standard provider), **When** clearing the token filter, **Then** all entries are shown.
3. **Given** a token filter is active, **When** the operator sorts by token count descending, **Then** the heaviest-consuming requests appear first, revealing which users/keys drive the most utilization.

---

### User Story 4 - View upstream token in spend log detail (Priority: P4)

As a platform operator viewing a specific spend log entry in the admin UI, I need to see which upstream OAuth token was used so I can cross-reference with the rate limit dashboard.

**Why this priority**: Provides per-entry visibility. Lower priority because the filter view (P3) is more useful for investigation than looking at individual entries.

**Independent Test**: Can be verified by navigating to any spend log detail page for a native proxy request and confirming the upstream token identifier is displayed.

**Acceptance Scenarios**:

1. **Given** a spend log entry has an upstream token recorded, **When** viewing the log detail page, **Then** the upstream token identifier is displayed in the details section.
2. **Given** a spend log entry has no upstream token (standard provider path), **When** viewing the log detail page, **Then** the upstream token field is either hidden or shows "N/A".

---

### Edge Cases

- What happens when the native proxy selects a token but the upstream request fails before reaching Anthropic? The token identifier should still be recorded since the selection already happened — this request consumed scheduling capacity even if it didn't consume API quota.
- What happens when the rate limit store prunes a token's state? The SpendLogs entries are independent of the in-memory store and remain intact for historical investigation.
- What happens when a token is removed from config and old logs reference it? The identifier is a stable hash, so historical logs remain valid and queryable even if the token no longer exists in config.
- What happens when all requests come from a single virtual key? The breakdown still shows that one key is the sole consumer, which is itself a useful diagnostic finding.
- What happens when a virtual key is deleted but SpendLogs still reference its hash? The key alias column shows "–" since no VerificationToken record exists; the key hash remains visible for reference.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST record the upstream OAuth token identifier (SHA256 hash prefix, 12 chars) in every SpendLog entry created through the native proxy path.
- **FR-002**: System MUST record an empty token identifier for requests that do not go through the native proxy path (standard provider requests).
- **FR-003**: System MUST use the same token identifier format as the rate limit store to enable direct cross-referencing between the rate limit dashboard and spend logs.
- **FR-004**: The spend logs list MUST support filtering by upstream token identifier to enable per-token usage investigation.
- **FR-005**: The spend log detail page MUST display the upstream token identifier when present.
- **FR-006**: System MUST record the token identifier regardless of whether the upstream request succeeds or fails (the token selection is the relevant event, not the response).
- **FR-007**: Recording the upstream token MUST NOT degrade request latency (the token is already selected before the upstream call; this is a logging-path change only).
- **FR-008**: The request log table MUST display a "Key Alias" column showing the human-readable alias from the virtual key configuration, resolved by joining the key hash to the VerificationToken record. If no matching key exists (deleted key, master key), display "–".
- **FR-009**: ErrorLogs MUST be augmented with `upstream_token_key`, `end_user`, and `organization_id` columns to achieve parity with SpendLogs, enabling consistent filtering and investigation across success and failure entries.
- **FR-010**: The native proxy error path MUST record the model name (from the request body) in ErrorLogs instead of an empty string.
- **FR-011**: ErrorLogs entries for native proxy failures MUST include the upstream token identifier, so operators can correlate failed requests (e.g. 429) with specific tokens.

### Key Entities

- **SpendLog**: Existing entity tracking per-request cost and metadata (virtual key hash, user ID, team ID, model, token counts, cost). Gains a new attribute: upstream token identifier (text, may be empty for non-native-proxy requests).
- **ErrorLog**: Existing entity tracking per-request errors. Gains three new attributes: upstream token identifier, end_user, and organization_id — aligning with SpendLog for consistent cross-table querying.
- **Upstream Token Identifier**: A stable, short hash (12 chars) derived from the OAuth API key. Same format used by the rate limit store and displayed on the rate limit dashboard (e.g. `b545c01499c2`), enabling direct correlation.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of native proxy requests have a non-empty upstream token identifier in SpendLogs (success) and ErrorLogs (failure).
- **SC-002**: Operator can answer "which keys/users are consuming token X's quota" within 30 seconds using the UI filter, without writing SQL.
- **SC-003**: Zero increase in p99 request latency after the change (token recording happens in the existing logging callback path).
- **SC-004**: Historical SpendLog and ErrorLog entries (before migration) remain queryable with empty token identifier, no data corruption.
- **SC-005**: ErrorLogs for native proxy failures contain model name, upstream token, end_user, and organization_id — no more empty fields for data that is available at the call site.

## Assumptions

- The token identifier format (SHA256 prefix, 12 chars) is already established by the rate limit store and will not change.
- Only the Anthropic native proxy path involves multiple upstream OAuth tokens. Standard provider paths always use a single configured key and don't need token tracking.
- The UI spend logs page already has a filter mechanism that can be extended (no new filter infrastructure needed).
- SpendLogs volume is manageable with a simple indexed text column; no partitioning or archival changes are needed.
- The existing SpendLog columns (virtual key hash, user ID, team ID, model, token counts) already provide the "who/what" dimension — the missing piece is only the "through which upstream token" dimension.

## Scope

**In scope**: Recording token identifier in SpendLogs + ErrorLogs, enriching ErrorLogs with missing fields (model, end_user, organization_id), key alias in request log table, filtering spend logs by token in UI, displaying token in log detail page.

**Out of scope**: Per-token cost analytics dashboards, automatic load rebalancing based on per-key consumption, real-time alerting on per-key-per-token usage spikes (these may be future enhancements).
