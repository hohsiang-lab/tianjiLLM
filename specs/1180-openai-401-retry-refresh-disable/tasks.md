# Tasks: OpenAI 401 Retry, Refresh Failure, and Disable Behavior

**Input**: Design documents from `/specs/1180-openai-401-retry-refresh-disable/`
**Prerequisites**: `spec.md`, `plan.md`, `research.md`, `data-model.md`, `contracts/openai-401-refresh-failover.md`

**Tests**: Failing tests are mandatory. No production implementation task may begin until the relevant failing tests are written and confirmed to fail against current behavior.

## Phase 1: Test Baseline

**Purpose**: Lock HO-1180 behavior before changing retry/refresh code.

- [X] T001 [P] Write failing direct-handler test `TestOpenAISubscriptionRouting_401RefreshRetrySameCredentialSucceeds` in `internal/proxy/handler/openai_subscription_endpoints_test.go`
- [X] T002 [P] Write failing direct-handler test `TestOpenAISubscriptionRouting_401RefreshFailureFailsOver` in `internal/proxy/handler/openai_subscription_endpoints_test.go`
- [X] T003 [P] Write failing direct-handler test `TestOpenAISubscriptionRouting_401RetryStillFailsDisablesAndFailsOver` in `internal/proxy/handler/openai_subscription_endpoints_test.go`
- [X] T004 [P] Write failing direct-handler test `TestOpenAISubscriptionRouting_AllCredentialsAuthFailedReturnsReauthError` in `internal/proxy/handler/openai_subscription_endpoints_test.go`
- [X] T005 [P] Write failing proxy-transport test `TestOpenAISubscriptionProxyTransport_401RefreshRetrySameCredentialSucceeds` in `internal/proxy/handler/openai_subscription_endpoints_test.go`
- [X] T006 [P] Write failing proxy-transport test `TestOpenAISubscriptionProxyTransport_401RetryStillFailsFailsOver` in `internal/proxy/handler/openai_subscription_endpoints_test.go`
- [X] T007 [P] Write failing refresh test `TestForceRefreshOpenAISubscriptionCredential_BypassesFreshness` in `internal/proxy/handler/openai_subscription_refresh_test.go`
- [X] T008 [P] Write failing metadata test `TestOpenAISubscriptionCredential_AuthFailureMetadataRedacted` in `internal/proxy/handler/credential_test.go` or `openai_subscription_refresh_test.go`
- [X] T009 [P] Write regression test `TestOpenAISubscriptionRouting_401DoesNotRefreshAPIKeyPath` in `internal/proxy/handler/openai_subscription_endpoints_test.go`
- [X] T010 Update existing non-rate-limit 4xx regression so ordinary 400/403 remains no-retry while subscription 401 follows forced refresh
- [X] T011 Run targeted tests and confirm failures are current behavior, not environment or real network

**Checkpoint**: Tests prove current code returns/fails on 401 instead of forced refresh/retry/failover.

## Phase 2: Forced Refresh Boundary

**Goal**: Refresh a specific credential because upstream rejected its bearer token.

- [X] T012 Add helper to force-refresh an OpenAI subscription credential by `credential_id`
- [X] T013 Ensure forced refresh reloads the latest credential row before network call
- [X] T014 Ensure forced refresh bypasses normal local freshness when reason is upstream 401
- [X] T015 Reuse existing OpenAI refresh-token grant and configured endpoint override
- [X] T016 Reuse `UpdateOpenAISubscriptionCredential` for successful forced refresh, preserving rotated/omitted refresh-token behavior
- [X] T017 Reuse `UpdateOpenAISubscriptionCredentialFailure` for refresh failure metadata
- [X] T018 Deduplicate concurrent forced refreshes for the same credential or reuse latest stored refreshed token
- [X] T019 Run forced-refresh tests and confirm pass

**Checkpoint**: A fresh local token can be force-refreshed safely after upstream 401.

## Phase 3: Direct Handler 401 Retry Loop

**Goal**: Add same-credential retry and failover to direct OpenAI handlers.

- [X] T020 Update `doOpenAISubscriptionProviderRequest` to detect 401 only when `attempt.credentialID` is non-empty
- [X] T021 On first 401 for a credential, close/discard response body before refresh
- [X] T022 Force-refresh that credential and rebuild the upstream request with refreshed bearer
- [X] T023 Retry the same credential once with the refreshed bearer
- [X] T024 On refreshed retry success or non-401 final response, return response without trying next credential
- [X] T025 On refresh failure, record reason and continue to next credential
- [X] T026 On refreshed retry 401, persist auth-failed disabled metadata and continue to next credential
- [X] T027 Return sanitized reauthorization-required error when every credential fails auth/refresh
- [X] T028 Preserve existing 429/5xx retry and rate-limit state behavior
- [X] T029 Run direct-handler 401 tests and existing HO-1179 routing tests

**Checkpoint**: Direct official OpenAI handlers satisfy HO-1180 401 behavior.

## Phase 4: Proxy Transport 401 Retry Loop

**Goal**: Apply the same behavior to proxy-style official OpenAI endpoints.

- [X] T030 Update `openAISubscriptionFailoverTransport` to detect 401 for subscription candidates
- [X] T031 Rebuild proxy requests with buffered body and refreshed bearer
- [X] T032 Ensure endpoint-specific headers such as `OpenAI-Beta` behavior remain unchanged where applicable
- [X] T033 On proxy refreshed retry 401, persist auth-failed disabled metadata and try next candidate
- [X] T034 Ensure proxy transport still does not retry after response body is returned to caller
- [X] T035 Run proxy-transport 401 tests and responses endpoint regression tests

**Checkpoint**: `/v1/responses` and sibling proxy endpoints share HO-1180 behavior.

## Phase 5: Metadata, Errors, and Redaction

**Goal**: Make failures actionable without leaking secrets.

- [X] T036 Add helper for auth-failed-after-refresh metadata if existing failure helper is too generic
- [X] T037 Use stable `disabled_reason=auth_failed_after_refresh` for retry-still-401
- [X] T038 Redact upstream 401 body text before storing or returning it
- [X] T039 Build all-failed reauthorization-required error with stable reason codes
- [X] T040 Assert response/log/metadata do not include access tokens, refresh tokens, bearer strings, JWTs, fallback API key, or raw upstream sensitive body
- [X] T041 Run redaction and metadata tests

**Checkpoint**: Safe metadata and all-failed errors are covered.

## Phase 6: Final Verification

**Purpose**: Prove HO-1180 does not regress sibling OpenAI subscription behavior.

- [X] T042 Run targeted HO-1180 tests:
  `go test ./internal/proxy/handler/... -run 'TestOpenAISubscriptionRouting_401|TestOpenAISubscriptionProxyTransport_401|TestForceRefreshOpenAISubscriptionCredential|TestOpenAISubscriptionCredential_AuthFailureMetadata' -count=1 -v`
- [X] T043 Run broader OpenAI subscription routing tests:
  `go test ./internal/proxy/handler/... -run 'TestOpenAISubscriptionRouting|TestOpenAISubscriptionEndpoints|TestResolveOpenAISubscription' -count=1 -v`
- [X] T044 Run affected package tests:
  `go test ./internal/proxy/handler/... ./internal/provider/openai/... ./internal/testutil/openaitest/... -count=1`
- [X] T045 Run `git diff --check origin/main...HEAD`
- [X] T046 Verify PR diff includes only HO-1180 implementation files and `specs/1180-openai-401-retry-refresh-disable/`
- [X] T047 Update SpecKit tasks to `[X]` only as implementation tasks complete
- [X] T048 Move Linear to In Review only after implementation gates pass and production code is pushed

## Dependencies and Parallelism

- T001-T010 can be written in parallel by test surface.
- T012-T019 must land before retry-loop implementation.
- T020-T029 and T030-T035 share forced-refresh helper and should coordinate to avoid duplicated logic.
- T036-T041 can proceed after the retry-still-401 metadata shape is chosen.
- T042-T048 are final verification.

## Scope Stop

This Todo planning PR stops at SpecKit artifacts and draft PR review. Production implementation starts only after Linear HO-1180 moves to In Progress.
