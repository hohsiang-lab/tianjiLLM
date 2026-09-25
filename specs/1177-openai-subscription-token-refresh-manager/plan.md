# Implementation Plan: OpenAI Subscription Token Refresh Manager

**Branch**: `HO-1177-openai-subscription-token-refresh-manager`
**Date**: 2026-05-07
**Spec**: [spec.md](spec.md)
**Input**: Linear HO-1177

## Summary

Implement an on-demand OpenAI subscription refresh manager inside the existing handler credential-resolution boundary. The manager loads encrypted `openai_subscription` credentials, reuses fresh access tokens, refreshes inside a 5-minute expiry buffer, suppresses concurrent same-credential refreshes with `singleflight`, persists rotated refresh tokens through existing HO-1176 helpers, and returns safe typed errors through the HO-1183 redaction boundary.

## Technical Context

**Language/Version**: Go 1.26 module in TianjiLLM
**Primary Dependencies**: Existing `net/http`, `encoding/json`, `time`, `golang.org/x/sync/singleflight v0.19.0` (currently indirect), existing `golang.org/x/oauth2 v0.35.0` as behavior reference, existing sqlc credential queries
**Storage**: Existing `CredentialTable` with encrypted `credential_value` and JSONB `credential_info`
**Testing**: Go unit/handler/provider tests using `httptest` and `internal/testutil/openaitest`; no real OpenAI network
**Target Platform**: Linux server / Docker / Kubernetes
**Performance Goals**: No refresh call for fresh tokens; one refresh call per credential for concurrent near-expiry requests
**Constraints**: Todo phase is docs-only; implementation must start with failing tests; no secrets in logs/responses/metadata
**Scale/Scope**: One stored subscription account per credential ID; on-demand refresh in current handler process

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| Repo reality first | PASS | Current resolver rejects expired credentials and already has HO-1176 persistence helpers to reuse. |
| Research before build | PASS | research.md records repo, OpenAI/Codex source/docs, Context7, pkg.go.dev, and grep-app failure. |
| Failing-tests-first | PASS | tasks.md starts each user story with failing tests. |
| sqlc-first DB access | PASS | Existing sqlc `GetCredential`, `UpdateCredentialValueAndInfo`, and `UpdateCredentialInfo` are enough unless implementation proves otherwise. |
| No real secrets/network | PASS | Tests must use synthetic token fixtures and `openaitest.NewGuardedClient`. |
| No production code in Todo | PASS | This branch is planning artifacts only until Linear moves to In Progress. |

## Project Structure

### Documentation

```text
specs/1177-openai-subscription-token-refresh-manager/
+-- spec.md
+-- plan.md
+-- research.md
+-- data-model.md
+-- quickstart.md
+-- analyze.md
+-- contracts/
|   +-- refresh-manager.md
+-- checklists/
|   +-- requirements.md
+-- tasks.md
```

### Source Code (future implementation scope; not written in Todo state)

```text
internal/
+-- provider/openai/
|   +-- oauth.go                         # MODIFY: add refresh-token grant helper
|   +-- oauth_test.go                    # MODIFY: request shape, parsing, redacted errors
+-- proxy/handler/
|   +-- handler.go                       # MODIFY: add refresh singleflight/dependency fields if needed
|   +-- openai_subscription_resolution.go # MODIFY: use refresh manager before returning credential
|   +-- openai_subscription_refresh.go    # NEW: manager, typed errors, testable clock/buffer
|   +-- openai_subscription_resolution_test.go
|   +-- openai_subscription_refresh_test.go
+-- testutil/openaitest/
    +-- oauth_server.go                  # READ/REUSE: refresh-token fixtures already supported
```

**Structure Decision**: Keep refresh manager near `internal/proxy/handler/openai_subscription_resolution.go` because it needs handler DB, master key, config, redaction, and existing credential update helpers. Add provider-level `RefreshToken` helper beside `ExchangeCode` so request shape and endpoint override tests stay in `internal/provider/openai`.

## Data Flow

```text
resolveOpenAISubscriptionCredential(ctx, params, transport)
  -> load CredentialTable row by configured credential_id
  -> reject missing/wrong type/disabled/malformed with typed safe errors
  -> decrypt OpenAISubscriptionTokenBundle
  -> if expires_at > now + refreshBuffer:
       return direct bearer or Codex app-server login payload
  -> else:
       refreshGroup.Do(credential_id, refresh)

refresh(credential_id)
  -> reload latest credential row inside singleflight function
  -> if another caller already refreshed and token is now fresh:
       return fresh bundle without upstream call
  -> require refresh_token
  -> POST token URL with grant_type=refresh_token, refresh_token, client_id
  -> parse access_token, optional refresh_token, expires_in, scope/raw metadata
  -> UpdateOpenAISubscriptionCredential(...)
  -> return refreshed bundle

refresh failure
  -> redact upstream/network/parse error
  -> UpdateOpenAISubscriptionCredentialFailure(...)
  -> return OpenAISubscriptionCredentialError{Code: refresh_failed}
```

## Repo Evidence

- `internal/proxy/handler/openai_subscription_resolution.go` currently decrypts and validates the bundle but returns `credential expired` when `ExpiresAt` is not after `time.Now()`.
- `resolveOpenAIAPIKeyForParams` is already the direct HTTP bearer boundary, while `openAISubscriptionTransportCodexAppServer` builds `chatgptAuthTokens` payloads from the stored access token.
- `internal/proxy/handler/credentials.go` already has `UpdateOpenAISubscriptionCredential` and `UpdateOpenAISubscriptionCredentialFailure`; these preserve/overwrite refresh tokens and update metadata.
- `internal/provider/openai/oauth.go` currently supports authorization-code exchange only; refresh-token grant needs a sibling helper.
- `internal/testutil/openaitest/oauth_server.go` already records `grant_type=refresh_token` requests and returns configured rotated token fixtures.
- `go.mod` already includes `golang.org/x/oauth2 v0.35.0` directly and `golang.org/x/sync v0.19.0` indirectly.

## External Evidence

- OpenAI Codex app-server README documents `account/read.refreshToken` and says managed ChatGPT auth persists and refreshes tokens automatically: <https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md#auth-endpoints>
- OpenAI Codex app-server protocol source says external auth ignores `account/read.refresh_token`; clients should refresh tokens themselves and call `account/login/start` with `chatgptAuthTokens`: <https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/src/protocol/v2/account.rs>
- OpenAI Codex login source persists `access_token`, `refresh_token`, `account_id`, and `last_refresh` after OAuth exchange: <https://github.com/openai/codex/blob/main/codex-rs/login/src/server.rs>
- OpenAI API key safety guidance says keys should stay server-side, should not be committed/shared, and should be rotated when compromised: <https://help.openai.com/en/articles/5112595-best-practices-for-api-key-safety>
- Context7 `/websites/pkg_go_dev_golang_org_x_sync` confirms `singleflight.Group.Do` suppresses duplicate in-flight calls per key and shares the result with duplicate callers.
- Context7 `/golang/oauth2` confirms `ReuseTokenSourceWithExpiry` uses an early-expiry buffer and refreshes before expiry; this plan adopts the buffer behavior while keeping Tianji's existing custom OpenAI request helper.
- pkg.go.dev confirms `singleflight` provides duplicate function call suppression and `oauth2` exposes `ReuseTokenSourceWithExpiry`.
- grep-app public GitHub searches for `singleflight.Group` and `grant_type", "refresh_token"` failed with transport `Unexpected content type: text/html`; no public pattern was adopted from grep-app.

## Failing Tests

| Test Function | File | Initial Failure | Covers |
|---------------|------|-----------------|--------|
| `TestOpenAIRefreshToken_PublicClientRequestShape` | `internal/provider/openai/oauth_test.go` | refresh helper missing | FR-004, FR-005 |
| `TestOpenAIRefreshToken_ParsesRotatedRefreshToken` | `internal/provider/openai/oauth_test.go` | refresh helper missing | FR-006, FR-008 |
| `TestOpenAIRefreshToken_ErrorRedactsRequestSecrets` | `internal/provider/openai/oauth_test.go` | refresh helper missing | FR-013 |
| `TestResolveOpenAISubscriptionCredential_FreshTokenSkipsRefresh` | `internal/proxy/handler/openai_subscription_refresh_test.go` | manager missing | FR-002 |
| `TestResolveOpenAISubscriptionCredential_RefreshesBeforeExpiryBuffer` | `internal/proxy/handler/openai_subscription_refresh_test.go` | expired-only behavior | FR-003 |
| `TestResolveOpenAISubscriptionCredential_ConcurrentRefreshSingleflight` | `internal/proxy/handler/openai_subscription_refresh_test.go` | no per-credential suppression | FR-011 |
| `TestResolveOpenAISubscriptionCredential_RefreshPersistsRotatedToken` | `internal/proxy/handler/openai_subscription_refresh_test.go` | manager missing | FR-007, FR-008 |
| `TestResolveOpenAISubscriptionCredential_RefreshFailurePersistsRedactedMetadata` | `internal/proxy/handler/openai_subscription_refresh_test.go` | manager missing | FR-010, FR-013 |
| `TestResolveOpenAISubscriptionCredential_TypedFailureCodes` | `internal/proxy/handler/openai_subscription_refresh_test.go` | typed error missing | FR-012 |
| `TestResolveOpenAISubscriptionCredential_CodexUsesRefreshedToken` | `internal/proxy/handler/openai_subscription_resolution_test.go` | current Codex payload uses stale token | FR-015 |

### Verification Commands

```bash
go test ./internal/provider/openai/... -run 'TestOpenAIRefreshToken' -v
go test ./internal/proxy/handler/... -run 'TestResolveOpenAISubscriptionCredential_.*Refresh|TestResolveOpenAISubscriptionCredential_Typed|TestResolveOpenAISubscriptionCredential_CodexUsesRefreshedToken' -v
go test ./internal/provider/openai/... ./internal/proxy/handler/... ./internal/testutil/openaitest/... -v
```

## Phase 1: Tests First

Write provider refresh-helper tests and handler refresh-manager tests before production changes. Expected initial failures are missing provider helper, expired-only resolver behavior, no refresh manager, no typed errors, and no concurrent suppression.

## Phase 2: Provider Refresh Helper

Add `RefreshToken(ctx, client, cfg, refreshToken)` in `internal/provider/openai/oauth.go` mirroring `ExchangeCode`:

- resolve endpoint/client defaults through `config.ResolveOpenAIOAuthConfig`
- post `application/x-www-form-urlencoded`
- include `grant_type=refresh_token`, `refresh_token`, and `client_id`
- never include `client_secret` or Basic auth
- parse response into the existing `TokenBundle`
- redact response/request secrets from returned errors

## Phase 3: Refresh Manager

Add a handler-local manager path:

- default refresh buffer: `5 * time.Minute`
- testable clock function or small dependency struct
- typed `OpenAISubscriptionCredentialError` with stable `Code`
- per-handler `singleflight.Group` keyed by `credential_id`
- reload credential inside the singleflight function before network call to avoid refreshing again after a prior caller already persisted a fresh token

## Phase 4: Persistence and Metadata

Reuse existing helpers:

- `UpdateOpenAISubscriptionCredential` for successful refresh, preserving/rotating refresh token as already implemented
- `UpdateOpenAISubscriptionCredentialFailure` for failure metadata only
- HO-1183 `redact.String` / credential metadata sanitization for upstream failure text
- safe metadata: `status=active`, `last_refresh_at`, optional `last_error`, optional `disabled_reason`

## Phase 5: Resolver Integration

Replace the current hard `credential expired` branch with:

- fresh path returns current bundle
- refresh-needed path calls manager
- direct HTTP receives refreshed `BearerToken`
- Codex app-server receives refreshed `CodexLogin`
- no-subscription and API-key paths remain unchanged

## Phase 6: Verify and Governance

- Run targeted provider and handler tests.
- Run affected package tests.
- Confirm no real OpenAI host is called by using `openaitest.NewGuardedClient`.
- Confirm `git diff --name-only origin/main...HEAD` contains only HO-1177 files during Todo planning.

## Risk Register

| Risk | Mitigation |
|------|------------|
| Refresh token stampede invalidates rotated token | Use per-credential `singleflight.Group` and reload credential inside the refresh function. |
| Transient refresh failure disables a usable account | Persist `refresh_failed` metadata, not `disabled`, unless a later explicit terminal-failure policy exists. |
| Upstream error leaks token material | Redact all error strings before returning or persisting. |
| Existing API-key fallback masks subscription failures | Preserve HO-1167 no-fallback behavior once subscription IDs are configured. |
| Multiple process instances still refresh concurrently | Document current in-process guarantee; add DB/advisory lock only if deployment evidence requires cross-process suppression. |
| `golang.org/x/sync` is currently indirect | Importing `singleflight` will promote it to direct in go.mod; no new module download is expected. |

## Plan Review

Plan reviewed via repo evidence / Context7 / grep-app / Google-web. Revisions applied: use `singleflight.Group` rather than a custom map of mutexes; use an explicit 5-minute expiry buffer following oauth2 early-expiry behavior; keep custom provider refresh helper rather than `oauth2.TokenSource` to preserve OpenAI endpoint overrides and existing raw metadata parsing. grep-app failed, so no public GitHub implementation was adopted.

## Todo Gate Status

Spec/plan/tasks/analyze are complete and ready for draft PR review. Todo -> Waiting is allowed after draft PR creation.
