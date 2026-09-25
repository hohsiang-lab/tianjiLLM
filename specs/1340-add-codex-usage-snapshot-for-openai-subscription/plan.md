# Implementation Plan: HO-1340 Codex usage snapshot

## Goal

Expose a safe, cached Codex usage snapshot for OpenAI subscription credentials using Tianji's existing credential resolution and refresh path.

The implementation should add one normalized usage surface:

- credential detail: per-credential snapshot
- `/ui/usage` Codex tab: cross-credential snapshot list
- manual refresh: explicit operator action respecting cache/backoff

## Constraints

- Todo phase is docs-only. No production code in this PR.
- `chatgpt.com/backend-api/wham/usage` is unofficial and best-effort; do not present it as OpenAI billing truth.
- Do not persist access tokens, refresh tokens, cookies, raw response bodies, authorization headers, or encrypted credential values.
- Do not fetch usage on every proxy request.
- Reuse existing OpenAI subscription credential resolution/refresh helpers; do not add a second decrypt path.
- Prefer redaction-safe normalized structs over generic `map[string]any` storage.

## Repo Reality

- `internal/provider/chatgptcodex/transport.go`
  - Owns ChatGPT Codex backend base URL defaults.
  - Already applies `Authorization: Bearer ...`, `originator`, `OpenAI-Beta`, and `ChatGPT-Account-Id` for Codex backend requests.
- `internal/proxy/handler/openai_subscription_resolution.go`
  - Defines `resolvedOpenAISubscriptionCredential` with `CredentialID`, `BearerToken`, and `AccountID`.
  - `resolveOpenAISubscriptionCredentialByID` calls `resolveUsableOpenAISubscriptionBundle`.
  - ChatGPT Codex backend transport receives refreshed bearer token through existing resolution.
- `internal/proxy/handler/openai_subscription_routing.go`
  - Contains retry/backoff-adjacent concepts for OpenAI subscription credential usability and rate limit gating.
  - Current route selection must not depend on the new snapshot path unless future scope explicitly changes that.
- `internal/proxy/handler/credentials.go`
  - Saves encrypted OpenAI subscription token bundles.
  - `marshalSafeCredentialInfo` rejects secret credential_info fields and redacts metadata.
- `internal/ui/handler_credentials.go`
  - `handleCredentialDetail` and `buildCredentialDetail` render OpenAI subscription credential detail.
  - Current quota display uses `h.openAIQuotaState(credentialID, now)` from `RateLimitStore`.
- `internal/ui/handler_usage.go`
  - `activeTab` currently allows `cost`, `model-activity`, `key-activity`, `endpoint-activity`, and `claude-code`.
  - `handleUsage` / `handleUsageTab` switch per tab.
- `internal/ui/handler_claude_code.go` and `internal/ui/pages/usage_claude_code.templ`
  - Existing pattern for a plan-usage tab, per-token cards, stale/no-data states, and manual refresh-style endpoint.
- `internal/ui/routes.go`
  - Current usage route group already includes `/ui/usage`, `/ui/usage/tab`, and `/ui/api/claude-code-usage`.

## External Evidence

- Official OpenAI Codex docs describe Codex as OpenAI's coding agent and expose settings/usage surfaces to users, but do not document `chatgpt.com/backend-api/wham/usage` as a stable public API.
- Official OpenAI API docs use bearer-token authorization for API requests; Tianji must keep using standard bearer semantics for the refreshed access token.
- Public issue investigation for HO-1340 found no `X-RateLimit-*` or `Retry-After` headers on live `wham/usage` responses, so local TTL/backoff is required.
- OpenAI Codex app-server/CLI ecosystem has a live usage/rate-limit read pattern, but Tianji's direct `wham/usage` call remains best-effort and must be isolated behind tests and cache.

## Proposed Design

### 1. Provider helper

Add a small helper under `internal/provider/chatgptcodex`, for example `usage.go`, with explicit types:

- `UsageClient`
- `UsageRequest`
- `UsageSnapshot`
- `UsageWindow`
- `UsageBucket`
- safe `UsageError` / reason code type

Responsibilities:

- Build `GET /wham/usage` URL from `Transport.BaseURL` defaulting to `https://chatgpt.com/backend-api`.
- Apply authorization and account headers.
- Apply Codex usage referer/originator-compatible headers.
- Decode known fields into normalized structs.
- Preserve unknown additional bucket names as normalized bucket entries.
- Never return raw response body to callers except inside local test-only assertions.

### 2. Handler/service layer

Add a service near OpenAI subscription lifecycle handlers, for example `openai_subscription_usage.go`.

Responsibilities:

- Resolve a credential by id through existing subscription helpers.
- Fetch usage with the provider helper.
- On 401: force-refresh once, retry once.
- On 429/5xx: set backoff and preserve last successful snapshot.
- On missing/disabled/malformed credential: return safe state.
- Centralize cache and backoff state by `credential_id`.

The service should be injectable/testable with:

- clock
- jitter source or deterministic test hook
- HTTP client / usage client
- cache store

### 3. Cache/store strategy

Use an in-memory per-process cache first unless implementation discovers an existing durable status-store pattern that fits better.

State per credential:

- `credential_id`
- `snapshot`
- `fetched_at`
- `expires_at` with TTL >= 60s
- `backoff_until`
- `backoff_attempt`
- `last_error_reason`
- `last_success_at`

Do not store raw upstream JSON. If future persistence is added, it must use only the normalized safe state.

### 4. UI integration

Credential detail:

- Extend `CredentialDetailData` with Codex usage view data.
- Render a Codex usage section near existing quota dimensions.
- Include manual refresh action targeting the detail content.

Usage page:

- Add `codex` to `activeTab`.
- Add `CodexUsageTabData` and `UsageCodexTab`.
- Render one card/row per OpenAI subscription credential from DB.
- Include per-credential manual refresh.
- Show stale, backoff, disabled, missing data, and last-updated states per credential.

### 5. API/HTMX endpoints

Add minimal internal UI endpoints:

- `GET /ui/api/codex-usage` or tab-render path for manual refresh JSON/HTML, matching existing UI conventions.
- `POST /ui/credentials/{credential_id}/codex-usage/refresh` or equivalent HTMX action if the existing credential action pattern is clearer.

Endpoint naming can follow repo conventions during implementation, but actions must remain UI/admin protected.

### 6. Tests

Write failing tests before implementation:

- provider helper request/header/parse tests using `httptest`.
- service tests for cache TTL, 401 refresh-once, 429/5xx backoff, no raw persistence.
- credential detail UI tests.
- `/ui/usage` Codex tab tests.
- no-proxy-request-fetch regression tests.
- redaction tests across persistence, JSON, HTML, logs/audit metadata.

Expected local gates:

```bash
go test ./internal/provider/chatgptcodex -run 'Usage|Wham|CodexUsage' -count=1
go test ./internal/proxy/handler -run 'OpenAISubscription.*Usage|CodexUsage|CredentialDetail' -count=1
go test ./internal/ui -run 'CodexUsage|CredentialDetail|UsageTab' -count=1
go test ./internal/ui/pages -run 'CodexUsage|Usage' -count=1
git diff --check
```

If templ files change:

```bash
make ui
```

## Plan Review

- Repo reality checked first: existing credential resolution already supplies refreshed `BearerToken` and `AccountID`; existing UI has credential detail and Claude Code tab patterns.
- Official docs checked for OpenAI/Codex public positioning and bearer-token semantics; no official stable `wham/usage` API contract was found.
- Issue investigation provides the live endpoint behavior and missing response rate-limit headers, so TTL/backoff must be local.
- No owner input needed for Todo scope.
