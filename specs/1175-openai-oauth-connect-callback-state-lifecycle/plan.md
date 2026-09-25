# Implementation Plan: OpenAI OAuth Connect/Callback State Lifecycle

**Branch**: `HO-1175-openai-oauth-connect-callback-state-lifecycle` | **Date**: 2026-05-07 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/1175-openai-oauth-connect-callback-state-lifecycle/spec.md`

## Summary

Add the backend OAuth lifecycle around existing OpenAI primitives: authenticated UI connect stores short-lived server-side state, public callback consumes that state, exchanges the code with the stored PKCE verifier, persists the token bundle for the stored organization, and renders safe readable pages for success/error outcomes.

## Technical Context

**Language/Version**: Go module in TianjiLLM
**Primary Dependencies**: Existing `net/http`, `github.com/go-chi/chi/v5`, `encoding/json`, `time`, existing `internal/cache`, existing `internal/provider/openai`, existing credential persistence helpers
**Storage**: Existing cache backend for OAuth state TTL; existing `CredentialTable` persistence through HO-1176 helpers
**Testing**: Go handler/unit tests using `httptest`, existing UI session helpers, HO-1190 `internal/testutil/openaitest`, and no real OpenAI network
**Target Platform**: Linux server / Docker / Kubernetes
**Performance Goals**: Connect/callback are low-throughput admin flows; cache get/set/delete and one token exchange dominate latency
**Constraints**: No production implementation in Todo state; state must be server-side; callback must not trust query `org_id`; callback must not require `/ui` cookie auth
**Scale/Scope**: One short-lived state record per in-flight OAuth connection

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | WARNING EXCEPTION | TianjiLLM is Go and the relevant OAuth/UI/cache code is Go-only in this repo slice. |
| II. Feature Parity | PASS | OpenAI subscription OAuth lifecycle extends existing UI/admin and credential flows without changing API-key behavior. |
| III. Research Before Build | PASS | research.md records Linear scope, repo evidence, Go OAuth2/chi docs, Context7, grep-app, and official OpenAI lookup results. |
| IV. Failing-Tests-First | PASS | Concrete failing tests are listed below and tasks start with tests. |
| V. Go Best Practices | PASS | Small state service, explicit context/cache dependency, chi route registration, no global mutable state. |
| VI. No Stale Knowledge | PASS | Mutable/code facts verified from repo; OAuth API behavior verified through Go docs and Context7. |
| VII. sqlc-First DB Access | PASS | Credential writes must reuse existing sqlc-backed HO-1176 helpers; no handwritten DB queries planned. |

## Project Structure

### Documentation

```text
specs/1175-openai-oauth-connect-callback-state-lifecycle/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── analyze.md
├── contracts/
│   └── openai-oauth-lifecycle.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### Source Code (future implementation scope; not written in Todo state)

```text
internal/
├── openaioauth/
│   ├── state_store.go                 # NEW: cache-backed state create/consume/delete
│   └── state_store_test.go            # NEW: TTL, consume-once, malformed JSON tests
├── ui/
│   ├── handler_openai.go              # NEW: authenticated /ui/openai/connect handler
│   ├── handler_openai_test.go         # NEW: session required + redirect/state tests
│   └── routes.go                      # MODIFY: register protected /openai/connect
└── proxy/
    ├── handler/
    │   ├── openai_oauth.go            # NEW: public /oauth/openai/callback handler
    │   └── openai_oauth_test.go       # NEW: success/error/consumption tests
    └── server.go                      # MODIFY: register public callback route
```

**Structure Decision**: Add a small `internal/openaioauth` package for state lifecycle because UI connect and proxy callback both need the same cache-backed record format and consume-once semantics. Keep UI session enforcement in `internal/ui`, callback/persistence orchestration in `internal/proxy/handler`, and OpenAI provider HTTP details in existing `internal/provider/openai`.

## Data Flow

```text
Authenticated UI GET /ui/openai/connect?org_id=org_123
  -> h.sessionAuth validates UI cookie
  -> validate org_id
  -> openai.GeneratePKCE + openai.GenerateState
  -> openaioauth.StateStore.Create(state, code_verifier, org_id, ttl)
  -> config.DeriveOpenAIRedirectURI(general_settings.public_base_url)
  -> openai.BuildAuthorizeURL(...)
  -> 303 redirect to configured OpenAI authorize URL

OpenAI GET /oauth/openai/callback?state=...&code=...
  -> StateStore.Consume(state)
  -> reject missing/malformed/expired state before exchange
  -> openai.ExchangeCode(..., stored code_verifier)
  -> SaveOpenAISubscriptionCredential(..., stored org_id, token bundle)
  -> render safe success page
```

## State Store Decision

Use the existing `cache.Cache` interface:

- `Set(ctx, key, value, ttl)` for state creation.
- `Get(ctx, key)` for callback lookup.
- `Delete(ctx, key)` for consume-once behavior.

The record should include `expires_at` in the JSON payload even though cache TTL exists, so stale bytes returned by any backend are still rejected defensively. The key should use a fixed prefix such as `openai_oauth_state:{state}` and must require exact state matching.

## No Schema Migration Decision

No schema migration is planned for HO-1175. State is ephemeral in cache, and durable credential storage already exists in `CredentialTable` through HO-1176. Implementation must add migration/rollback only if tests expose a real missing schema invariant.

## Failing Tests

### User Story 1 Tests - Authenticated Connect

| Test Function | File | Initial Failure | Covers |
|---------------|------|-----------------|--------|
| `TestHandleOpenAIConnect_RequiresSession` | `internal/ui/handler_openai_test.go` | route/handler missing | FR-001, FR-002 |
| `TestHandleOpenAIConnect_StoresStateAndRedirects` | `internal/ui/handler_openai_test.go` | route/handler/state store missing | FR-003..FR-007 |
| `TestHandleOpenAIConnect_RejectsMissingOrgID` | `internal/ui/handler_openai_test.go` | validation missing | FR-003 |
| `TestHandleOpenAIConnect_UsesEndpointOverride` | `internal/ui/handler_openai_test.go` | route/handler missing | FR-007, FR-018 |

### User Story 2 Tests - Callback Success and Server-Side Org

| Test Function | File | Initial Failure | Covers |
|---------------|------|-----------------|--------|
| `TestOpenAIOAuthCallback_SuccessConsumesStateAndPersistsCredential` | `internal/proxy/handler/openai_oauth_test.go` | callback route/handler missing | FR-008, FR-009, FR-013..FR-015 |
| `TestOpenAIOAuthCallback_IgnoresQueryOrgID` | `internal/proxy/handler/openai_oauth_test.go` | callback route/handler missing | FR-011, FR-012 |
| `TestOpenAIOAuthCallback_ReplayFailsAfterConsume` | `internal/proxy/handler/openai_oauth_test.go` | state consume missing | FR-015 |
| `TestOpenAIOAuthCallback_DoesNotRequireUISessionCookie` | `internal/proxy/handler/openai_oauth_test.go` | callback route missing | FR-008 |

### User Story 3 Tests - Safe Failure Pages

| Test Function | File | Initial Failure | Covers |
|---------------|------|-----------------|--------|
| `TestOpenAIOAuthCallback_StateExpiredFailsReadablePage` | `internal/proxy/handler/openai_oauth_test.go` | callback route/state store missing | FR-010, FR-016 |
| `TestOpenAIOAuthCallback_StateMismatchFailsReadablePage` | `internal/proxy/handler/openai_oauth_test.go` | callback route/state store missing | FR-010, FR-016 |
| `TestOpenAIOAuthCallback_OpenAIErrorConsumesState` | `internal/proxy/handler/openai_oauth_test.go` | provider-error branch missing | FR-015..FR-017 |
| `TestOpenAIOAuthCallback_TokenExchangeFailureConsumesStateAndRedacts` | `internal/proxy/handler/openai_oauth_test.go` | exchange failure branch missing | FR-015..FR-017 |

### State Store Tests

| Test Function | File | Initial Failure | Covers |
|---------------|------|-----------------|--------|
| `TestStateStore_CreateAndConsumeOnce` | `internal/openaioauth/state_store_test.go` | package missing | FR-004..FR-006, FR-015 |
| `TestStateStore_ExpiredRecordRejectedAndDeleted` | `internal/openaioauth/state_store_test.go` | package missing | FR-010 |
| `TestStateStore_MalformedRecordRejectedAndDeleted` | `internal/openaioauth/state_store_test.go` | package missing | FR-010 |

### Verification Commands

```bash
go test ./internal/openaioauth/... -v
go test ./internal/ui/... -run 'TestHandleOpenAIConnect' -v
go test ./internal/proxy/handler/... -run 'TestOpenAIOAuthCallback' -v
go test ./internal/proxy/... -run 'TestOpenAIOAuthRoutes' -v
```

## Plan Review

- **Repo evidence**: `internal/provider/openai/oauth.go` already provides `GeneratePKCE`, `ChallengeFromVerifier`, `GenerateState`, `BuildAuthorizeURL`, and `ExchangeCode`; HO-1175 should orchestrate those helpers instead of duplicating OAuth primitives.
- **Repo evidence**: `internal/config/openai_oauth.go` already derives `/oauth/openai/callback` from `general_settings.public_base_url` and validates production HTTPS vs localhost development redirects.
- **Repo evidence**: `internal/ui/routes.go` mounts protected UI routes inside `h.sessionAuth`; `/ui/openai/connect` belongs in that protected group.
- **Repo evidence**: `internal/ui/session.go` scopes the UI cookie to `/ui`; `/oauth/openai/callback` cannot depend on that cookie and must rely on server-side state.
- **Repo evidence**: `internal/proxy/server.go` currently mounts UI under `/ui` and SSO public routes under `/sso`; HO-1175 must add the OpenAI callback as a separate public route before/alongside auth-required route groups.
- **Repo evidence**: `cmd/tianji/main.go` wires the same cache backend into `Handlers` and `UIHandler`, so a shared cache-backed state store can bridge connect and callback.
- **Go oauth2 docs**: `AuthCodeURL` treats state as an opaque client value echoed back by the authorization server; PKCE helpers generate a verifier, send S256 challenge on authorize, and send verifier on exchange.
- **Context7 `/golang/oauth2`**: confirms `GenerateVerifier`, `S256ChallengeOption`, and `VerifierOption` are the standard PKCE shape; Tianji already has equivalent OpenAI-specific primitives from HO-1174.
- **grep-app public code search**: examples from Ory Hydra, Dex, Authorizer, and Memoh show state/code-verifier is stored server-side and matched/consumed during token exchange; CodeQL examples flag constant OAuth state as unsafe.
- **OpenAI official docs lookup**: OpenAI developer docs were searched for callback/state-specific guidance; no stable callback-state page was available through the current docs fetch, so this plan relies on repo OpenAI OAuth config plus standard OAuth2/PKCE behavior.

## Phase 1: Tests First

Write the tests listed in `## Failing Tests` before production changes. Initial expected failure mode: missing connect route, missing callback route, missing state store, and missing callback orchestration.

## Phase 2: State Store

Implement `internal/openaioauth.StateStore`:

- typed `StateRecord`
- `Create(ctx, orgID, codeVerifier, ttl) (state string, record StateRecord, error)`
- `Consume(ctx, state) (StateRecord, error)` that deletes found records after load
- exact key prefixing and defensive `expires_at` validation
- malformed/expired records are deleted and returned as safe invalid-state errors

## Phase 3: Authenticated Connect

Implement UI connect:

- register `/openai/connect` inside `h.sessionAuth` protected routes
- validate `org_id` and public base URL
- generate PKCE/state using existing OpenAI helper functions
- store state before redirect
- redirect to `openai.BuildAuthorizeURL` with endpoint overrides respected
- render existing UI-style readable errors for validation/config/cache failures

## Phase 4: Public Callback

Implement callback handler:

- register `/oauth/openai/callback` outside API auth and outside `/ui`
- parse `state`, `code`, `error`, and sanitized display fields
- consume state before token exchange or provider-error rendering
- exchange code using stored verifier and configured client/endpoints
- persist successful token bundle through existing OpenAI subscription credential helper using stored `org_id`
- render safe HTML success/error pages

## Phase 5: Verify

- Run targeted tests from Verification Commands.
- Run `git diff --check`.
- Confirm no migration files were added unless a real schema gap was found.
- Confirm no tests require live OpenAI network or credentials.
- Confirm callback error pages do not include `access_token`, `refresh_token`, `id_token`, `code_verifier`, or raw upstream response strings.

## Risk Register

| Risk | Mitigation |
|------|------------|
| Callback trusts query `org_id` by accident | Add explicit conflicting-query test; persistence input must use stored state record only. |
| Replay reuses state after token exchange failure | Consume loaded state before/around terminal provider branches and test replay behavior. |
| UI cookie unavailable on callback path | Keep callback public and state-authenticated; add no-UI-cookie success test. |
| Cache backend returns stale bytes | Store `expires_at` inside JSON and reject defensively after `Get`. |
| Token/provider errors leak secrets in HTML | Add redaction tests with sentinel token/verifier strings. |
| Real OpenAI network sneaks into tests | Use endpoint overrides and HO-1190 guard client in callback/connect tests. |
