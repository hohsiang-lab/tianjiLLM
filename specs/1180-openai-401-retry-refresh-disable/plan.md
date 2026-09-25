# Implementation Plan: OpenAI 401 Retry, Refresh Failure, and Disable Behavior

**Branch**: `HO-1180-openai-401-retry-refresh-disable`
**Date**: 2026-05-07
**Spec**: [spec.md](spec.md)
**Linear**: HO-1180

## Summary

Extend the existing OpenAI subscription provider-attempt loop so authentication failures are handled as a credential lifecycle event. For each subscription credential attempt, a pre-body upstream 401 should force-refresh that credential once, replay the same request once with the refreshed bearer token, then mark the credential unavailable and continue to the next configured candidate if the retry still returns 401. The implementation should reuse HO-1177 refresh persistence, HO-1179 candidate/failover ordering, and HO-1183 redaction.

## Technical Context

**Language/Version**: Go
**Relevant packages**: `internal/proxy/handler`, `internal/provider/openai`, `internal/testutil/openaitest`, `internal/security/redact`
**Existing routing**: HO-1179 added `resolveOpenAISubscriptionAttemptOrder`, `doOpenAISubscriptionProviderRequest`, `openAISubscriptionFailoverTransport`, and OpenAI rate-limit state.
**Existing refresh**: HO-1177 added `refreshOpenAISubscriptionCredential`, `resolveUsableOpenAISubscriptionBundle`, typed credential errors, and `singleflight.Group`.
**Existing metadata/redaction**: HO-1176/HO-1183 provide `UpdateOpenAISubscriptionCredential`, `UpdateOpenAISubscriptionCredentialFailure`, safe `credential_info`, and `redact.String`.
**Testing**: Go handler tests with `httptest`, `openaitest.OAuthServer`, and no-real-OpenAI guards.
**Constraints**: Todo phase is docs-only. Implementation must start with failing tests. No raw secret material in errors, logs, metadata, or test output.

## Current Repo Findings

- `internal/proxy/handler/openai_subscription_provider_retry.go` currently retries only network errors, 429, and 5xx across candidates; it does not treat 401 as refreshable.
- `openAISubscriptionRetryableStatus` currently excludes 401.
- `doOpenAISubscriptionProviderRequest` receives candidate `CredentialID` and `BearerToken`, so it has enough candidate identity to invoke a forced refresh path, but it currently has no forced-refresh helper.
- `openAISubscriptionFailoverTransport` wraps proxy-style endpoints and already rebuilds request bodies for each candidate; it needs the same 401 forced-refresh behavior.
- `resolveUsableOpenAISubscriptionBundle` refreshes only when the local expiry buffer says the token is stale; a 401 requires a new explicit forced-refresh path because upstream may reject a locally fresh token.
- `recordOpenAISubscriptionRefreshFailure` persists `status=refresh_failed`, `last_error`, and `disabled_reason=refresh_failed`.
- `loadOpenAISubscriptionCredential` excludes metadata with `status=disabled`; post-refresh 401 should use that existing exclusion path or an equivalent unavailable state.
- Endpoint tests in `openai_subscription_endpoints_test.go` already cover failover for 429 and 5xx, plus no retry for ordinary 4xx. HO-1180 should revise the 401 case from "ordinary 4xx" to "auth refresh path" only for subscription candidates.

## External Evidence

- OpenAI official error-code docs state 401 includes invalid authentication, incorrect API key, organization membership, and IP allowlist failures; `AuthenticationError` means the API key or token was invalid, expired, or revoked.
- Context7 `/openai/openai-go` docs say the official Go client automatically retries connection errors, 408, 409, 429, and 5xx by default. It does not list 401 as a generic transport retry.
- OpenAI official docs advise retrying 500/503 and pacing/backoff for 429. This supports keeping HO-1179's 429/5xx behavior separate from HO-1180's credential-refresh-specific 401 behavior.
- grep-app public GitHub searches for Go 401 and refresh-token examples failed with transport `Unexpected content type: text/html`; no public implementation pattern was adopted.

## Constitution Check

| Principle | Status | Notes |
| --- | --- | --- |
| Repo reality first | PASS | Plan is anchored in current handler retry, refresh, and metadata code. |
| Research before build | PASS | Official OpenAI docs, Context7, repo evidence, and grep-app result are recorded. |
| Failing tests first | PASS | tasks.md starts with auth-refresh/failover tests before implementation. |
| No production code in Todo | PASS | This PR is planning artifacts only. |
| Redaction discipline | PASS | Plan routes all auth failure text through HO-1183 redaction. |
| Bounded retries | PASS | One forced refresh and one retry per credential per request. |

## Project Structure

### Documentation

```text
specs/1180-openai-401-retry-refresh-disable/
+-- spec.md
+-- plan.md
+-- research.md
+-- data-model.md
+-- quickstart.md
+-- analyze.md
+-- contracts/
|   +-- openai-401-refresh-failover.md
+-- checklists/
|   +-- requirements.md
+-- tasks.md
```

### Source Code, Future Implementation Scope

```text
internal/
+-- proxy/handler/
|   +-- openai_subscription_provider_retry.go       # MODIFY: direct handler 401 forced refresh/retry/failover
|   +-- assistants.go                               # MODIFY: proxy transport 401 forced refresh/retry/failover
|   +-- openai_subscription_refresh.go              # MODIFY: add forced refresh boundary
|   +-- credentials.go                              # MODIFY if auth-failed metadata needs helper
|   +-- openai_subscription_endpoints_test.go       # MODIFY: 401 refresh/retry/failover endpoint matrix
|   +-- openai_subscription_refresh_test.go         # MODIFY: forced refresh and metadata tests
|   +-- openai_subscription_resolution_test.go      # MODIFY: disabled-after-auth failure exclusion regression
|   +-- redaction_sinks_test.go                     # MODIFY if new metadata/error sink coverage belongs there
```

## Proposed Data Flow

```text
Client request
  -> route resolves official OpenAI subscription candidates in HO-1179 order
  -> attempt credential A with current bearer token
  -> upstream returns 401 before body is committed
  -> force-refresh credential A once
       -> reload stored credential
       -> use refresh_token grant even if access token is locally fresh
       -> persist successful rotated token metadata, or refresh_failed metadata on failure
  -> if refresh succeeds:
       -> retry original upstream request once with refreshed bearer
       -> if retry succeeds, return response
       -> if retry returns 401, persist auth_failed_after_refresh/disabled metadata
  -> if refresh fails or retry still returns 401:
       -> try next configured candidate
  -> after all candidates fail:
       -> return sanitized reauthorization-required error
```

## Design Decisions

### 401 is not a generic retryable status

Do not add 401 to `openAISubscriptionRetryableStatus`. Generic retry would replay the same stale token or incorrectly apply to API-key routes. Add a subscription-aware auth-failure branch that can refresh credential material before replay.

### Forced refresh is separate from freshness-based refresh

`resolveUsableOpenAISubscriptionBundle` should keep the normal expiry-buffer logic. HO-1180 needs a new forced path because upstream 401 is stronger evidence than local `expires_at`. The forced path should still reuse the same persistence and redaction helpers.

### Same-credential retry is capped

Each credential attempt may produce at most:

1. initial upstream request with current bearer
2. one forced refresh
3. one replay with refreshed bearer

After that, the credential is unavailable for this client request and for later routing if persisted as disabled.

### Disable metadata should use existing exclusion behavior

Current candidate loading excludes `info.Status == "disabled"`. Post-refresh 401 should write safe metadata that makes later candidate resolution skip the credential. Recommended metadata:

```json
{
  "status": "disabled",
  "last_error": "OpenAI authentication failed after forced refresh",
  "disabled_reason": "auth_failed_after_refresh"
}
```

Exact text may vary, but status/reason must be stable enough for tests and operators.

### Direct handler and proxy transport must share behavior

The repo has two attempt surfaces:

- `doOpenAISubscriptionProviderRequest` for chat/completions/embeddings/images/audio style handlers.
- `openAISubscriptionFailoverTransport` for response/proxy style endpoints.

Both must use the same 401 forced-refresh semantics or tests will pass for one endpoint family and fail for another.

## Failing Tests

### Direct handler tests

| Test Function | File | Assertion |
| --- | --- | --- |
| `TestOpenAISubscriptionRouting_401RefreshRetrySameCredentialSucceeds` | `internal/proxy/handler/openai_subscription_endpoints_test.go` | First request uses old bearer, upstream returns 401, forced refresh returns new bearer, same request retries and succeeds. |
| `TestOpenAISubscriptionRouting_401RefreshFailureFailsOver` | same | Forced refresh returns `invalid_grant`; next credential serves response and first credential stores redacted `refresh_failed` metadata. |
| `TestOpenAISubscriptionRouting_401RetryStillFailsDisablesAndFailsOver` | same | Refreshed retry also returns 401; first credential stores disabled/auth-failed metadata and second credential succeeds. |
| `TestOpenAISubscriptionRouting_AllCredentialsAuthFailedReturnsReauthError` | same | Every credential fails refresh or post-refresh auth; response is clear 401 reauth error with no API-key fallback. |

### Proxy transport tests

| Test Function | File | Assertion |
| --- | --- | --- |
| `TestOpenAISubscriptionProxyTransport_401RefreshRetrySameCredentialSucceeds` | `internal/proxy/handler/openai_subscription_endpoints_test.go` | `/v1/responses` proxy path uses same refresh/retry behavior. |
| `TestOpenAISubscriptionProxyTransport_401RetryStillFailsFailsOver` | same | Proxy transport disables failed credential and tries next candidate. |

### Refresh/metadata tests

| Test Function | File | Assertion |
| --- | --- | --- |
| `TestForceRefreshOpenAISubscriptionCredential_BypassesFreshness` | `internal/proxy/handler/openai_subscription_refresh_test.go` | Fresh local access token is refreshed when upstream 401 demands it. |
| `TestOpenAISubscriptionCredential_AuthFailureMetadataRedacted` | `internal/proxy/handler/credential_test.go` or refresh test | Stored `last_error` and `disabled_reason` contain safe reason only and no token-shaped text. |
| `TestOpenAISubscriptionRouting_401DoesNotRefreshAPIKeyPath` | endpoint test | API-key deployment 401 does not call refresh and preserves current behavior. |
| `TestOpenAISubscriptionRouting_401Preserves429And5xxFailover` | endpoint test | Existing 429/5xx failover tests still pass. |

## Verification Commands

```bash
go test ./internal/proxy/handler/... -run 'TestOpenAISubscriptionRouting_401|TestOpenAISubscriptionProxyTransport_401|TestForceRefreshOpenAISubscriptionCredential|TestOpenAISubscriptionCredential_AuthFailureMetadata' -count=1 -v
go test ./internal/proxy/handler/... -run 'TestOpenAISubscriptionRouting|TestOpenAISubscriptionEndpoints|TestResolveOpenAISubscription' -count=1 -v
go test ./internal/proxy/handler/... ./internal/provider/openai/... ./internal/testutil/openaitest/... -count=1
git diff --check origin/main...HEAD
```

## Implementation Phases

### Phase 1: Tests First

Add failing tests for direct handler and proxy transport 401 flows. Confirm failures are current behavior: one 401 response is returned without forced refresh, or no disabled/auth-failed metadata is persisted.

### Phase 2: Forced Refresh Boundary

Add a handler helper that refreshes a specific OpenAI subscription credential because upstream rejected it. It should reload the latest credential state, use existing refresh-token grant/persistence helpers, preserve rotated refresh-token behavior, and return a `resolvedOpenAISubscriptionCredential` with the new bearer.

### Phase 3: Direct Handler Retry Loop

Update `doOpenAISubscriptionProviderRequest` so a subscription candidate that receives 401 executes forced refresh once and replays the same request with the refreshed bearer. On refresh failure or second 401, record safe failure metadata and continue to the next candidate.

### Phase 4: Proxy Transport Retry Loop

Update `openAISubscriptionFailoverTransport` with the same 401 flow while preserving request body replay behavior. It must not affect Assistants-only custom upstreams without subscription candidates.

### Phase 5: Metadata and Aggregate Error

Add or reuse helper(s) for auth-failed-after-refresh metadata. Build an all-failed response that uses stable reason codes and redacted messages. Ensure response status is authentication-style, not a generic 502, when all subscription credentials need reauthorization.

### Phase 6: Regression Verification

Run targeted 401 tests, existing HO-1179 routing tests, affected package tests, and `git diff --check`.

## Risk Register

| Risk | Mitigation |
| --- | --- |
| 401 gets treated as generic retry and loops | Keep 401 out of generic retry status and use a per-credential one-shot forced-refresh branch. |
| Request body cannot be replayed | Use existing builders/body buffering and require tests across endpoint families. |
| Auth failure disables credential on transient non-auth errors | Disable only after post-refresh 401, not after 429/5xx/network errors. |
| Refresh failure writes raw upstream token text | Redact before persistence and add explicit metadata tests. |
| API-key deployments accidentally refresh | Branch only when `candidate.CredentialID` is non-empty. |
| Proxy transport and direct handlers diverge | Add tests for both surfaces. |

## Todo Gate Status

Spec/plan/tasks/analyze are complete and ready for draft PR review. Production implementation starts only after Linear HO-1180 moves to In Progress.
