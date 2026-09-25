# Tasks: OpenAI Connect localhost paste flow and remove `public_base_url`

**Input**: `spec.md`, `plan.md`, Linear HO-1259, current `origin/main` repo reality.

## Phase 1 - RED Tests First

- [x] T001 Add failing config test `TestResolveOpenAIOAuthRedirectURI_DefaultsToLocalhostCallback` in `internal/config/openai_oauth_test.go`.
- [x] T002 Add failing config loader regression `TestLoadWithOpenAIOAuthConfig_DefaultsLocalhostRedirect` in `internal/config/loader_test.go`.
- [x] T003 Add failing state-store test `TestStateStore_CreateStoresRedirectURI` in `internal/openaioauth/state_store_test.go`.
- [x] T004 Add failing UI handler test `TestHandleOpenAIConnect_UsesConfiguredRedirectURIAndStoresIt` in `internal/ui/handler_openai_test.go`.
- [x] T005 Add failing UI render test `TestCredentialsOpenAIConnectLaunch_OpensAuthorizeInNewTab` proving the protected Tianji page remains available while OpenAI authorize opens in a new tab/window.
- [x] T006 Add failing UI render test `TestCredentialsOpenAIPasteCallbackModal_RendersProtectedSubmitForm` proving `/ui/credentials` has a paste modal trigger/form.
- [x] T007 Add failing callback handler test `TestOpenAIOAuthCallback_UsesStoredRedirectURIForExchange` in `internal/proxy/handler/openai_oauth_test.go`.
- [x] T008 Add failing pasted callback parser tests for valid localhost URL, malformed URL, unexpected host/path, provider error, missing state, and missing code.
- [x] T009 Add failing pasted callback handler test for successful full URL paste consuming state and saving credential.
- [x] T010 Add failing pasted callback handler test proving invalid pasted URL does not leak raw URL/code/state into HTML/errors/audit metadata.
- [x] T011 Add failing E2E `TestOpenAIConnectFlow_LocalhostPasteSuccessWithMockOAuth` using `internal/testutil/openaitest`.
- [x] T012 Run targeted RED commands and record expected failures in PR body before implementation.

## Phase 2 - Redirect Config Cleanup

- [x] T013 Add explicit OpenAI OAuth redirect URI config under `OpenAIOAuthConfig`, with default `http://localhost:1455/auth/callback`.
- [x] T014 Replace `DeriveOpenAIRedirectURI` / `ValidateOpenAIRedirectURI` OpenAI OAuth runtime usage with explicit redirect resolution.
- [x] T015 Remove `GeneralSettings.PublicBaseURL` if no non-OpenAI live runtime usage remains; otherwise remove it from OpenAI OAuth path and document any unrelated retained usage.
- [x] T016 Update config loader tests/fixtures/docs and any `proxy.yml` / proxy YAML config artifacts so OpenAI OAuth no longer requires or includes `public_base_url`.

## Phase 3 - State Record Redirect Identity

- [x] T017 Extend `openaioauth.StateRecord` with `RedirectURI`.
- [x] T018 Change `StateStore.Create` to accept and persist redirect URI.
- [x] T019 Update `StateRecord.validFor` to require exact non-empty redirect URI.
- [x] T020 Update existing state-store and connect tests for the new `Create` signature.

## Phase 4 - Connect Flow

- [x] T021 Update `handleOpenAIConnect` to resolve redirect URI from `OpenAIOAuthConfig`, create state with that URI, and pass `record.RedirectURI` to `BuildAuthorizeURL`.
- [x] T022 Update connect handler error messages so config/cache failures remain safe and do not mention pasted codes or raw URLs.
- [x] T023 Update E2E helper `configureOpenAIOAuthMock` to stop setting `cfg.GeneralSettings.PublicBaseURL`.

## Phase 5 - Shared Callback Completion

- [x] T024 Extract direct callback completion into a shared helper that accepts parsed state/code/provider-error fields.
- [x] T025 Ensure shared helper consumes state once, handles provider error/missing code safely, and uses `record.RedirectURI` for `ExchangeCode`.
- [x] T026 Preserve direct `GET /oauth/openai/callback` behavior by routing it through the shared helper.
- [x] T027 Ensure save/audit uses stored `record.OrgID` only and never trusts query/form org values.

## Phase 6 - Hosted Paste Flow UI/Endpoint

- [x] T028 Register protected UI routes for pasted callback URL, e.g. `GET /ui/openai/callback-url` for modal partial/fallback and `POST /ui/openai/callback-url` for submit.
- [x] T029 Render a Tianji-styled `Paste callback URL` button/modal on `/ui/credentials`, using the existing `dialog` component pattern.
- [x] T030 Keep `Connect OpenAI` opening `/ui/openai/connect?...` in a new tab/window via `target="_blank"` or equivalent and `rel="noopener"`.
- [x] T031 Parse `callback_url` server-side with a narrow helper that validates scheme, host, port, path, and query shape against the configured/stored redirect contract.
- [x] T032 Submit valid pasted URL through shared callback completion and render the same safe success/failure surface.
- [x] T033 Ensure the modal/form never echoes raw pasted URL, code, state, verifier, or token material after submit.

## Phase 7 - Cleanup and Verification

- [x] T034 Run `rg "PublicBaseURL|public_base_url|DeriveOpenAIRedirectURI|ValidateOpenAIRedirectURI" internal test cmd config || true` plus a proxy YAML scan, and remove live OpenAI OAuth / `proxy.yml` references.
- [x] T035 Run targeted Go tests for config/state/UI/proxy/provider.
- [x] T036 Run E2E command for OpenAI connect/callback flow or document exact environment gate if local E2E services are unavailable.
- [x] T037 Run `git diff --check origin/main...HEAD`.
- [x] T038 Update PR body with root cause, redirect identity evidence, test plan, and secret redaction evidence.

## Dependency Notes

- T001-T012 must be completed before production implementation.
- T017-T020 must land before callback exchange can safely stop recomputing redirect URI.
- T024-T027 must land before adding paste endpoint, so direct and pasted callback behavior cannot diverge.
- T034 is a blocking cleanup gate before moving to In Review.

## Implementation Evidence

- RED command before implementation: `go test ./internal/config ./internal/openaioauth ./internal/ui ./internal/ui/pages ./internal/proxy/handler -run 'Test.*OpenAI|Test.*OAuth|TestCredentialsOpenAI' -count=1` failed as expected because `RedirectURI`, stored state redirect URI, paste parser, and pasted callback handler did not exist yet.
- Green targeted command: `go test ./internal/config ./internal/openaioauth ./internal/ui ./internal/ui/pages ./internal/proxy ./internal/proxy/handler -run 'Test.*OpenAI|Test.*OAuth|TestCredentialsOpenAI|TestStateStore' -count=1`.
- Green broader command: `go test ./internal/...`.
- Green lint: `make lint`.
- Green whitespace gate: `git diff --check origin/main...HEAD`.
- Cleanup scan: `rg "PublicBaseURL|public_base_url|DeriveOpenAIRedirectURI|ValidateOpenAIRedirectURI" internal test cmd test/fixtures || true` returned no matches; proxy YAML scan found only `test/fixtures/proxy_config_full.yaml` and `test/fixtures/mcp/proxy_config_mcp.yaml`.
- Local E2E command attempted: `go test -tags e2e ./test/e2e -run 'TestOpenAIConnectFlow_LocalhostPasteSuccessWithMockOAuth|TestOpenAIConnectFlow_AuthenticatedSuccessWithMockOAuth|TestOpenAICallback_FailurePagesAreReadableAndSafe' -count=1`; local runner stopped at setup because `E2E_DATABASE_URL` is required, so CI must execute the E2E suite.
