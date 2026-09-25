# Implementation Plan: OpenAI Credential Test/Refresh/Disable Lifecycle APIs

**Branch**: `HO-1185-openai-credential-test-refresh-disable-lifecycle`
**Spec**: `/specs/1185-openai-credential-test-refresh-disable-lifecycle/spec.md`
**Language**: Go 1.26
**Status**: Todo planning only; production code starts only after Linear moves to In Progress。

## Technical Context

TianjiLLM already has:

- `CredentialTypeOpenAISubscription`、`OpenAISubscriptionTokenBundle`、`OpenAISubscriptionCredentialInfo` in `internal/proxy/handler/credentials.go`。
- Safe CRUD behavior from HO-1184: create/list/info/update/delete responses route through `redactCredential` and sanitize `credential_info`。
- Refresh helpers from HO-1177/HO-1180: `resolveUsableOpenAISubscriptionBundle`、`forceRefreshOpenAISubscriptionCredential`、`refreshOpenAISubscriptionCredential`、`UpdateOpenAISubscriptionCredentialFailure`。
- Disable metadata behavior from HO-1180: `recordOpenAISubscriptionAuthFailureAfterRefresh` and candidate filtering for `status=disabled`。
- Lifecycle audit helper: `auditOpenAISubscriptionLifecycle`。
- Test harness: `internal/testutil/openaitest.UpstreamServer` already supports `GET /v1/models` and guarded transports block real `api.openai.com` / `auth.openai.com`。

OpenAI official docs confirm `GET /v1/models` is a lightweight model-listing endpoint and API requests use `Authorization: Bearer ...`。

## Design Decisions

### Endpoint shape

Add lifecycle endpoints under the existing management credential surface, keeping auth/RBAC colocated with `/credentials`:

- `POST /credentials/openai-subscription/{credential_id}/test`
- `POST /credentials/openai-subscription/{credential_id}/refresh`
- `POST /credentials/openai-subscription/{credential_id}/disable`

Rationale: these actions are credential lifecycle operations, not provider request APIs, and should inherit existing proxy-admin management boundary。

### Test API upstream call

Use `GET /v1/models` through a configurable official OpenAI base URL/transport path compatible with current mock harness. The response should not expose raw model list by default; count and optional first safe model ID are enough for diagnostics。

### Refresh API

Reuse existing refresh helper instead of adding a second refresh implementation. The API should force refresh because it is explicitly operator-triggered, while still deduplicating concurrent refreshes through the existing singleflight group。

### Disable API

Disable is local metadata update only. Use `status=disabled` and `disabled_reason=operator_disabled`; preserve safe fields such as `email`、`scopes`、`last_refresh_at` where possible。

### Response contract

Use a shared redacted lifecycle response:

```json
{
  "credential_id": "cred_123",
  "action": "test",
  "status": "ok",
  "reason_code": "",
  "metadata": {
    "models_count": 3,
    "last_refresh_at": "2026-05-08T00:00:00Z"
  }
}
```

`metadata` must pass shared redaction before response/audit/persistence。

## Project Structure

Expected production files after In Progress:

- `internal/proxy/handler/openai_subscription_lifecycle.go`
- `internal/proxy/handler/openai_subscription_lifecycle_test.go`
- `internal/proxy/server.go`
- `internal/auth/rbac.go` or existing route-permission map if lifecycle paths require explicit entries
- `internal/testutil/openaitest/upstream_server.go` only if current `/v1/models` fixture needs extra failure controls

Spec-only Todo work only changes:

- `specs/1185-openai-credential-test-refresh-disable-lifecycle/*`

## Testing Strategy

- Write failing handler tests before implementation。
- Use `httptest` / `openaitest.UpstreamServer` for `/v1/models`。
- Use `openaitest.NewGuardedClient` or equivalent no-real-OpenAI transport for all lifecycle tests。
- Assert every HTTP response, stored `credential_info`, and audit payload omits token-shaped strings。
- Run targeted handler tests first, then broader affected packages。

## Failing Tests

- `TestOpenAISubscriptionLifecycleTest_FreshCredentialCallsModelsWithBearer` in `internal/proxy/handler/openai_subscription_lifecycle_test.go`: asserts `POST /credentials/openai-subscription/{credential_id}/test` calls mock `GET /v1/models` with the subscription bearer and returns only safe status/model-count metadata。
- `TestOpenAISubscriptionLifecycleTest_StaleCredentialRefreshesBeforeModels` in `internal/proxy/handler/openai_subscription_lifecycle_test.go`: asserts stale token first refreshes through the existing helper, persists the refreshed bundle, then calls `/v1/models` with the refreshed bearer。
- `TestOpenAISubscriptionLifecycleTest_UpstreamFailuresAreRedacted` in `internal/proxy/handler/openai_subscription_lifecycle_test.go`: asserts upstream 401/403/429/5xx and token-shaped error text map to stable safe `reason_code` without raw upstream body/token leakage。
- `TestOpenAISubscriptionLifecycleRefresh_ForcesRefreshAndRedactsResponse` in `internal/proxy/handler/openai_subscription_lifecycle_test.go`: asserts explicit refresh force-refreshes even fresh credentials, persists encrypted bundle / `last_refresh_at`, preserves existing refresh token when omitted, and returns no token material。
- `TestOpenAISubscriptionLifecycleRefresh_FailurePersistsRedactedMetadata` in `internal/proxy/handler/openai_subscription_lifecycle_test.go`: asserts `invalid_grant` / malformed token responses persist safe `refresh_failed` metadata and redact response/audit payloads。
- `TestOpenAISubscriptionLifecycleDisable_IdempotentLocalMetadataOnly` in `internal/proxy/handler/openai_subscription_lifecycle_test.go`: asserts disable active and already-disabled credentials both return safe success/no-op, preserve the credential row, update only local metadata, and make no upstream call。
- `TestOpenAISubscriptionLifecycle_RejectsWrongTypeAndMissingSafely` in `internal/proxy/handler/openai_subscription_lifecycle_test.go`: asserts lifecycle APIs reject `api_key`/missing credentials with safe stable errors and never decrypt or expose secret data。
- `TestOpenAISubscriptionLifecycle_AuditsAreRedacted` in `internal/proxy/handler/openai_subscription_lifecycle_test.go`: asserts `test`、`refresh`、`disable` success/failure audit payloads contain action/status/reason only and no access token、refresh token、JWT、bearer、encrypted blob、fallback API key、or raw upstream body。
- `TestOpenAISubscriptionLifecycle_DeleteCompatibilityRemainsIdempotent` in `internal/proxy/handler/credential_test.go` or `openai_subscription_lifecycle_test.go`: asserts existing `DELETE /credentials/delete/{credential_id}` remains idempotent/redacted for `openai_subscription` credentials。

## Gates

Todo planning gate:

- SpecKit artifacts are Traditional Chinese except technical identifiers。
- Diff is docs-only under `specs/1185-openai-credential-test-refresh-disable-lifecycle/`。
- `git diff --check origin/main...HEAD` passes。

## Plan Review Evidence

- OpenAI official API reference: `GET /v1/models` is the model-listing endpoint suitable for a lightweight credential test。
- OpenAI official API reference: API authentication uses HTTP `Authorization: Bearer ...` semantics。
- Repo evidence: `internal/testutil/openaitest.UpstreamServer` already supports mock `GET /v1/models` and guarded transports block real `api.openai.com` / `auth.openai.com`。
- GitHub/real-code evidence slot: lifecycle tests use normal `httptest` handler-style assertions matching the repo's existing `internal/proxy/handler/*_test.go` pattern rather than adding a new external test framework。
- Draft PR opened; Linear moves to Waiting。

Implementation gate after Linear In Progress:

- Failing tests are written and confirmed failing before production code。
- Targeted lifecycle tests pass。
- Existing OpenAI subscription CRUD/refresh/routing tests pass。
- `go test ./internal/proxy/handler/... ./internal/testutil/openaitest/... -count=1` passes。
- `go tool golangci-lint run` passes or pre-existing warnings are documented。
- `git diff --check origin/main...HEAD` passes。

## Risk Controls

- Do not duplicate token refresh logic; reuse current helpers to preserve rotation semantics。
- Do not expose `/v1/models` raw response in lifecycle response。
- Do not make disable remote-revoke semantics。
- Do not weaken existing `/credentials` auth/RBAC。
- Do not introduce real OpenAI host calls in tests。
