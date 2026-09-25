# Specification: HO-1340 Codex usage snapshot for OpenAI subscription credentials

## Scope

Add a best-effort Codex usage snapshot for Tianji OpenAI subscription credentials.

This issue owns the Tianji server-side fetch/normalization/cache path, a safe UI exposure on OpenAI subscription credential detail, and a new `/ui/usage` Codex tab. It does not change proxy request routing semantics, does not persist token material or raw ChatGPT response bodies, and does not make `chatgpt.com/backend-api/wham/usage` a billing source of truth.

## Problem

Tianji can store and refresh OpenAI subscription credentials, and the ChatGPT Codex backend transport already uses refreshed subscription access tokens for Codex requests. Operators currently cannot see the Codex usage windows tied to those subscription credentials inside Tianji.

Investigation showed a refreshed Tianji subscription credential can call `GET https://chatgpt.com/backend-api/wham/usage` successfully and receive Codex usage buckets such as 5-hour primary usage, weekly usage, and additional model buckets.

Root cause chain: Tianji resolves and refreshes OpenAI subscription credentials for Codex transport, but no Tianji component fetches or normalizes the Codex usage snapshot; therefore the UI can show generic quota gate state from proxy traffic, but not the Codex-specific usage windows operators need for credential monitoring.

## User Stories

### US1 - Credential detail shows Codex usage snapshot (P1)

As a Tianji operator, I want an OpenAI subscription credential detail page to show the latest safe Codex usage snapshot so I can understand whether that credential is near Codex limits before routing more Codex traffic through it.

**Independent Test**: handler/UI tests seed a successful normalized snapshot for one credential and assert `/ui/credentials/{credential_id}` renders email, plan type, primary 5h, weekly, additional bucket, status/reset, and last-updated metadata without any token/raw body fields.

### US2 - Manual refresh reuses subscription refresh flow (P1)

As a Tianji operator, I want manual refresh to use Tianji's existing OpenAI subscription credential resolution and refresh behavior so expired access tokens recover without adding a second decrypt/refresh path.

**Independent Test**: provider/helper tests simulate an initial 401 from `wham/usage`, assert Tianji refreshes the credential once through `resolveUsableOpenAISubscriptionBundle` / force-refresh helper, retries once with the refreshed token, and stores only normalized safe fields.

### US3 - Codex usage tab summarizes all subscription credentials (P1)

As a Tianji operator, I want `/ui/usage` to have a Codex tab showing all OpenAI subscription credentials with current Codex usage windows so I can compare capacity across credentials.

**Independent Test**: UI tests assert `/ui/usage?tab=codex` and `/ui/usage/tab?tab=codex` render one card/row per OpenAI subscription credential, with stale/error/no-data states handled per credential.

### US4 - Cache and backoff protect the unofficial endpoint (P1)

As a Tianji maintainer, I want Codex usage fetches to be operator-initiated or UI/status-driven with cache and backoff, not part of every proxy request.

**Independent Test**: tests make repeated UI/API/manual refresh calls inside the TTL and assert only one upstream call occurs; tests for 429/5xx assert exponential backoff with jitter and last-success preservation.

### US5 - Secret and raw-response safety is enforced (P1)

As a Tianji maintainer, I want no access token, refresh token, cookie, authorization header, or raw upstream body to be persisted, rendered, logged, or returned by the usage snapshot path.

**Independent Test**: redaction tests inject token-shaped strings into upstream body/error/header fixtures and assert DB rows, JSON responses, logs/audit metadata, and rendered HTML contain only stable normalized fields or redacted error/status codes.

## Functional Requirements

- **FR-001**: Tianji MUST add a provider/helper near `internal/provider/chatgptcodex` for `GET https://chatgpt.com/backend-api/wham/usage`.
- **FR-002**: The helper MUST send `Authorization: Bearer <access_token>`.
- **FR-003**: The helper MUST send `ChatGPT-Account-Id` when the resolved subscription bundle has an account id.
- **FR-004**: The helper MUST send Codex usage referer/originator-compatible headers needed by the endpoint, using the existing ChatGPT Codex backend config defaults where applicable.
- **FR-005**: Fetching MUST reuse existing OpenAI subscription credential resolution/refresh behavior and MUST NOT add a separate token decrypt path.
- **FR-006**: A 401 response MUST trigger one credential refresh and exactly one retry before returning a safe auth error state.
- **FR-007**: 429 and 5xx responses MUST set per-credential exponential backoff with jitter.
- **FR-008**: Successful snapshots MUST be cached per `credential_id` for at least 60 seconds.
- **FR-009**: Manual refresh MUST respect active backoff and MUST either return the cached last successful snapshot or a safe status explaining refresh is deferred.
- **FR-010**: Snapshot fetching MUST NOT happen on every proxy request.
- **FR-011**: Snapshot fetching MAY run from credential detail, `/ui/usage` Codex tab, explicit manual refresh, or a future background health/status job.
- **FR-012**: Tianji MUST normalize and expose only non-secret fields: credential id, email, plan type, snapshot status, primary 5h usage/reset/status, weekly usage/reset/status, additional bucket usage/reset/status/name, credit/spend-control status when present, fetched-at, stale/backoff metadata, and safe error reason.
- **FR-013**: Tianji MUST NOT persist or expose access tokens, refresh tokens, cookies, authorization headers, raw response bodies, raw request headers, encrypted credential values, or token-shaped upstream errors.
- **FR-014**: Credential detail UI MUST show the credential's Codex usage snapshot below/near existing quota dimensions.
- **FR-015**: `/ui/usage` MUST add a `codex` tab similar in spirit to the existing Claude Code tab, scoped to OpenAI subscription credentials.
- **FR-016**: `/ui/usage/tab?tab=codex` MUST support HTMX tab loading.
- **FR-017**: A manual refresh action MUST exist for credential detail and Codex tab.
- **FR-018**: Missing, disabled, malformed, wrong-type, or DB-unavailable credentials MUST render safe per-credential no-data/error states without panics or secret leakage.
- **FR-019**: The normalized parser MUST tolerate missing optional buckets and unknown additional bucket names without dropping known primary/weekly values.
- **FR-020**: Regression coverage MUST prove no upstream `wham/usage` call is made during normal proxy request handling.

## Non-Goals

- Do not make `wham/usage` a public/stable OpenAI API contract.
- Do not use Codex usage snapshot as billing truth.
- Do not change `/v1/chat/completions`, `/v1/responses`, WebSocket generation, or Codex payload behavior.
- Do not change OpenAI subscription credential CRUD, OAuth connect, or model refresh semantics except for reusing existing refresh helpers.
- Do not persist raw ChatGPT usage responses for later replay.
- Do not call the endpoint per proxied model request.
- Do not add cookies as a required stored credential field.
- Do not broaden Claude Code usage tab behavior.

## Success Criteria

- **SC-001**: Credential detail displays safe Codex usage data for a refreshed OpenAI subscription credential.
- **SC-002**: `/ui/usage?tab=codex` displays all OpenAI subscription credentials with safe per-credential usage/error/stale states.
- **SC-003**: Repeated reads inside the TTL reuse cached data.
- **SC-004**: 401 refresh-once retry succeeds when refresh returns a usable token.
- **SC-005**: 429/5xx responses preserve the last successful snapshot and activate backoff.
- **SC-006**: Tests prove proxy request paths do not invoke the usage endpoint.
- **SC-007**: Redaction tests prove token/raw body material is absent from persistence, JSON, HTML, and logs.

## Open Questions

None for Todo scope. Implementation may tune the exact normalized field names after inspecting the current `wham/usage` JSON fixture, but the scope and safety constraints are locked.
