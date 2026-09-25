# Implementation Plan: OpenAI OAuth PKCE Primitives and Provider Config

**Branch**: `HO-1174-openai-oauth-pkce` | **Date**: 2026-05-06 | **Spec**: [spec.md](spec.md)  
**Input**: Feature specification from `/specs/569-openai-oauth-pkce/spec.md`

## Summary

Add backend-only OpenAI OAuth primitives and provider config: secure PKCE/state generation, OpenAI OAuth defaults/overrides, redirect URI derivation/validation, authorize URL builder, and public-client token exchange request/response parsing. This slice intentionally does not add callback routes, credential persistence, refresh, model routing, or UI.

## Technical Context

**Language/Version**: Go (repo currently uses Go modules; exact CI version must be verified during implementation)  
**Primary Dependencies**: Existing `golang.org/x/oauth2 v0.35.0`, `net/http`, `httptest`, `crypto/rand`, `crypto/sha256`, `encoding/base64`  
**Storage**: None in this issue  
**Testing**: `go test` + existing `testify` patterns; `httptest.NewServer` for token endpoint mocks  
**Target Platform**: Linux server / Docker / Kubernetes  
**Performance Goals**: OAuth primitive generation and request building are local and negligible; token exchange uses caller-provided context/timeouts  
**Constraints**: No real OpenAI network in tests; no `client_secret`; no production implementation until Linear is moved to **In Progress**  
**Scale/Scope**: One provider config per Tianji deployment; many auth attempts can generate independent PKCE/state pairs

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | ⚠️ EXCEPTION | This is new TianjiLLM-Go subscription OAuth support; no Python Tianji equivalent found in repo reconnaissance. |
| II. Feature Parity | ⚠️ EXCEPTION | New Go-only feature split from HO-1166 epic; existing OpenAI API-key behavior must remain compatible. |
| III. Research Before Build | ✅ PASS | research.md documents RFC/OpenAI/Codex/oauth2/repo decisions. |
| IV. Failing-Tests-First | ✅ PASS | Concrete failing tests are listed below; tasks begin each story with tests. |
| V. Go Best Practices | ✅ PASS | Small package-level helpers, context-aware token exchange, no globals for test endpoints. |
| VI. No Stale Knowledge | ✅ PASS | PKCE helpers verified via Context7; OpenAI/Codex fields verified from fetched source/docs. |
| VII. sqlc-First DB Access | ✅ N/A | No database access in HO-1174. Later credential-storage issues own DB work. |

## Project Structure

### Documentation (this feature)

```text
specs/569-openai-oauth-pkce/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### Source Code (future implementation scope; not written in Todo state)

```text
internal/
├── config/
│   ├── config.go                 # MODIFY: typed OpenAI OAuth config under GeneralSettings
│   └── openai_oauth_test.go       # NEW: defaults/overrides/redirect validation tests
├── provider/openai/
│   ├── oauth.go                   # NEW: OpenAI OAuth config, PKCE/state, authorize URL, token exchange
│   └── oauth_test.go              # NEW: PKCE, authorize URL, token request/response tests
└── auth/
    └── sso.go                     # READ ONLY: do not reuse confidential-client client_secret behavior for OpenAI OAuth
```

**Structure Decision**: Keep OpenAI subscription OAuth primitives close to `internal/provider/openai` because they are provider-specific, but put deployment config parsing in `internal/config` alongside existing `GeneralSettings`. Do not modify current `TransformRequest`/API-key provider path in this issue.

## Data Flow

```text
Config load
  proxy_config.yaml general_settings.openai_oauth
    → OpenAIOAuthConfig defaults/overrides
    → ValidateRedirectURI(public_base_url, env)

Authorize preparation
  GeneratePKCE() + GenerateState()
    → BuildAuthorizeURL(config, redirectURI, challenge, state)
    → later callback issue redirects user/browser to URL

Token exchange
  callback issue receives code + validates state
    → ExchangeCode(ctx, config, code, redirectURI, verifier)
    → POST token endpoint form:
       grant_type=authorization_code
       code=<code>
       redirect_uri=<redirectURI>
       client_id=<public client id>
       code_verifier=<verifier>
       (no client_secret)
    → typed TokenBundle returned to later credential persistence issue
```

## Failing Tests

### User Story 1 Tests — PKCE and State Primitives

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestGeneratePKCE_ProducesRFC7636S256Pair` | `internal/provider/openai/oauth_test.go` | Verifier length is RFC-valid, URL-safe/no padding; challenge equals base64url-no-padding(SHA256(verifier)) | AS-1.1 |
| `TestGeneratePKCE_KnownVerifierChallenge` | `internal/provider/openai/oauth_test.go` | Deterministic verifier derives expected S256 challenge | AS-1.3 |
| `TestGenerateState_UniqueURLSafe` | `internal/provider/openai/oauth_test.go` | Two states are non-empty, URL-safe, and different | AS-1.2 |

### User Story 2 Tests — Provider Config Defaults and Overrides

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestOpenAIOAuthConfig_Defaults` | `internal/config/openai_oauth_test.go` | Empty config yields OpenAI auth/token endpoints, public client ID default, expected scopes/flags | AS-2.1 |
| `TestOpenAIOAuthConfig_EndpointOverrides` | `internal/config/openai_oauth_test.go` | Local authorize/token override URLs are preserved exactly | AS-2.2 |
| `TestOpenAIOAuthConfig_ClientIDOverride` | `internal/config/openai_oauth_test.go` | Configured client ID replaces default | AS-2.3 |

### User Story 3 Tests — Authorize URL and Public-Client Token Exchange

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestBuildAuthorizeURL_OpenAIPKCEFields` | `internal/provider/openai/oauth_test.go` | Query includes response_type, client_id, redirect_uri, scope, code_challenge, S256 method, state, id_token_add_organizations, codex_cli_simplified_flow, originator | AS-3.1 |
| `TestExchangeCode_PublicClientPKCERequestShape` | `internal/provider/openai/oauth_test.go` | Mock token endpoint receives form with grant_type/code/redirect_uri/client_id/code_verifier | AS-3.2 |
| `TestExchangeCode_DoesNotSendClientSecret` | `internal/provider/openai/oauth_test.go` | Request form has no client_secret and Authorization header has no Basic credential | AS-3.3 |
| `TestExchangeCode_ParsesTokenBundle` | `internal/provider/openai/oauth_test.go` | JSON response preserves access/refresh/id token, token type, expiry, scope/metadata | AS-3.4 |
| `TestExchangeCode_ErrorPreservesStatus` | `internal/provider/openai/oauth_test.go` | Non-200 mock response returns an error with status/body context and no secret leakage | Edge |

### User Story 4 Tests — Redirect URI Safety

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestDeriveOpenAIRedirectURI_HTTPSProduction` | `internal/config/openai_oauth_test.go` | `https://tianji.example.com/` derives `https://tianji.example.com/oauth/openai/callback` | AS-4.1 |
| `TestValidateOpenAIRedirectURI_RejectsProductionHTTP` | `internal/config/openai_oauth_test.go` | Production `http://tianji.example.com` returns config error | AS-4.2 |
| `TestValidateOpenAIRedirectURI_AllowsLocalhostDev` | `internal/config/openai_oauth_test.go` | Dev/test `http://localhost:<port>` succeeds | AS-4.3 |
| `TestDeriveOpenAIRedirectURI_MalformedBaseURL` | `internal/config/openai_oauth_test.go` | Empty/malformed base URL returns clear error | AS-4.4 |

### Regression Tests — Existing OpenAI API-Key Path

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestTransformRequest_OpenAIAPIKeyUnaffectedByOAuthConfig` | `internal/provider/openai/openai_test.go` | Existing `TransformRequest` still sends `Authorization: Bearer <api_key>` to default OpenAI API URL | FR-012 |
| `TestOpenAIOAuthConfig_CustomAPIBaseInvalidWhenSubscriptionIDsLaterEnabled` | `internal/config/openai_oauth_test.go` | Placeholder validation documents later invariant: subscription credentials cannot combine with custom `api_base` | Epic invariant |

### Verification Command

```bash
go test ./internal/config/... -run 'TestOpenAIOAuth|TestDeriveOpenAI|TestValidateOpenAI' -v
go test ./internal/provider/openai/... -run 'TestGeneratePKCE|TestGenerateState|TestBuildAuthorizeURL|TestExchangeCode|TestTransformRequest_OpenAIAPIKeyUnaffected' -v
```

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Python parity exception | No Python source-of-truth feature exists for OpenAI subscription OAuth in repo reconnaissance | Blocking on Python parity would stall a new Go-only product capability |
| OpenAI-specific helper package | OAuth fields/flags are provider-specific | Generic SSO handler sends client_secret and would create wrong public-client behavior |

## Phase 0: Research Complete

- RFC 7636 checked for S256 PKCE math.
- `golang.org/x/oauth2` docs checked through Context7 for PKCE helper APIs.
- OpenAI/Codex source checked for issuer, endpoints, authorize fields, scopes, and PKCE/state generation pattern.
- Repo checked for existing `GeneralSettings`, OpenAI provider, SSO handler, and current API-key path.

## Phase 1: Design Tests First

Write all tests in the Failing Tests section before any production implementation. Initial expected failure mode: missing OpenAI OAuth config/types/functions, not panic or environment/network failure.

## Phase 2: Implement Primitives and Config

Implement the smallest provider-specific package/config additions to satisfy tests:

- `OpenAIOAuthConfig` defaults/overrides.
- `GeneratePKCE`, `ChallengeFromVerifier`, `GenerateState` wrappers.
- `DeriveOpenAIRedirectURI`, `ValidateOpenAIRedirectURI`.
- `BuildAuthorizeURL`.
- `ExchangeCode` public-client request + token response parser.

## Phase 3: Verify and Preserve Existing Behavior

- Run targeted tests from Verification Command.
- Run existing OpenAI provider tests to prove current API-key path is unchanged.
- No callback route, DB, UI, or routing changes in this issue.

## Risk Register

| Risk | Mitigation |
|------|------------|
| OpenAI undocumented OAuth fields may change | Isolate fields in config defaults; tests verify request shape without hitting live OpenAI |
| Public base URL config naming may need repo convention cleanup during implementation | Use the Linear issue concept "Tianji public base URL"; prefer `public_base_url` unless repo convention says otherwise |
| Accidentally reusing SSO handler could send `client_secret` | Tests explicitly assert no `client_secret` and no Basic auth |
