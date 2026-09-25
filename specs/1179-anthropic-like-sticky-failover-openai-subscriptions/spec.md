# Feature Specification: Anthropic-like Sticky/Failover for OpenAI Subscriptions

**Feature Branch**: `HO-1179-anthropic-like-sticky-failover-openai-subscriptions`
**Created**: 2026-05-07
**Status**: Draft
**Input**: Linear HO-1179 - `[BE] Anthropic-like sticky/failover for OpenAI subscriptions`

## Scope Boundary

This issue adds routing over multiple configured OpenAI subscription credential IDs. It is a backend routing slice: selection, disabled exclusion, rate-limit/quota gating where OpenAI exposes enough response data, failover, and explicit all-unusable errors.

**In scope**:

- Build an OpenAI subscription candidate set from every configured `openai_subscription_credential_ids` entry instead of always choosing `ids[0]`.
- Exclude disabled credentials before routing.
- Reuse existing native upstream strategy semantics for OpenAI subscription credentials: default round-robin, sticky reuse, and utilization-aware selection where OpenAI response headers provide comparable request/token limit state.
- Make sticky selection org-wide: the sticky track must include org identity from request context, not per-user/per-token state and not one global track shared across all orgs.
- Gate candidates only when OpenAI rate-limit headers or explicit credential errors make them unavailable; missing headers must not fabricate utilization data.
- Fail over to another configured credential when the selected credential is unusable before request dispatch or when an official OpenAI HTTP endpoint returns a retryable/rate-limited response before any streaming body is relayed.
- Return explicit sanitized errors when every configured credential is missing, disabled, malformed, expired, refresh-failed, gated, or upstream-unavailable.
- Preserve the no-fallback invariant: when subscription credential IDs are configured, Tianji must not silently use `api_key`.

**Out of scope**:

- UI, credential creation screens, redaction/storage migrations, OAuth connect/callback, and token refresh internals already owned by sibling issues.
- Custom OpenAI-compatible `api_base` routing for subscription credentials; current validation already rejects that combination.
- Anthropic 5h/7d/sonnet utilization math for OpenAI. OpenAI headers expose request/token remaining and reset timing, not Anthropic OAuth quota windows.
- Cross-process persistent OpenAI sticky state. In-memory sticky state is enough for this slice, matching current native upstream routing behavior.
- Retrying a streaming response after bytes have already been sent to the client.

## User Scenarios & Testing

### User Story 1 - Resolve All Configured Subscription Credentials (Priority: P1)

As a TianjiLLM operator, I want every configured OpenAI subscription credential ID to participate in routing, so a model config can use multiple subscription accounts without editing deployment config for each request.

**Why this priority**: Current resolver chooses only the first configured ID. All routing, sticky, and failover behavior depends on first seeing the complete configured candidate set.

**Independent Test**: Resolver tests seed multiple OpenAI subscription credentials and assert the routing candidate list includes all usable configured IDs, excludes disabled credentials, and never uses `api_key` when IDs are configured.

**Acceptance Scenarios**:

1. **Given** a model config has credential IDs `cred_a`, `cred_b`, and `cred_c`, **When** the OpenAI subscription resolver builds candidates, **Then** all three IDs are evaluated in config order.
2. **Given** `cred_b` is marked disabled, **When** candidates are built, **Then** `cred_b` is excluded and the other usable credentials remain available.
3. **Given** all configured credentials are unusable, **When** a request is routed, **Then** Tianji returns a typed OpenAI subscription routing error with sanitized per-credential reason codes.
4. **Given** `api_key` is present and subscription credential IDs are also configured, **When** all subscription credentials fail, **Then** Tianji does not fall back to `api_key`.

---

### User Story 2 - Apply Org-wide Sticky Selection (Priority: P1)

As an org using OpenAI subscriptions through Tianji, I want sticky routing to keep my org on a stable subscription credential while it remains available, so prompt/cache affinity and quota behavior are predictable without leaking one org's sticky selection into another org.

**Why this priority**: The Linear scope explicitly requires org-wide sticky. Current Anthropic sticky state is provider/model-scoped; applying that shape directly to OpenAI would make sticky global across orgs.

**Independent Test**: Handler tests issue requests from two org contexts with multiple credentials and assert each org gets stable sticky reuse while orgs do not share the same sticky track.

**Acceptance Scenarios**:

1. **Given** `native_upstream_strategy: sticky`, **When** org `org_a` sends repeated OpenAI subscription requests and the selected credential remains usable, **Then** org `org_a` keeps the same credential.
2. **Given** org `org_a` and org `org_b` both call the same model, **When** sticky state is tracked, **Then** each org has an independent sticky track.
3. **Given** a sticky credential becomes disabled, expired, refresh-failed, or rate-limited, **When** the next request arrives, **Then** sticky re-evaluates and chooses another available credential.
4. **Given** no org ID exists because the master key path is used, **When** sticky routing runs, **Then** it uses a deterministic master/no-org track instead of panicking or using an empty ambiguous key.

---

### User Story 3 - Gate on OpenAI Rate-limit Signals Where Available (Priority: P1)

As a TianjiLLM operator, I want OpenAI subscription routing to avoid credentials that are currently rate-limited when response headers make that visible, so traffic naturally shifts to credentials with remaining capacity.

**Why this priority**: The issue requires quota/rate-limit gating where available. OpenAI official API responses expose request/token limit, remaining, and reset headers.

**Independent Test**: HTTP endpoint tests return OpenAI `x-ratelimit-*` headers and assert the selected credential state is updated, gated when remaining capacity reaches zero, and re-enabled after reset.

**Acceptance Scenarios**:

1. **Given** an OpenAI response includes `x-ratelimit-remaining-requests: 0` and `x-ratelimit-reset-requests: 60s`, **When** routing state is updated, **Then** that credential is gated until the request reset time.
2. **Given** an OpenAI response includes `x-ratelimit-remaining-tokens: 0` and `x-ratelimit-reset-tokens: 6m0s`, **When** routing state is updated, **Then** that credential is gated until the token reset time.
3. **Given** a response includes no OpenAI rate-limit headers, **When** routing state is updated, **Then** the credential remains available unless another explicit error made it unavailable.
4. **Given** OpenAI headers include request and token reset times, **When** sticky or utilization-aware strategy needs to choose among candidates, **Then** it uses OpenAI reset/remaining state without applying Anthropic-specific 5h/7d gates.

---

### User Story 4 - Fail Over Across Credentials Before Returning Failure (Priority: P1)

As a TianjiLLM client, I want requests to try another configured subscription credential when the selected one is unavailable, so one bad credential does not fail the whole model while another configured credential can serve the request.

**Why this priority**: Failover is the main reliability behavior requested by HO-1179.

**Independent Test**: Official OpenAI endpoint tests seed two credentials, make the first refresh fail or receive a pre-body 429/5xx, and assert the request is retried with the second credential before returning a response.

**Acceptance Scenarios**:

1. **Given** `cred_a` is selected first but token refresh fails, **When** `cred_b` is usable, **Then** the request is sent using `cred_b`.
2. **Given** the upstream returns 429 with OpenAI rate-limit headers before any response body is relayed, **When** another credential is available, **Then** Tianji gates the first credential and retries with the next credential.
3. **Given** the upstream returns a retryable 5xx before any response body is relayed, **When** another credential is available, **Then** Tianji retries with the next credential and records the first attempt as unavailable for this request.
4. **Given** a streaming response has already begun relaying bytes to the client, **When** the stream fails mid-body, **Then** Tianji does not attempt credential failover for that request.

## Edge Cases

- Duplicate credential IDs are already rejected by config validation and must remain rejected.
- Missing DB configuration with subscription IDs configured remains an explicit error.
- Malformed or wrong-type credentials must produce sanitized credential reason codes, not raw tokens or encrypted payloads.
- Disabled credentials must not become sticky selections and must not be attempted as fallback candidates.
- OpenAI subscription candidate identity must be credential ID or account ID, never raw bearer token.
- Failover must cap attempts at the configured candidate count to avoid loops.
- If every credential is unknown to rate-limit state, strategy must keep deterministic behavior based on config order/round-robin without inventing utilization.
- Codex app-server transport can share credential filtering/selection, but OpenAI HTTP response-header gating only applies to direct official OpenAI HTTP calls.

## Requirements

### Functional Requirements

- **FR-001**: System MUST build an OpenAI subscription routing candidate for each configured `openai_subscription_credential_ids` entry.
- **FR-002**: System MUST exclude disabled OpenAI subscription credentials from routing before selection.
- **FR-003**: System MUST keep current API-key behavior unchanged when no OpenAI subscription credential IDs are configured.
- **FR-004**: System MUST NOT fall back to `api_key` when one or more OpenAI subscription credential IDs are configured and all configured credentials are unusable.
- **FR-005**: System MUST return a sanitized explicit error when all configured OpenAI subscription credentials are unusable.
- **FR-006**: System MUST preserve typed credential failure reason codes such as missing, disabled, wrong type, malformed, expired, lookup failed, and refresh failed.
- **FR-007**: System MUST honor `native_upstream_strategy` for OpenAI subscription routing using provider-appropriate semantics: default round-robin, sticky reuse, and utilization-aware ordering only from OpenAI rate-limit state.
- **FR-008**: System MUST scope OpenAI sticky state by org ID plus routing/model class; master-key/no-org requests MUST use a deterministic separate track.
- **FR-009**: System MUST re-evaluate sticky selection when the sticky credential is no longer available.
- **FR-010**: System MUST parse OpenAI official API rate-limit headers for request/token limit, remaining, and reset timing when present.
- **FR-011**: System MUST gate credentials whose parsed OpenAI remaining requests or remaining tokens are exhausted until the relevant reset time.
- **FR-012**: System MUST leave credentials available when OpenAI rate-limit headers are absent and no other explicit error exists.
- **FR-013**: System MUST fail over to another configured credential after pre-dispatch credential resolution/refresh failure.
- **FR-014**: System MUST fail over after a pre-body retryable OpenAI HTTP response such as 429 or 5xx when another credential is available.
- **FR-015**: System MUST NOT fail over a streaming response after bytes have already been relayed to the client.
- **FR-016**: System MUST avoid logging or returning raw access tokens, refresh tokens, bearer tokens, encrypted credential values, or full upstream sensitive payloads.

### Key Entities

- **OpenAI Subscription Route Candidate**: Configured credential ID plus resolved transport material, account ID, availability reason, and current OpenAI rate-limit state.
- **OpenAI Rate-limit State**: In-memory per-credential state derived from official OpenAI response headers: request/token limits, remaining counts, reset deadlines, and last update time.
- **OpenAI Sticky Track**: In-memory sticky state keyed by org scope and routing/model class, pointing at a selected credential ID while it remains available.
- **Routing Attempt**: One request attempt using one candidate credential; retries are capped at candidate count.
- **All Credentials Unusable Error**: Sanitized error aggregating per-credential reason codes for all configured IDs.

## Success Criteria

- **SC-001**: Multiple configured credential IDs participate in routing; tests prove the resolver no longer always selects only `ids[0]`.
- **SC-002**: Disabled credentials are excluded and never attempted.
- **SC-003**: Sticky selection is stable per org and does not leak between orgs.
- **SC-004**: A selected unusable credential fails over to the next usable credential.
- **SC-005**: 429/rate-limit responses update OpenAI rate-limit state and gate the exhausted credential.
- **SC-006**: All-unusable subscription configs return an explicit sanitized error and do not use fallback `api_key`.
- **SC-007**: Existing API-key OpenAI routing still works when no subscription IDs are configured.

## Assumptions

- OpenAI official HTTP APIs expose `x-ratelimit-limit-requests`, `x-ratelimit-limit-tokens`, `x-ratelimit-remaining-requests`, `x-ratelimit-remaining-tokens`, `x-ratelimit-reset-requests`, and `x-ratelimit-reset-tokens` headers where rate-limit data is available.
- Current `native_upstream_strategy` values remain `round_robin`, `lowest_utilization`, and `sticky`.
- The authoritative org scope is `middleware.ContextKeyOrgID`; missing org ID is expected for master-key/admin paths and must not block routing.
- This Todo gate creates planning artifacts only. Production code begins only after Linear HO-1179 moves to In Progress.
