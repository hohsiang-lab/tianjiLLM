# Feature Specification: OpenAI Quota/Rate-limit Header Parser and Store Integration

**Feature Branch**: `HO-1181-openai-quota-rate-limit-header-parser-and-store`
**Created**: 2026-05-07
**Status**: Draft
**Input**: Linear HO-1181 - `[BE] OpenAI quota/rate-limit header parser and store integration`

## Summary

Parse quota and rate-limit signals returned by official OpenAI subscription upstreams, normalize the parsed result into a stable internal store contract, and make the existing OpenAI subscription routing gate, sticky selection, and lowest-utilization selection consume that stored state. Missing headers must degrade safely. TianjiLLM must not estimate billing quota, monthly spend, or subscription entitlement locally.

## Scope

In scope:

- Parse official OpenAI `x-ratelimit-*` response headers for request/token limit, remaining, and reset timing.
- Parse any OpenAI subscription mock-harness quota status headers that are already available to tests, including allowed/rejected/reset/utilization cases, without treating those mock-only signals as proof of public OpenAI API guarantees.
- Normalize parsed state into one store-facing OpenAI quota/rate-limit model keyed by credential ID or account ID, never by raw bearer/access/refresh token.
- Feed the existing OpenAI subscription gate/sticky/lowest-utilization routing paths from that normalized store state.
- Update state on 200, 401 retry, 429, and retryable 5xx responses whenever headers are present before the response body is committed.
- Preserve safe degradation when headers are absent, malformed, partial, or expired.

Out of scope:

- Estimating billing quota, subscription allowance, usage tier, monthly spend, or remaining dollars locally.
- UI changes, OpenAI OAuth connect/callback, credential persistence schema changes, token refresh lifecycle, and redaction policy changes owned by sibling issues.
- Anthropic 5h/7d/sonnet quota math. OpenAI gets provider-specific semantics.
- Custom OpenAI-compatible `api_base` providers; this is official OpenAI subscription routing only.
- Cross-process distributed sticky coordination unless an existing store interface already provides it.
- Real OpenAI network calls in tests.

## User Stories and Tests

### User Story 1 - Parse Official OpenAI Rate-limit Headers (Priority: P1)

As a TianjiLLM operator, I want official OpenAI response headers parsed into typed state, so routing can avoid credentials that OpenAI says have no request or token capacity left.

**Independent Test**: Parser tests pass `http.Header` fixtures with request exhaustion, token exhaustion, both dimensions, RFC3339 reset fallback if supported, malformed values, and empty headers.

**Acceptance Scenarios**:

1. Given `x-ratelimit-remaining-requests: 0` and `x-ratelimit-reset-requests: 1s`, when parsed, then request capacity is marked exhausted until the reset deadline.
2. Given `x-ratelimit-remaining-tokens: 0` and `x-ratelimit-reset-tokens: 6m0s`, when parsed, then token capacity is marked exhausted until the reset deadline.
3. Given limit and remaining headers are present with non-zero remaining counts, when parsed, then the credential remains available and utilization can be derived only from parsed limit/remaining values.
4. Given no official rate-limit headers are present, when parsed, then no state update is emitted and no gate is fabricated.

### User Story 2 - Normalize Mock Quota Status Signals (Priority: P1)

As a test author, I want OpenAI subscription mock quota fixtures to exercise allowed, rejected, reset, and utilization states, so the store/gate behavior can be validated without real OpenAI calls.

**Independent Test**: Mock-harness tests return status/utilization/reset fixtures and assert normalized store state matches allowed/rejected/reset behavior.

**Acceptance Scenarios**:

1. Given a mock quota status indicates `allowed`, when parsed, then the credential remains selectable.
2. Given a mock quota status indicates `rejected` with a future reset, when parsed, then the credential is gated until reset.
3. Given mock utilization is present, when normalized, then utilization is clamped to `[0,1]` and missing utilization remains explicit unknown.
4. Given mock-only headers are absent in production responses, when parsing official OpenAI headers, then official header behavior is unchanged.

### User Story 3 - Store Parsed State Once and Reuse It Everywhere (Priority: P1)

As a maintainer, I want one OpenAI quota/rate-limit store contract instead of per-handler private maps, so gate, sticky, and lowest-utilization routing do not diverge.

**Independent Test**: Store integration tests update a credential state from response headers, then assert candidate gating, sticky re-evaluation, and lowest-utilization selection read that stored state.

**Acceptance Scenarios**:

1. Given a response updates `cred_a` as request-exhausted, when the next request resolves candidates, then `cred_a` is gated and another usable credential is selected.
2. Given sticky currently points to `cred_a` and stored state marks `cred_a` rejected/exhausted, when a later request arrives, then sticky re-evaluates.
3. Given lowest-utilization strategy sees `cred_a` at high utilization and `cred_b` lower, when both are otherwise available, then `cred_b` is preferred.
4. Given state expires after reset, when routing evaluates it, then stale exhaustion is cleared and the credential can be considered again.

### User Story 4 - Safe Degradation and No Secret Leakage (Priority: P1)

As a TianjiLLM operator, I want bad or missing upstream headers to be harmless, so routing continues without leaking sensitive tokens or making false quota claims.

**Independent Test**: Negative tests cover absent headers, malformed integers/durations, out-of-range utilization, token-shaped header values, and response bodies that contain secrets.

**Acceptance Scenarios**:

1. Given malformed numeric or duration headers, when parsed, then the parser ignores the bad field and does not gate solely from it.
2. Given partial headers are present, when normalized, then known dimensions update and unknown dimensions remain unknown.
3. Given parsed state is logged, persisted, returned in errors, or surfaced in diagnostics, then raw access tokens, refresh tokens, bearer values, JWTs, and encrypted credential values are absent.
4. Given a response has no quota/rate-limit headers, when routing continues, then behavior matches current no-data fallback.

## Functional Requirements

- **FR-001**: System MUST parse official OpenAI response headers `x-ratelimit-limit-requests`, `x-ratelimit-limit-tokens`, `x-ratelimit-remaining-requests`, `x-ratelimit-remaining-tokens`, `x-ratelimit-reset-requests`, and `x-ratelimit-reset-tokens`.
- **FR-002**: System MUST parse reset headers as durations relative to parse time; RFC3339 support is allowed only as a compatibility fallback and must be tested if implemented.
- **FR-003**: System MUST derive request/token utilization only when both positive limit and non-negative remaining values are available.
- **FR-004**: System MUST represent unknown values explicitly; unknown MUST NOT be treated as zero capacity or zero utilization.
- **FR-005**: System MUST parse available mock-harness quota status/utilization/reset fixtures for allowed/rejected/reset/utilization cases without documenting them as official OpenAI public headers.
- **FR-006**: System MUST normalize parsed OpenAI state into one store-facing type or interface consumed by OpenAI subscription routing.
- **FR-007**: System MUST key OpenAI quota/rate-limit state by credential ID or account ID, not raw token material.
- **FR-008**: System MUST update stored state on direct official OpenAI HTTP responses whenever headers are present, including 200, 401 retry paths, 429, and retryable 5xx before response body commitment.
- **FR-009**: System MUST gate a credential when parsed status is rejected/exhausted and the relevant reset deadline is in the future.
- **FR-010**: System MUST clear or ignore expired exhaustion after the relevant reset time passes.
- **FR-011**: System MUST make sticky and lowest-utilization OpenAI subscription routing consume the same normalized store state used by the gate.
- **FR-012**: System MUST preserve existing API-key behavior when no OpenAI subscription credential IDs are configured.
- **FR-013**: System MUST NOT estimate monthly billing quota, usage tier, spend caps, or subscription allowance locally.
- **FR-014**: Tests MUST use `httptest` and `internal/testutil/openaitest`; no test may call `api.openai.com` or `auth.openai.com`.
- **FR-015**: Errors, logs, metadata, and test output MUST NOT expose raw access tokens, refresh tokens, bearer values, JWTs, encrypted credential values, or full sensitive upstream payloads.

## Key Entities

- **OpenAI Quota Header Snapshot**: Raw response-header-derived facts from official OpenAI and mock-harness quota headers.
- **OpenAI Quota State**: Normalized per-credential/account state containing status, request/token limits, remaining counts, reset deadlines, utilization values, updated time, and unknown flags.
- **OpenAI Quota Store**: Store-facing interface or existing store extension that writes and reads OpenAI quota state for routing.
- **Routing Gate Decision**: Computed availability result from stored state at a given time.
- **Sticky Re-evaluation Trigger**: Stored state transition that invalidates the current sticky credential because it is exhausted/rejected until reset.

## Edge Cases

- Reset headers with `1s`, `6m0s`, larger durations, empty values, invalid strings, and already-expired deadlines.
- Remaining count `0` without reset header: state updates, but gating must be conservative because no future reset deadline exists.
- Limit `0` or missing limit with remaining present: do not divide by zero; utilization remains unknown.
- Remaining count greater than limit: clamp derived utilization to `[0,1]` and preserve raw counts for diagnostics if needed.
- Multiple response attempts in one client request update different credentials.
- 401 forced refresh retry returns headers for both the failed and refreshed attempt.
- Streaming response body already committed: do not attempt new failover, but response headers seen before commit may still update state.

## Success Criteria

- **SC-001**: Parser tests cover official request/token exhausted, non-exhausted, missing, malformed, and reset cases.
- **SC-002**: Mock quota tests cover allowed, rejected, reset, and utilization cases available from the mock harness.
- **SC-003**: Store integration tests prove parsed state updates one normalized store path, not only a handler-private map.
- **SC-004**: Routing tests prove gate, sticky re-evaluation, and lowest-utilization consume stored OpenAI quota state.
- **SC-005**: Missing or malformed headers degrade safely and never fabricate local billing/quota estimates.
- **SC-006**: Redaction tests prove no token material leaks from parser/store diagnostics.

## Dependencies

- HO-1167: OpenAI subscription config and credential resolution.
- HO-1176: Credential persistence and metadata helpers.
- HO-1177: Token refresh manager.
- HO-1178: Subscription bearer injection across official OpenAI endpoints.
- HO-1179: Multi-credential OpenAI subscription routing, sticky, failover, and initial private rate-limit state.
- HO-1180: 401 forced refresh/retry/failover behavior.
- HO-1183: Token/JWT redaction across subscription status and error sinks.
- HO-1190: OpenAI OAuth/upstream mock harness and no-real-OpenAI guard.
