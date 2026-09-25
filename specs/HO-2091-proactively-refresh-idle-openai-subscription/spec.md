# Feature Specification: Proactive OpenAI Subscription Credential Refresh

**Feature Branch**: `HO-2091-proactively-refresh-idle-openai-subscription`
**Created**: 2026-06-18
**Status**: Draft
**Input**: Linear HO-2091 - `[Tianji] Proactively refresh idle OpenAI subscription credentials`

## Summary

Tianji must proactively refresh active `openai_subscription` credentials before their stored access tokens expire, even when no route, UI test, Codex usage refresh, or upstream 401 path touches them. Operators must see a clear reconnect-required state when OpenAI invalidates the refresh token/session, and routing must stop repeatedly considering credentials that are already failed, disabled, or deleted.

## User Scenarios & Testing _(mandatory)_

### User Story 1 - Idle Credentials Refresh Before Expiry (Priority: P1)

As an operator, I need active idle OpenAI subscription credentials refreshed in the background before expiry so unused accounts do not silently become stale in the database.

**Why this priority**: This directly prevents the incident class where the next user request is the first moment Tianji discovers an expired or unrecoverable session.

**Independent Test**: Seed an active credential whose `expires_at` is within the proactive buffer, run the proactive job once, and assert the mock OpenAI token endpoint is called once and the stored access token / expiry are updated.

**Acceptance Scenarios**:

1. **Given** an active `openai_subscription` credential has `expires_at <= now + 30m`, **When** the proactive refresh job runs, **Then** Tianji refreshes it through the existing refresh path and updates the stored access token and expiry.
2. **Given** an active `openai_subscription` credential has `expires_at > now + 30m`, **When** the proactive refresh job runs, **Then** Tianji skips it and performs no upstream token call.
3. **Given** the server is running normally with database access, **When** time passes, **Then** the proactive refresh job runs on the configured cadence without requiring user traffic.

---

### User Story 2 - No Stampede Across Pods or Request-Time Resolution (Priority: P1)

As the proxy under concurrent workloads and multiple pods, I need proactive refresh and request-time refresh to coordinate so the same credential is not refreshed by multiple workers at once.

**Why this priority**: Duplicate refresh calls can race with refresh-token rotation and create the exact unrecoverable session behavior this issue is meant to reduce.

**Independent Test**: Run two proactive job instances and a request-time resolver against the same near-expiry credential with a blocking mock token endpoint; assert only one upstream refresh request succeeds and the other callers observe the refreshed credential or skip under lock.

**Acceptance Scenarios**:

1. **Given** two Tianji pods run the proactive job at the same time, **When** both see the same eligible credential, **Then** only one pod refreshes that credential.
2. **Given** request-time resolution starts while proactive refresh is refreshing the same credential, **When** both use the existing credential refresh lock semantics, **Then** only one upstream refresh is in flight for that credential.
3. **Given** a job cannot acquire the refresh lock for a credential, **When** the job records its result, **Then** it treats the credential as skipped rather than failed.

---

### User Story 3 - Reconnect Required State Is Persisted And Visible (Priority: P1)

As an operator, I need Tianji to distinguish a recoverable access-token refresh from an invalidated OpenAI session so I know when to reconnect the credential.

**Why this priority**: The incident showed `refresh_token_invalidated` / `Your session has ended. Please log in again.` was discovered late and surfaced as noisy route freshness failures instead of an operator action.

**Independent Test**: Configure the mock token endpoint to return `401 refresh_token_invalidated`; run proactive refresh; assert metadata stores the first failure timestamp/reason, the credential becomes non-selectable, and UI/action responses show `OpenAI session ended; reconnect required`.

**Acceptance Scenarios**:

1. **Given** OpenAI returns `refresh_token_invalidated`, **When** proactive refresh handles the failure, **Then** Tianji persists a stable non-selectable state and first failure timestamp/reason.
2. **Given** a credential is in that state, **When** an operator views the credential UI or lifecycle response, **Then** the message says `OpenAI session ended; reconnect required`.
3. **Given** a transient refresh failure occurs without an invalidated-session signal, **When** metadata is persisted, **Then** Tianji records a safe failure reason without falsely claiming reconnect is required.

---

### User Story 4 - Stale Route Candidates Stop Repeating Noise (Priority: P2)

As an operator maintaining model routes, I need route candidate lists to stop repeatedly selecting failed/deleted credentials and to make stale references visible or removable.

**Why this priority**: Failed or deleted credentials left in route candidate lists caused repeated `codex-sticky` not-fresh noise and hid the actionable reconnect/route-cleanup work.

**Independent Test**: Configure routes with active, refresh-failed, disabled, and missing credential IDs; assert routing skips failed/deleted credentials and model UI/logging reports stale references without exposing secrets.

**Acceptance Scenarios**:

1. **Given** a model route includes a credential with `status=refresh_failed`, **When** route resolution builds candidates, **Then** that credential is skipped before route selection.
2. **Given** a model route includes a missing or deleted credential ID, **When** the route is viewed or evaluated, **Then** Tianji surfaces the stale reference clearly.
3. **Given** all configured route credentials are non-selectable, **When** a request resolves the route, **Then** Tianji returns a clear sanitized unusable-credential error rather than repeatedly logging sticky freshness noise.

### Edge Cases

- The database is not configured; the proactive job must not run.
- The refresh token is absent or the encrypted bundle is malformed.
- OpenAI returns invalid JSON, network failure, `429`, `5xx`, or a non-session `401`.
- A refresh response rotates the refresh token or omits it.
- The same credential is eligible on multiple pods.
- A credential is operator-disabled and must remain untouched.
- A credential is already `refresh_failed` or deleted and must not be selected again.
- Route config references a credential ID that no longer exists.
- Logs, metadata, UI responses, Linear/GitHub comments, and tests must never expose access tokens, refresh tokens, JWTs, bearer strings, or encrypted credential blobs.

## Requirements _(mandatory)_

### Functional Requirements

- **FR-001**: System MUST run a proactive OpenAI subscription credential refresh job every 5 minutes when database-backed credential storage is available.
- **FR-002**: System MUST process only `openai_subscription` credentials whose safe metadata marks them active/selectable and whose `expires_at <= now + 30m`.
- **FR-003**: System MUST skip credentials whose `expires_at > now + 30m` without an upstream token request.
- **FR-004**: System MUST keep the existing request-time freshness rule of `expires_at > now + 5m` unchanged.
- **FR-005**: System MUST reuse the existing OpenAI subscription refresh path and token rotation semantics for proactive refresh.
- **FR-006**: System MUST coordinate proactive refresh with existing same-credential refresh locking so concurrent pods and request-time resolution do not stampede.
- **FR-007**: System MUST update `access_token`, `expires_at`, rotated `refresh_token` when present, and safe refresh metadata after successful proactive refresh.
- **FR-008**: System MUST clear transient refresh failure metadata after a successful refresh.
- **FR-009**: System MUST persist first refresh failure timestamp and stable safe reason when refresh fails.
- **FR-010**: System MUST classify upstream `refresh_token_invalidated` as a reconnect-required terminal session state with operator message `OpenAI session ended; reconnect required`.
- **FR-011**: System MUST mark reconnect-required credentials non-selectable by routing.
- **FR-012**: System MUST skip credentials marked `refresh_failed`, `disabled`, reconnect-required, deleted, or missing when building route candidates.
- **FR-013**: System MUST surface stale route candidate references in the model UI, route diagnostics, logs, or an auto-prune result so operators can correct model routing.
- **FR-014**: System MUST preserve existing API-key route behavior when no `openai_subscription_credential_ids` are configured.
- **FR-015**: System MUST redact secret material in failure metadata, audit payloads, logs, UI responses, PR/Linear evidence, and test fixtures.
- **FR-016**: System MUST test proactive refresh with mocks only and MUST NOT call real `auth.openai.com` or `api.openai.com`.

### Key Entities _(include if feature involves data)_

- **OpenAI Subscription Credential**: A `CredentialTable` row with encrypted `credential_value` and safe `credential_info` metadata.
- **Proactive Refresh Candidate**: An active `openai_subscription` credential whose access token expires inside the 30-minute proactive buffer.
- **Refresh Failure Metadata**: Redacted metadata that records status, first failure time, latest failure time, reason code, and operator-facing message without token material.
- **Route Candidate Reference**: A model route entry under `openai_subscription_credential_ids` that may point to active, failed, disabled, deleted, or missing credentials.
- **Proactive Refresh Job Result**: Counts of scanned, refreshed, skipped, locked, failed, and reconnect-required credentials for logs/tests.

## Out of Scope

- Replacing the OpenAI OAuth connect flow.
- Refreshing every credential on every tick.
- Changing the 5-minute request-time freshness buffer.
- Introducing real OpenAI network calls in tests.
- Re-enabling credentials whose OpenAI session has ended.
- Disabling API-key routes or changing non-OpenAI routing.
- Committing UI mock screenshots in this Todo planning branch.

## Success Criteria _(mandatory)_

### Measurable Outcomes

- **SC-001**: A credential expiring within 30 minutes is refreshed by the job before any route selects it.
- **SC-002**: A credential expiring outside 30 minutes causes zero refresh HTTP requests during a job tick.
- **SC-003**: Concurrent proactive jobs and request-time resolution produce at most one upstream refresh request for the same credential.
- **SC-004**: `refresh_token_invalidated` stores reconnect-required metadata and makes the credential non-selectable.
- **SC-005**: Route resolution no longer repeatedly attempts credentials already marked failed, disabled, deleted, or missing.
- **SC-006**: Operator-facing UI/action text clearly distinguishes reconnect-required from transient refresh failure.
- **SC-007**: Redaction tests prove access token, refresh token, bearer/JWT-like text, and encrypted blobs are absent from metadata/log/UI response assertions.

## Dependencies

- Existing refresh manager and `singleflight` behavior in `internal/proxy/handler/openai_subscription_refresh.go`.
- Existing route resolution and sticky/lowest-utilization selection in `internal/proxy/handler/openai_subscription_routing.go`.
- Existing lifecycle UI/action metadata redaction in `internal/proxy/handler/openai_subscription_lifecycle.go`, `internal/ui/handler_credentials.go`, and `internal/proxy/handler/credentials.go`.
- Existing scheduler and optional Redis distributed lock wrapper in `internal/scheduler`.
- Existing sqlc credential query layer in `internal/db/queries/credential.sql`.
