# Feature Specification: Remove Codex failed-SSE synthetic cooldown

**Feature Branch**: `1466-remove-codex-sse-cooldown`

**Created**: 2026-08-13

**Status**: Draft

**Input**: User description: "做窄移除：移除 Tianji 對 Codex HTTP 200 failed-SSE 的固定 2 分鐘 cooldown。Tianji 只負責 proxy，不應替 upstream 做本地 cooldown；保留 upstream failure passthrough、官方 quota header gating、HTTP 429/5xx failover、既有 usage snapshot backoff 與其他 credential safety 行為。"

## Scope

Remove only Tianji's locally synthesized two-minute cooldown for a Codex response that has already started as an HTTP 200 SSE stream and later contains a qualifying `response.failed` event such as an upstream overload/capacity failure.

The proxy continues to parse and log the failed event and continues to relay the original HTTP 200/SSE body. It must not convert that event into a local rejected quota state, a future `rate_limited` credential-resolution failure, or a same-request retry after response bytes have been committed.

## Non-Goals

- Do not add a replacement cooldown, exponential backoff, jitter, retry loop, or scheduler for failed SSE responses.
- Do not change official `x-ratelimit-*` header parsing or quota-state gating.
- Do not change pre-body HTTP 429/5xx subscription failover.
- Do not change disabled-credential exclusion, credential refresh, or credential persistence.
- Do not change Codex usage snapshot refresh/cache/backoff behavior.
- Do not change the upstream HTTP 200/SSE payload or response status exposed to the client.
- Do not retry a request after streaming bytes have been sent to the client.
- Do not make database schema or durable credential-state changes.

## User Scenarios & Testing

### User Story 1 - Failed SSE is proxied without a Tianji cooldown (Priority: P1)

As a client using Codex through Tianji, I want an upstream HTTP 200 SSE failure to pass through as received, without Tianji locally blocking the credential for two minutes, so the upstream service—not the proxy—owns recovery timing.

**Why this priority**: The synthetic gate is the behavior being removed. It can incorrectly classify a credential as `rate_limited` even though the upstream only returned an in-stream failure after the request had already begun.

**Independent Test**: A focused handler test makes the mocked Codex backend return HTTP 200 with a qualifying `response.failed` SSE event, sends one `/v1/responses` request, and asserts the response body/status and failure log remain present while the selected credential has no newly recorded local rejected/reset gate.

**Acceptance Scenarios**:

1. **Given** a Codex backend returns HTTP 200 and a qualifying `response.failed` SSE event, **When** Tianji handles `/v1/responses`, **Then** Tianji returns HTTP 200 with the original failed SSE content and does not create a local rejected quota state for the attributed credential.
2. **Given** the same failed SSE event was observed, **When** the next credential resolution runs immediately, **Then** the credential is not reported as `rate_limited` solely because of that failed SSE event.
3. **Given** the failed SSE is non-qualifying (for example invalid request or permission failure), **When** Tianji handles it, **Then** existing failure parsing/logging behavior remains unchanged and no synthetic cooldown is created.

### User Story 2 - Existing upstream-owned protections remain intact (Priority: P1)

As a Tianji maintainer, I want the narrow removal to leave all existing upstream-owned or pre-body safety behavior unchanged, so removing one local policy does not weaken unrelated routing and quota handling.

**Why this priority**: The change is safe only if it removes the synthetic post-commit gate and does not alter official quota signals, failover, credential lifecycle, or usage snapshot behavior.

**Independent Test**: Existing focused tests for official rate-limit headers, HTTP 429/5xx subscription failover, disabled credentials, credential refresh, and Codex usage snapshot backoff remain green; a regression test proves no post-commit retry is introduced.

**Acceptance Scenarios**:

1. **Given** an upstream response supplies recognized `x-ratelimit-*` headers, **When** Tianji records them, **Then** existing quota-state parsing and gating behavior is unchanged.
2. **Given** a subscription attempt receives HTTP 429 or HTTP 5xx before response output is committed, **When** multiple candidates are configured, **Then** existing pre-body failover behavior is unchanged.
3. **Given** a response has already committed HTTP 200/SSE bytes, **When** a failed event is detected, **Then** Tianji does not retry the same request or emit duplicate output.
4. **Given** Codex usage snapshot refresh encounters its own upstream failure, **When** its existing cache/backoff path runs, **Then** that independent exponential backoff and jitter behavior is unchanged.

## Edge Cases

- A failed SSE body is empty, malformed, or lacks a recognized failure event: preserve current passthrough and logging behavior; do not create a synthetic gate.
- The response body contains a non-qualifying failure such as invalid request, unsupported value, missing scope, or permission error: preserve current diagnostics; do not create a synthetic gate.
- The attributed credential ID is missing: do not attempt to record local quota state or log secret material.
- The handler or attribution is nil: failure observation remains safe and must not panic.
- A credential already has an independent official quota/header gate or disabled state: this feature does not clear, overwrite, or bypass that state.
- Multiple credentials are configured: removal of the failed-SSE gate does not add a new selection policy or reorder candidates beyond existing behavior.

## Requirements

### Functional Requirements

- **FR-001**: Tianji MUST NOT synthesize a local rejected quota state or future `rate_limited` credential-resolution result solely from a qualifying Codex `response.failed` event received inside an already-started HTTP 200 SSE response.
- **FR-002**: Tianji MUST continue to detect and log the failed SSE event with the existing safe failure fields and MUST continue relaying the original HTTP status and body to the client.
- **FR-003**: Tianji MUST NOT retry a request after any response bytes have been committed to the client.
- **FR-004**: Tianji MUST preserve official OpenAI rate-limit-header parsing and quota gating for states derived from upstream headers.
- **FR-005**: Tianji MUST preserve existing HTTP 429/5xx pre-body subscription failover and credential-attribution behavior.
- **FR-006**: Tianji MUST preserve disabled-credential exclusion, credential refresh, and durable credential persistence behavior.
- **FR-007**: Tianji MUST preserve the separate Codex usage snapshot cache, refresh, exponential backoff, and jitter path; this feature MUST NOT alter or reuse that policy for failed SSE responses.
- **FR-008**: Regression coverage MUST verify the absence of the synthetic failed-SSE gate and the continued presence of passthrough/logging behavior.
- **FR-009**: The change MUST remain narrow: no database schema, persistent quota-state format, public endpoint, or new retry/backoff abstraction is introduced.

### Key Entities

- **Codex failed-SSE event**: An upstream `response.failed` event observed inside a response that began with HTTP 200, including safe type/code/message fields used for logging.
- **OpenAI quota state**: Tianji's in-memory or existing stored state used for official quota signals and credential gating; this feature removes only the synthetic state derived from failed SSE.
- **Subscription credential resolution**: The existing candidate-selection path that may report a credential as `rate_limited` when an independent quota gate is active.

## Success Criteria

### Measurable Outcomes

- **SC-001**: The focused failed-SSE regression test passes with HTTP 200/body passthrough and observes no newly recorded rejected/reset quota state for the attributed credential.
- **SC-002**: The focused failed-SSE regression test still records the existing failure log/event and does not report a success callback for the failed request.
- **SC-003**: Existing tests covering official quota headers, HTTP 429/5xx failover, credential refresh/disable, and usage snapshot backoff remain green after the narrow change.
- **SC-004**: No test or production path performs a same-request retry after the HTTP 200/SSE response has committed bytes.
- **SC-005**: The implementation changes only the failed-SSE handling/test boundary and introduces no schema, public API, or new backoff state.

## Assumptions

- The current repository source and focused regression test are the source of truth for this planning-only change.
- “Narrow removal” means deleting the locally synthesized two-minute failed-SSE gate, not removing all quota gating or all retry behavior.
- OpenAI remains responsible for upstream capacity/rate-limit recovery for an already-started failed SSE.
- The implementation will use existing handler, callback, routing, and test helpers; no new dependency is needed.
- Production implementation is deferred until this specification, plan, task list, and cross-artifact analysis are clean.
