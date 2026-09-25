# Tasks: OpenAI OAuth Connect/Callback State Lifecycle

**Input**: Design documents from `/specs/1175-openai-oauth-connect-callback-state-lifecycle/`
**Prerequisites**: HO-1174 OpenAI OAuth PKCE/config primitives, HO-1176 credential persistence helpers, HO-1190 OpenAI OAuth mock harness

**Tests**: Failing tests are mandatory. Write tests first and confirm they fail before implementation.

## Phase 0: Pre-Implementation Gate

**Goal**: Confirm Todo planning artifacts are complete before implementation.

- [x] T000 Confirm `spec.md`, `plan.md`, `tasks.md`, and `analyze.md` exist; `analyze.md` reports 0 fatal / 0 critical / 0 owner input.

## Phase 1: Tests First - State Store

**Goal**: Prove state is TTL-bound, exact-match, and consume-once before handler code exists.

- [x] T001 [P] Add failing `TestStateStore_CreateAndConsumeOnce` in `internal/openaioauth/state_store_test.go`.
- [x] T002 [P] Add failing `TestStateStore_ExpiredRecordRejectedAndDeleted` in `internal/openaioauth/state_store_test.go`.
- [x] T003 [P] Add failing `TestStateStore_MalformedRecordRejectedAndDeleted` in `internal/openaioauth/state_store_test.go`.
- [x] T004 Run state-store tests and confirm they fail for missing package/implementation.

## Phase 2: Tests First - Authenticated Connect

**Goal**: Prove `/ui/openai/connect` requires UI auth and stores state before redirect.

- [x] T005 [P] Add failing `TestHandleOpenAIConnect_RequiresSession` in `internal/ui/handler_openai_test.go`.
- [x] T006 [P] Add failing `TestHandleOpenAIConnect_StoresStateAndRedirects` in `internal/ui/handler_openai_test.go`.
- [x] T007 [P] Add failing `TestHandleOpenAIConnect_RejectsMissingOrgID` in `internal/ui/handler_openai_test.go`.
- [x] T008 [P] Add failing `TestHandleOpenAIConnect_UsesEndpointOverride` in `internal/ui/handler_openai_test.go`.
- [x] T009 Run targeted UI tests and confirm they fail for missing route/handler.

## Phase 3: Tests First - Callback Success and Org Trust

**Goal**: Prove callback uses only server-side state and consumes it after success.

- [x] T010 [P] Add failing `TestOpenAIOAuthCallback_SuccessConsumesStateAndPersistsCredential` in `internal/proxy/handler/openai_oauth_test.go`.
- [x] T011 [P] Add failing `TestOpenAIOAuthCallback_IgnoresQueryOrgID` in `internal/proxy/handler/openai_oauth_test.go`.
- [x] T012 [P] Add failing `TestOpenAIOAuthCallback_ReplayFailsAfterConsume` in `internal/proxy/handler/openai_oauth_test.go`.
- [x] T013 [P] Add failing `TestOpenAIOAuthCallback_DoesNotRequireUISessionCookie` in `internal/proxy/handler/openai_oauth_test.go`.
- [x] T014 Run targeted callback tests and confirm they fail for missing callback route/handler.

## Phase 4: Tests First - Safe Failure Pages

**Goal**: Prove invalid state/provider/token failures are readable, redacted, and non-reusable.

- [x] T015 [P] Add failing `TestOpenAIOAuthCallback_StateExpiredFailsReadablePage` in `internal/proxy/handler/openai_oauth_test.go`.
- [x] T016 [P] Add failing `TestOpenAIOAuthCallback_StateMismatchFailsReadablePage` in `internal/proxy/handler/openai_oauth_test.go`.
- [x] T017 [P] Add failing `TestOpenAIOAuthCallback_OpenAIErrorConsumesState` in `internal/proxy/handler/openai_oauth_test.go`.
- [x] T018 [P] Add failing `TestOpenAIOAuthCallback_TokenExchangeFailureConsumesStateAndRedacts` in `internal/proxy/handler/openai_oauth_test.go`.
- [x] T019 Run targeted callback failure tests and confirm they fail for missing implementation.

## Phase 5: Implement State Store

**Goal**: Add shared cache-backed OAuth state lifecycle.

- [x] T020 Add `internal/openaioauth.StateRecord` with `state`, `code_verifier`, `org_id`, `created_at`, and `expires_at`.
- [x] T021 Add cache key prefixing and exact state lookup.
- [x] T022 Implement `Create` with fresh OpenAI state/PKCE generation and cache TTL.
- [x] T023 Implement `Consume` with get/validate/delete behavior.
- [x] T024 Ensure expired/malformed found records are deleted and return safe invalid-state errors.
- [x] T025 Run `go test ./internal/openaioauth/... -v`.

## Phase 6: Implement Authenticated Connect

**Goal**: Start OpenAI OAuth from UI session only.

- [x] T026 Register `/openai/connect` inside the existing protected UI `sessionAuth` route group.
- [x] T027 Implement `handleOpenAIConnect` in `internal/ui/handler_openai.go`.
- [x] T028 Validate `org_id` before state creation.
- [x] T029 Derive redirect URI from `general_settings.public_base_url`.
- [x] T030 Build authorize URL with existing OpenAI helper and endpoint overrides.
- [x] T031 Render readable UI errors for validation/config/cache failures.
- [x] T032 Run `go test ./internal/ui/... -run 'TestHandleOpenAIConnect' -v`.

## Phase 7: Implement Public Callback

**Goal**: Exchange code and persist credentials using server-side state.

- [x] T033 Register `/oauth/openai/callback` as a public route outside API auth and UI auth.
- [x] T034 Implement callback query parsing and sanitized success/error page rendering.
- [x] T035 Consume state before provider error rendering or token exchange.
- [x] T036 Reject missing/malformed/expired/mismatched state before token exchange.
- [x] T037 Exchange authorization code with stored `code_verifier`.
- [x] T038 Persist successful token bundle through OpenAI subscription credential helper using stored `org_id`.
- [x] T039 Ignore query/body `org_id`, `organization_id`, and `credential_id`.
- [x] T040 Delete/consume state after token exchange failures and provider-denial callbacks.
- [x] T041 Run `go test ./internal/proxy/handler/... -run 'TestOpenAIOAuthCallback' -v`.

## Phase 8: Verification and PR Hygiene

- [x] T042 Run route registration tests: `go test ./internal/proxy/... -run 'TestOpenAIOAuthRoutes' -v`.
- [x] T043 Run broader affected packages: `go test ./internal/openaioauth/... ./internal/ui/... ./internal/proxy/... ./internal/proxy/handler/... -v`.
- [x] T044 Run `git diff --check`.
- [x] T045 Confirm no migration files were added unless a real schema gap was found.
- [x] T046 Confirm no test requires real OpenAI credentials or network access.
- [x] T047 Confirm `git diff --name-only origin/main...HEAD` is limited to HO-1175 scope.

## Dependencies

- State store tests and implementation block connect/callback implementation.
- Connect must land before callback success E2E-style tests can seed real state through the UI handler.
- Callback persistence depends on HO-1176 helpers on the base branch.
- Offline callback tests depend on HO-1190 mock harness on the base branch.
- Implementation must not start until Linear HO-1175 moves to In Progress.
