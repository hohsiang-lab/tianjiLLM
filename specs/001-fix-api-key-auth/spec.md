# Feature Specification: Fix API Key Authentication

**Feature Branch**: `001-fix-api-key-auth`
**Created**: 2026-02-25
**Status**: Draft
**Input**: User description: "Fix issue #9: Created API keys cannot authenticate - only master key works. Root cause: cmd/tianji/main.go:423 creates proxy.ServerConfig without passing DBQueries, so auth middleware Validator is always nil and virtual key validation is skipped, returning 401."

## Clarifications

### Session 2026-02-25

- Q: What HTTP status code should the proxy return when the database is unreachable during virtual key validation? → A: 503 Service Unavailable
- Q: Should new unit and integration tests be added as part of this fix to prevent regression? → A: Yes — add targeted unit tests for the auth middleware wiring and an integration test covering the full virtual key auth path
- Q: Should virtual key database lookups be cached (in-memory or Redis) as part of this fix? → A: No — caching is out of scope; this fix addresses only the wiring bug; caching is a separate performance optimization
- Q: Should authentication events (success and failure) emit structured log entries? → A: Yes — emit structured log entries at INFO for successful virtual key auth and WARN/ERROR for auth failures, consistent with existing codebase logging conventions
- Q: Is expired-key enforcement (checking an `expires_at` field) in scope for this fix? → A: No — out of scope; the assumption of no schema changes is preserved; expired key handling is a separate feature

## User Scenarios & Testing *(mandatory)*

### User Story 1 - API Key Authentication Works (Priority: P1)

An administrator creates a virtual API key through the UI or management API and distributes it to a developer. The developer sends requests to the proxy using that API key. Currently, the proxy always returns 401 Unauthorized even when the key is valid, because the key lookup against the database is never performed. After this fix, the proxy correctly validates virtual keys against the database and allows authenticated requests through.

**Why this priority**: This is a critical regression that completely blocks all users from using virtual API keys, making the key management feature non-functional. Only the master key works, which is unsuitable for production environments where scoped, revocable keys are required.

**Independent Test**: Create a virtual API key via the UI, send a request to `POST /v1/chat/completions` using that key in the `Authorization: Bearer` header, and verify a successful response (not 401).

**Acceptance Scenarios**:

1. **Given** a valid virtual API key has been created and stored in the database, **When** a client sends a request with that key, **Then** the proxy authenticates the request and routes it to the appropriate upstream provider
2. **Given** a valid virtual API key, **When** the request succeeds, **Then** the authenticated user's identity (user ID, team ID) is correctly associated with the request for spend tracking
3. **Given** a virtual API key that has been blocked by an administrator, **When** a client sends a request with that key, **Then** the proxy returns 403 Forbidden (not 401)
4. **Given** a completely invalid or non-existent key, **When** a client sends a request, **Then** the proxy returns 401 Unauthorized

---

### User Story 2 - Master Key Still Works (Priority: P2)

After the fix, the master key continues to work exactly as before. No regression is introduced to master key authentication.

**Why this priority**: While virtual keys are the broken path, verifying master key auth is unaffected prevents introducing new regressions.

**Independent Test**: Send a request using the master key and verify it still authenticates successfully.

**Acceptance Scenarios**:

1. **Given** the master key configured in the system, **When** a client sends a request with that key, **Then** the request is authenticated as a master-key request and processed normally
2. **Given** a request authenticated with the master key, **When** the request is processed, **Then** no spend tracking or budget limits are applied (master key bypasses these)

---

### User Story 3 - Virtual Key Guardrails Apply (Priority: P3)

When a virtual API key has guardrails (banned keywords, content policies) associated with it, those guardrails are correctly loaded and enforced for requests authenticated with that key.

**Why this priority**: This is a secondary concern — correct authentication must work first, then guardrail enforcement. This validates that the full virtual key context (not just auth) is properly restored.

**Independent Test**: Create a virtual API key with a guardrail configured, send a request that violates the guardrail, and verify the guardrail blocks the request.

**Acceptance Scenarios**:

1. **Given** a virtual API key with an associated guardrail, **When** a client sends a request with content matching the guardrail rule, **Then** the request is blocked with an appropriate error
2. **Given** a virtual API key with an associated guardrail, **When** a client sends a clean request, **Then** the request passes through normally

---

### Edge Cases

- What happens when the database is unreachable during virtual key validation? The proxy returns **503 Service Unavailable** (not 401 or 200), distinguishing infrastructure failure from auth failure and allowing clients to implement retry logic
- What happens when a virtual key exists in the database but has expired? Out of scope for this fix — expired key enforcement requires a separate feature; current behavior is unchanged
- What happens when a request is made with no API key at all? The proxy should return 401 Unauthorized
- What happens when a key exists in both the master key config and the database? Master key check takes precedence (current behavior is preserved)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST validate virtual API keys against the persistent key store for every authenticated request
- **FR-002**: System MUST allow requests authenticated with a valid, non-blocked virtual key to proceed to the upstream provider
- **FR-003**: System MUST reject requests with blocked virtual keys with a 403 Forbidden response
- **FR-004**: System MUST reject requests with invalid or non-existent virtual keys with a 401 Unauthorized response
- **FR-005**: System MUST associate authenticated virtual key requests with the correct user and team identity for downstream spend tracking and budget enforcement
- **FR-006**: System MUST continue to authenticate master key requests as before, with no regression
- **FR-007**: System MUST load and apply any guardrails associated with a virtual key during authentication
- **FR-008**: System MUST preserve existing behavior: master key authentication is checked before virtual key lookup
- **FR-009**: System MUST return HTTP 503 Service Unavailable when the database is unreachable during virtual key validation
- **FR-010**: System MUST emit structured log entries at INFO level on successful virtual key authentication and at WARN/ERROR level on authentication failures, using the existing codebase logging conventions
- **FR-011**: This fix MUST be accompanied by targeted unit tests for the auth middleware wiring and at least one integration test covering the full virtual key authentication path

### Key Entities

- **Virtual API Key**: A scoped, revocable authentication token stored in the database. Has attributes: token hash, owner user ID, team ID, blocked status, associated guardrails
- **Master Key**: A single administrative key configured at server startup. Not stored in the database; checked first in auth flow
- **Authentication Context**: The resolved identity (user ID, team ID, is-master-key flag, token hash, guardrail names) attached to a request after successful authentication

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of requests using valid virtual API keys are authenticated successfully (0% false-negative rate)
- **SC-002**: 0 regressions in master key authentication — all existing master key requests continue to succeed
- **SC-003**: Blocked virtual keys correctly receive 403 (not 401 or 200) in 100% of cases
- **SC-004**: Invalid or non-existent keys receive 401 in 100% of cases
- **SC-005**: Authentication latency for virtual key validation adds no more than 50ms overhead compared to master key authentication (one database lookup is acceptable)
- **SC-006**: All existing automated tests pass with no modifications required
- **SC-007**: Database unavailability during virtual key lookup results in 503 (not 401 or 500) in 100% of cases
- **SC-008**: Authentication success and failure events produce structured log entries observable in the application log stream

## Assumptions

- The virtual key storage (database) is already fully implemented and correctly stores/retrieves virtual keys — only the wiring to connect the auth middleware to the database is broken
- The master key comparison logic is correct and only the virtual key validation path is affected
- No schema or query changes are needed; only configuration/wiring is missing
- The fix is a one-line or minimal change at server initialization time, not a redesign of the auth system
- The existing test suite covers the expected auth behavior; the bug exists because the integration point was not tested end-to-end
- Virtual key expiration enforcement (`expires_at`) is explicitly out of scope for this fix and will be addressed as a separate feature
- Virtual key lookup caching (in-memory or Redis) is explicitly out of scope for this fix
