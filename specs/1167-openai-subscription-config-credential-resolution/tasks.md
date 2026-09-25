# Tasks: OpenAI Subscription Config and Credential Resolution

**Input**: SpecKit artifacts in `specs/1167-openai-subscription-config-credential-resolution/`
**Prerequisites**: Linear state moved from Waiting to In Progress

## Phase 0: Governance

- [x] T001 Confirm Linear state is In Progress before production implementation.
- [x] T002 Confirm implementation continues in this existing issue worktree/branch.
- [x] T003 Re-verify OpenAI Codex app-server protocol version for `chatgptAuthTokens`.

## Phase 1: Config Tests First

- [x] T004 Add `TestLoad_OpenAISubscriptionCredentialIDs` in `internal/config/loader_test.go`.
- [x] T005 Add `TestLoad_OpenAISubscriptionIDsEmptyPreservesAPIKey` in `internal/config/loader_test.go`.
- [x] T006 Add `TestValidate_OpenAISubscriptionRejectsCustomAPIBase` in `internal/config/loader_test.go`.
- [x] T007 Add `TestValidate_OpenAISubscriptionRejectsCustomProvider` in `internal/config/loader_test.go`.
- [x] T008 Add `TestValidate_OpenAISubscriptionRejectsEmptyOrDuplicateIDs` in `internal/config/loader_test.go`.
- [x] T009 Run config tests for the new coverage.

## Phase 2: Config Implementation

- [x] T010 Add `OpenAISubscriptionCredentialIDs []string` to `config.TianjiParams`.
- [x] T011 Keep existing env interpolation for `api_key`, `api_base`, and `api_version` unchanged.
- [x] T012 Add targeted validation for subscription ID constraints.
- [x] T013 Run `go test ./internal/config/... -run 'TestLoad_OpenAISubscription|TestValidate_OpenAISubscription' -v`.

## Phase 3: Resolver Tests First

- [x] T014 Add `TestResolveOpenAISubscriptionCredential_ExplicitIDsOnly`.
- [x] T015 Add `TestResolveOpenAISubscriptionCredential_NoAPIKeyFallbackOnFailure`.
- [x] T016 Add `TestResolveProvider_NoSubscriptionKeepsAPIKeyPath`.
- [x] T017 Add `TestDirectOpenAIHTTPResolution_ReturnsBearerMaterial`.
- [x] T018 Run resolver tests for the new coverage.

## Phase 4: Resolver Implementation

- [x] T019 Add typed resolver input/output structs near the handler/router boundary.
- [x] T020 Implement exact credential ID lookup using existing sqlc-backed credential helpers.
- [x] T021 Require `credential_type = "openai_subscription"`.
- [x] T022 Refresh or reject expired credentials according to existing OpenAI subscription helper contract.
- [x] T023 Return sanitized explicit errors for missing, disabled, expired, malformed, or unusable credentials.
- [x] T024 Preserve API-key path when subscription IDs are omitted.
- [x] T025 Prevent API-key fallback when subscription IDs are configured.
- [x] T026 Run targeted resolver tests.

## Phase 5: Codex App-Server Tests First

- [x] T027 Add `TestCodexAppServerResolution_UsesChatGPTAuthTokensLogin`.
- [x] T028 Add `TestCodexAppServerEnv_StripsOpenAIKeysForSubscriptionProfile`.
- [x] T029 Add `TestCodexAppServerWebSocket_ConnectionAuthSeparatedFromOpenAIAuth`.
- [x] T030 Run app-server auth tests for the new coverage.

## Phase 6: Codex App-Server Implementation

- [x] T031 Add app-server auth helper package or handler-local helper.
- [x] T032 Build `account/login/start` payload for `chatgptAuthTokens`.
- [x] T033 Ensure subscription credentials are not applied as app-server HTTP bearer headers.
- [x] T034 Strip `CODEX_API_KEY` and `OPENAI_API_KEY` from local stdio env for subscription-style profiles.
- [x] T035 Keep WebSocket `appServer.authToken`/headers separate from OpenAI account login RPC.
- [x] T036 Run app-server auth tests.

## Phase 7: Integration Verification

- [x] T037 Run `go test ./internal/config/... ./internal/proxy/handler/... ./internal/router/... -v`.
- [x] T038 Run any app-server package tests introduced by implementation.
- [x] T039 Run `git diff --check`.
- [x] T040 Verify no raw token/account material appears in logs or error responses; reuse HO-1183 redaction helpers.
- [x] T041 Update SpecKit docs if implementation chooses a different package boundary.
- [x] T042 Push branch and update draft PR only after normal branch/PR workflow is allowed.
