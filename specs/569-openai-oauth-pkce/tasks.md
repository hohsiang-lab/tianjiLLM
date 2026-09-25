# Tasks: OpenAI OAuth PKCE Primitives and Provider Config

**Input**: Design documents from `/specs/569-openai-oauth-pkce/`  
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, quickstart.md

**Tests**: Failing tests are MANDATORY (Constitution Principle IV). Task 1 of every user story MUST be "Write failing tests" based on the plan's `## Failing Tests` section. No implementation task may begin until failing tests are written and confirmed to fail.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task maps to (e.g., US1, US2)
- Include exact file paths in descriptions

## Phase 1: Setup / Test Scaffolding

**Purpose**: Add failing tests first. No production code until tests compile and fail for missing implementation.

- [x] T001 [P] [US1] Write failing test `TestGeneratePKCE_ProducesRFC7636S256Pair` in `internal/provider/openai/oauth_test.go` — assert verifier format/length and challenge equals base64url-no-padding(SHA256(verifier))
- [x] T002 [P] [US1] Write failing test `TestGeneratePKCE_KnownVerifierChallenge` in `internal/provider/openai/oauth_test.go` — assert deterministic verifier derives known S256 challenge
- [x] T003 [P] [US1] Write failing test `TestGenerateState_UniqueURLSafe` in `internal/provider/openai/oauth_test.go` — assert two generated states are non-empty, URL-safe, and different
- [x] T004 [P] [US2] Write failing test `TestOpenAIOAuthConfig_Defaults` in `internal/config/openai_oauth_test.go` — assert default issuer/auth/token/client/scopes/flags
- [x] T005 [P] [US2] Write failing test `TestOpenAIOAuthConfig_EndpointOverrides` in `internal/config/openai_oauth_test.go` — assert local override endpoints are preserved exactly
- [x] T006 [P] [US2] Write failing test `TestOpenAIOAuthConfig_ClientIDOverride` in `internal/config/openai_oauth_test.go` — assert configured client ID replaces default
- [x] T007 [P] [US3] Write failing test `TestBuildAuthorizeURL_OpenAIPKCEFields` in `internal/provider/openai/oauth_test.go` — assert exact authorize query fields
- [x] T008 [P] [US3] Write failing test `TestExchangeCode_PublicClientPKCERequestShape` in `internal/provider/openai/oauth_test.go` — mock token endpoint asserts form fields
- [x] T009 [P] [US3] Write failing test `TestExchangeCode_DoesNotSendClientSecret` in `internal/provider/openai/oauth_test.go` — assert no `client_secret` and no Basic auth
- [x] T010 [P] [US3] Write failing test `TestExchangeCode_ParsesTokenBundle` in `internal/provider/openai/oauth_test.go` — assert token JSON response fields are preserved
- [x] T011 [P] [US3] Write failing test `TestExchangeCode_ErrorPreservesStatus` in `internal/provider/openai/oauth_test.go` — assert non-200 response returns useful error without secrets
- [x] T012 [P] [US4] Write failing test `TestDeriveOpenAIRedirectURI_HTTPSProduction` in `internal/config/openai_oauth_test.go` — assert HTTPS base derives callback path
- [x] T013 [P] [US4] Write failing test `TestValidateOpenAIRedirectURI_RejectsProductionHTTP` in `internal/config/openai_oauth_test.go` — assert production HTTP rejected
- [x] T014 [P] [US4] Write failing test `TestValidateOpenAIRedirectURI_AllowsLocalhostDev` in `internal/config/openai_oauth_test.go` — assert localhost HTTP allowed in dev/test
- [x] T015 [P] [US4] Write failing test `TestDeriveOpenAIRedirectURI_MalformedBaseURL` in `internal/config/openai_oauth_test.go` — assert malformed base URL returns config error
- [x] T016 [P] Write regression test `TestTransformRequest_OpenAIAPIKeyUnaffectedByOAuthConfig` in `internal/provider/openai/openai_test.go` — assert existing API-key request path unchanged
- [x] T017 Run targeted tests and confirm they compile then fail for missing implementation: `go test ./internal/config/... ./internal/provider/openai/... -run 'TestOpenAIOAuth|TestDeriveOpenAI|TestValidateOpenAI|TestGeneratePKCE|TestGenerateState|TestBuildAuthorizeURL|TestExchangeCode|TestTransformRequest_OpenAIAPIKeyUnaffected' -v`

**Checkpoint**: All planned tests exist and fail for the expected missing implementation, not due to network/environment.

---

## Phase 2: User Story 1 — PKCE and State Primitives (Priority: P1)

**Goal**: Generate RFC 7636-compatible PKCE pairs and secure state values.

**Independent Test**: `go test ./internal/provider/openai/... -run 'TestGeneratePKCE|TestGenerateState' -v`

- [x] T018 [US1] Create `internal/provider/openai/oauth.go` with `PKCEPair` type and `GeneratePKCE()` wrapper using `golang.org/x/oauth2` helpers or equivalent RFC 7636 implementation
- [x] T019 [US1] Add `ChallengeFromVerifier(verifier string)` helper for deterministic tests and validation
- [x] T020 [US1] Add `GenerateState()` using cryptographically secure randomness and URL-safe base64 without padding
- [x] T021 [US1] Run US1 tests and confirm pass

**Checkpoint**: PKCE/state primitives are reusable and offline-tested.

---

## Phase 3: User Story 2 — Provider Config Defaults and Overrides (Priority: P1)

**Goal**: Define safe OpenAI OAuth provider config defaults plus test/deployment overrides.

**Independent Test**: `go test ./internal/config/... -run 'TestOpenAIOAuthConfig' -v`

- [x] T022 [US2] Add typed `OpenAIOAuthConfig` to `internal/config/config.go` under `GeneralSettings`, using the Linear issue concept "Tianji public base URL" (prefer `public_base_url` unless repo convention says otherwise)
- [x] T023 [US2] Implement defaulting function for issuer, authorize endpoint, token endpoint, public client ID, scopes, originator, and boolean flags
- [x] T024 [US2] Implement endpoint/client ID override handling without appending paths to full override URLs
- [x] T025 [US2] Run US2 tests and confirm pass

**Checkpoint**: Config defaults/overrides are deterministic and CI-testable.

---

## Phase 4: User Story 4 — Redirect URI Safety (Priority: P1)

**Goal**: Derive and validate OpenAI callback redirect URI safely.

**Independent Test**: `go test ./internal/config/... -run 'TestDeriveOpenAI|TestValidateOpenAI' -v`

- [x] T026 [US4] Implement `DeriveOpenAIRedirectURI(publicBaseURL string) (string, error)` in `internal/config` or shared config helper
- [x] T027 [US4] Implement environment-aware redirect validation: production requires HTTPS; dev/test allows localhost/loopback HTTP only
- [x] T028 [US4] Ensure trailing slash handling derives exactly `/oauth/openai/callback`
- [x] T029 [US4] Run US4 tests and confirm pass

**Checkpoint**: Unsafe redirect config fails before any OAuth callback code exists.

---

## Phase 5: User Story 3 — Authorize URL and Token Exchange (Priority: P1)

**Goal**: Build OpenAI authorize URLs and public-client token exchange requests with PKCE.

**Independent Test**: `go test ./internal/provider/openai/... -run 'TestBuildAuthorizeURL|TestExchangeCode' -v`

- [x] T030 [US3] Implement `BuildAuthorizeURL(config, redirectURI, codeChallenge, state)` in `internal/provider/openai/oauth.go`
- [x] T031 [US3] Define typed `TokenBundle` response model preserving access/refresh/id token, token type, expiry, scope, and raw metadata
- [x] T032 [US3] Implement `ExchangeCode(ctx, httpClient, config, code, redirectURI, codeVerifier)` using form-encoded POST to configured token endpoint
- [x] T033 [US3] Ensure token exchange sends `client_id` and `code_verifier`, never `client_secret` and never Basic auth
- [x] T034 [US3] Implement non-200 token exchange error handling with useful status/body context and no secret leakage
- [x] T035 [US3] Run US3 tests and confirm pass

**Checkpoint**: Later callback issue can reuse tested request builders without reimplementing OAuth form logic.

---

## Phase 6: Regression and Final Verification

**Purpose**: Prove existing OpenAI API-key path remains unchanged and no live OpenAI network is required.

- [x] T036 Run regression test `go test ./internal/provider/openai/... -run 'TestTransformRequest_OpenAIAPIKeyUnaffectedByOAuthConfig|TestTransformRequest' -v`
- [x] T037 Run all targeted HO-1174 tests: `go test ./internal/config/... ./internal/provider/openai/... -run 'TestOpenAIOAuth|TestDeriveOpenAI|TestValidateOpenAI|TestGeneratePKCE|TestGenerateState|TestBuildAuthorizeURL|TestExchangeCode|TestTransformRequest_OpenAIAPIKeyUnaffected' -v`
- [x] T038 Run package tests for touched packages: `go test ./internal/config/... ./internal/provider/openai/...`
- [x] T039 Verify no production callback route, DB migration, UI, credential persistence, or model routing code was added in HO-1174
- [x] T040 Commit and push HO-1174 changes after implementation gate passes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Tests)**: No dependencies; must run first.
- **Phase 2 (US1)**: Depends on T001-T003 failing.
- **Phase 3 (US2)**: Depends on T004-T006 failing.
- **Phase 4 (US4)**: Depends on T012-T015 failing; can run after or alongside US2 implementation.
- **Phase 5 (US3)**: Depends on US1 PKCE primitives and US2 config defaults.
- **Phase 6 (Regression)**: Depends on all implementation phases.

### Parallel Opportunities

- T001-T016 can be written in parallel by file grouping.
- US1 implementation can proceed independently of redirect validation once tests are failing.
- US2 config and US4 redirect helpers can be implemented in parallel if they touch different files/helpers.

## Implementation Strategy

### MVP First

1. Write all failing tests.
2. Implement PKCE/state primitives.
3. Implement config defaults/overrides and redirect validation.
4. Implement authorize/token request builders.
5. Verify existing OpenAI API-key behavior unchanged.

### Scope Stop

Because current Linear state is Todo/Waiting, this planning PR stops at SpecKit artifacts. Norman confirmed scope should align to the Linear issue; production implementation tasks above start only after Linear is moved to **In Progress**.

## Notes

- Do not reuse `internal/auth/SSOHandler.ExchangeCode` for OpenAI OAuth unless it is refactored not to send `client_secret`; the safer path is provider-specific request builder with explicit tests.
- Keep all tests offline with `httptest`.
- Keep callback/persistence/refresh/routing out of HO-1174.
