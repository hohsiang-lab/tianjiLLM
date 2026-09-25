# Implementation Plan: OpenAI Subscription Config and Credential Resolution

**Branch**: HO-1167-openai-subscription-config-credential-resolution
**Date**: 2026-05-07
**Spec**: [spec.md](spec.md)
**Input**: Linear HO-1167

## Summary

Implement an explicit `openai_subscription_credential_ids` field on model `tianji_params`, validate that it only applies to default official OpenAI deployments, and add a resolver boundary that chooses subscription credentials over API-key fallback only when IDs are explicitly configured. Runtime credential application is transport-specific: direct OpenAI HTTP receives bearer material for later injection, while Codex app-server receives account credentials through `account/login/start` using `chatgptAuthTokens`.

## Technical Context

**Language/Version**: Go module in TianjiLLM
**Primary Dependencies**: Existing `gopkg.in/yaml.v3`, `net/http`, sqlc DB layer, existing HO-1176 credential helpers, existing HO-1183 redaction helpers
**Storage**: Existing `CredentialTable` rows with `credential_type = "openai_subscription"`
**Testing**: Go config/unit/handler tests, no real OpenAI network
**Target Platform**: Linux server / Docker / Kubernetes
**Performance Goals**: Credential resolution is per selected deployment and should avoid broad DB scans
**Constraints**: No production code in Todo state; branch/commit/push/draft PR are limited to SpecKit planning artifacts
**Scale/Scope**: Per-model config and resolver boundary for OpenAI subscription credentials

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | WARNING EXCEPTION | TianjiLLM config/router/auth code is Go-only in this repo slice. |
| II. Feature Parity | PASS | Existing API-key behavior is preserved when subscription IDs are omitted. |
| III. Research Before Build | PASS | research.md records repo evidence plus OpenAI Codex app-server protocol/source evidence. |
| IV. Failing-Tests-First | PASS | tasks.md starts each story with failing tests. |
| V. Go Best Practices | PASS | Plan uses typed config and small resolver interfaces instead of stringly typed Overflow reads. |
| VI. No Stale Knowledge | PASS | Mutable Codex app-server behavior is marked version-sensitive and must be verified at implementation time. |
| VII. sqlc-First DB Access | PASS | Credential lookup/refresh must use existing sqlc-backed helpers; no handwritten DB queries. |

## Project Structure

### Documentation

```text
specs/1167-openai-subscription-config-credential-resolution/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── analyze.md
├── contracts/
│   └── openai-subscription-config.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### Source Code (future implementation scope; not written in Todo state)

```text
internal/
├── config/
│   ├── config.go              # MODIFY: typed subscription credential IDs field
│   ├── loader.go              # MODIFY: keep env interpolation unchanged; no env expansion for IDs
│   ├── validate.go            # MODIFY: hard validation for subscription ID constraints
│   └── loader_test.go         # MODIFY: config load/validation regression tests
├── proxy/
│   └── handler/
│       ├── handler.go         # MODIFY: resolver boundary for model-based provider resolution
│       ├── forward.go         # MODIFY: direct OpenAI default-base behavior checks
│       ├── native_upstream.go # MODIFY: direct native upstream credential resolution boundary
│       └── credential_resolution_test.go
├── router/
│   └── router.go              # MODIFY: preserve deployment selection; expose config to resolver
└── codexapp/
    ├── auth.go                # NEW or equivalent local package: app-server login RPC payload/env shaping
    └── auth_test.go
```

**Structure Decision**: Keep YAML parsing and schema validation in `internal/config`. Add a resolver boundary near existing handler/router provider resolution so direct HTTP code can consume resolved bearer material later without entangling it with Codex app-server login RPC. Put app-server-specific env and JSON-RPC payload shaping behind a small package or handler-local helper, depending on where the eventual app-server integration already lives.

## Data Flow

```text
proxy_config.yaml
  -> config.Load / LoadWithSecrets
  -> TianjiParams.OpenAISubscriptionCredentialIDs
  -> Validate subscription IDs:
       explicit IDs only
       provider = openai/default
       no custom api_base
       no duplicate/empty IDs
  -> router selects deployment
  -> resolver inspects selected deployment

If no subscription IDs:
  -> existing api_key / OPENAI_API_KEY path unchanged

If subscription IDs configured:
  -> lookup exactly configured CredentialTable IDs
  -> require credential_type=openai_subscription
  -> decrypt/refresh or reject
  -> return transport-specific credential application:
       Direct OpenAI HTTP: resolved bearer credential boundary
       Codex app-server: account/login/start chatgptAuthTokens payload
```

## Repo Evidence

- `internal/config/config.go` currently models `TianjiParams` with `Model`, `APIKey`, `APIBase`, `APIVersion`, rate limits, region, auto-router fields, and `Overflow`; no typed subscription credential IDs exist yet.
- `internal/config/loader.go` currently resolves environment variables only for API-key/base/version fields in model params; this must remain compatible.
- `internal/config/validate.go` currently warns unknown overflow fields rather than failing; HO-1167 needs hard validation for the new typed field.
- `internal/proxy/handler/handler.go`, `forward.go`, `native_upstream.go`, and `internal/router/router.go` currently resolve providers from `api_base` and `api_key`; the resolver boundary must be inserted without changing no-subscription behavior.
- Repo search found no Tianji app-server implementation using `chatgptAuthTokens` yet, so that path needs a new integration boundary when implementation starts.

## External Evidence

- OpenAI Codex app-server README documents the auth/account JSON-RPC surface, including `account/read`, `account/login/start`, API-key mode, and ChatGPT managed auth: <https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md#auth-endpoints>
- OpenAI Codex protocol source defines `account/login/start` and a `chatgptAuthTokens` login variant marked experimental/internal-use in `codex-rs/app-server-protocol/src/protocol/v2/account.rs`: <https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/src/protocol/v2/account.rs>
- OpenAI Codex protocol source states that in external auth mode, clients refresh tokens themselves and call `account/login/start` with `chatgptAuthTokens`: <https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/src/protocol/v2/account.rs>
- Public openai/codex issues report `OPENAI_API_KEY` environment values overriding or confusing ChatGPT/OAuth auth flows; this supports the env-stripping risk guard for local stdio launches: <https://github.com/openai/codex/issues/15151>
- Context7 `/yaml/go-yaml` docs confirm strict known-field validation exists, but this repo intentionally uses `Overflow` to preserve LiteLLM config compatibility; HO-1167 should add targeted validation instead of globally enabling strict YAML.
- `grep-app-cli` returned OpenAI Codex protocol hits for `chatgptAuthTokens`; later grep-app searches intermittently failed with a transport content-type error, so only successful openai/codex hits are used as evidence.

## Failing Tests

| Test Function | File | Initial Failure | Covers |
|---------------|------|-----------------|--------|
| `TestLoad_OpenAISubscriptionCredentialIDs` | `internal/config/loader_test.go` | field missing | FR-001, FR-002 |
| `TestLoad_OpenAISubscriptionIDsEmptyPreservesAPIKey` | `internal/config/loader_test.go` | field missing/regression guard | FR-006 |
| `TestValidate_OpenAISubscriptionRejectsCustomAPIBase` | `internal/config/loader_test.go` | validation missing | FR-004 |
| `TestValidate_OpenAISubscriptionRejectsCustomProvider` | `internal/config/loader_test.go` | validation missing | FR-005 |
| `TestValidate_OpenAISubscriptionRejectsEmptyOrDuplicateIDs` | `internal/config/loader_test.go` | validation missing | FR-003 |
| `TestResolveOpenAISubscriptionCredential_ExplicitIDsOnly` | `internal/proxy/handler/credential_resolution_test.go` | resolver missing | FR-007, FR-008 |
| `TestResolveOpenAISubscriptionCredential_NoAPIKeyFallbackOnFailure` | `internal/proxy/handler/credential_resolution_test.go` | resolver missing | FR-010, FR-011 |
| `TestResolveProvider_NoSubscriptionKeepsAPIKeyPath` | `internal/proxy/handler/credential_resolution_test.go` | regression guard | FR-006 |
| `TestDirectOpenAIHTTPResolution_ReturnsBearerMaterial` | `internal/proxy/handler/credential_resolution_test.go` | resolver missing | FR-012 |
| `TestCodexAppServerResolution_UsesChatGPTAuthTokensLogin` | `internal/codexapp/auth_test.go` | app-server helper missing | FR-013, FR-014 |
| `TestCodexAppServerEnv_StripsOpenAIKeysForSubscriptionProfile` | `internal/codexapp/auth_test.go` | env helper missing | FR-015 |
| `TestCodexAppServerWebSocket_ConnectionAuthSeparatedFromOpenAIAuth` | `internal/codexapp/auth_test.go` | app-server helper missing | FR-016 |

### Verification Commands

```bash
go test ./internal/config/... -run 'TestLoad_OpenAISubscription|TestValidate_OpenAISubscription' -v
go test ./internal/proxy/handler/... -run 'TestResolveOpenAISubscription|TestResolveProvider_NoSubscription|TestDirectOpenAIHTTPResolution' -v
go test ./internal/codexapp/... -run 'TestCodexAppServer' -v
go test ./internal/config/... ./internal/proxy/handler/... ./internal/router/... -v
```

## Phase 1: Tests First

Write the tests listed above before production changes. Initial expected failures are missing config field, missing validation, missing resolver boundary, and missing app-server auth/env helpers.

## Phase 2: Typed Config and Validation

Add `OpenAISubscriptionCredentialIDs []string` to `TianjiParams`. Keep `APIKey`, `APIBase`, and existing env interpolation unchanged. Add targeted validation for duplicate/empty IDs, provider/default-base restrictions, and `api_base` incompatibility.

## Phase 3: Credential Resolver Boundary

Add a resolver that accepts the selected deployment/model config, organization/caller context, and desired transport. It returns one of:

- existing API-key config when subscription IDs are empty
- direct HTTP resolved bearer material when subscription IDs are configured for direct OpenAI paths
- app-server login payload material when subscription IDs are configured for Codex app-server paths
- sanitized explicit error with no API-key fallback when configured subscription IDs fail

## Phase 4: Codex App-Server Auth Separation

Implement app-server-specific helpers:

- build `account/login/start` payload with `type: "chatgptAuthTokens"` and selected account tokens
- strip `CODEX_API_KEY` and `OPENAI_API_KEY` for local stdio launches using subscription-style profiles
- keep WebSocket `appServer.authToken`/headers separate from OpenAI account credential delivery
- verify app-server protocol behavior against the checked Codex app-server version before implementation

## Phase 5: Verify and Governance

- Run targeted tests first, then affected package tests.
- Ensure `git diff --name-only` contains only HO-1167 files in implementation scope.
- During Todo planning, commit/push/draft PR are allowed only for SpecKit artifacts; do not write production code.

## Risk Register

| Risk | Mitigation |
|------|------------|
| New YAML field is ignored through `Overflow` | Add typed field plus load test proving IDs populate. |
| Subscription IDs accidentally fallback to API key | Add resolver failure test proving no fallback once IDs are configured. |
| Custom OpenAI-compatible providers receive subscription credentials | Validate provider/default base and reject custom `api_base`. |
| Codex app-server protocol changes | Verify checked app-server protocol/source at implementation time and pin helper tests to that shape. |
| Environment API key overrides app-server account auth | Strip `CODEX_API_KEY` and `OPENAI_API_KEY` in stdio subscription launches and test it. |
| Token material leaks in errors | Reuse HO-1183 redaction boundary for resolver errors and metadata. |

## Todo Gate Status

Spec/plan/tasks/analyze are complete and ready for draft PR review. Todo -> Waiting is allowed after draft PR creation.
