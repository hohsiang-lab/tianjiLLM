# Tasks: ChatGPT Codex backend transport

**Input**: `spec.md`, `plan.md`, Linear HO-1288, current `origin/main` repo reality.
**Prerequisite**: Linear HO-1288 must move to `In Progress` before editing production files.

## Phase 0 - Governance

- [x] T001 Confirm Linear HO-1288 is `In Progress` before production edits.
- [x] T002 Confirm work continues in `worktrees/tianjiLLM/HO-1288-chatgpt-codex-backend-transport`.
- [x] T003 Re-read Linear description, this specs directory, and current `internal/provider/openai`, `internal/proxy/handler/openai_subscription_*`, and `internal/config/openai_oauth.go`.
- [x] T004 Confirm branch diff before implementation contains only HO-1288 SpecKit docs.

## Phase 1 - RED Tests First

- [x] T005 Add failing test proving explicit `chatgpt_codex_backend` transport sends POST to `/backend-api/codex/responses`.
- [x] T006 Add failing test proving the same transport does not call `/v1/chat/completions`.
- [x] T007 Add failing test proving headers include `Authorization: Bearer <access_token>`.
- [x] T008 Add failing test proving `ChatGPT-Account-Id` is sent when account ID exists and omitted/safe when absent.
- [x] T009 Add failing test proving default `originator` is `codex_cli_rs`.
- [x] T010 Add failing config test proving originator override is honored.
- [x] T011 Add failing config/test override proving backend base URL can point to `httptest.Server`.
- [x] T012 Add failing body-mapping test proving chat messages map to backend Responses-style `input`.
- [x] T013 Add failing non-streaming response adapter test.
- [x] T014 Add failing streaming/SSE adapter test or explicit safe unsupported streaming test.
- [x] T015 Add failing credential failure test proving explicit backend subscription config does not fallback to API key.
- [x] T016 Add failing leakage test with sentinel access/refresh tokens and credential JSON.
- [x] T017 Add API-key OpenAI provider regression test proving `/v1/chat/completions` behavior remains unchanged.
- [x] T018 Run targeted RED commands and record expected failures.

## Phase 2 - Config and Transport Selection

- [x] T019 Add explicit transport selection field or equivalent repo-native enum for `chatgpt_codex_backend`.
- [x] T020 Validate `chatgpt_codex_backend` requires non-empty `openai_subscription_credential_ids`.
- [x] T021 Validate custom `api_base` does not accidentally override the ChatGPT backend URL.
- [x] T022 Add backend base URL config/default and normalization to `/codex/responses`.
- [x] T023 Add backend originator config/default separate from OAuth authorize originator.
- [x] T024 Update config loader tests for defaults and overrides.

## Phase 3 - Credential Resolution

- [x] T025 Extend subscription resolver transport enum or add equivalent backend resolver path.
- [x] T026 Reuse existing credential attempt ordering, disabled/malformed/missing handling, sticky/failover, and rate-limit gates.
- [x] T027 Return bearer access token and account ID for backend transport.
- [x] T028 Preserve `codex_app_server` login payload behavior unchanged.
- [x] T029 Preserve `direct_openai_http` bearer behavior unchanged.

## Phase 4 - Backend Request/Response Transport

- [x] T030 Add backend transport builder for URL, headers, and request body.
- [x] T031 Map chat completion messages to backend Responses-style input.
- [x] T032 Normalize model names for backend requests.
- [x] T033 Send required headers and conditional account header.
- [x] T034 Send request through injectable/testable HTTP client or URL override.
- [x] T035 Map non-streaming backend response to Tianji/OpenAI-compatible response.
- [x] T036 Map streaming backend SSE to Tianji streaming chunks or return explicit safe unsupported error per T014 decision.
- [x] T037 Map backend error responses without leaking secrets.

## Phase 5 - Runtime Integration

- [x] T038 Route selected chat completion requests through the backend transport.
- [x] T039 Route selected `/v1/responses` requests through the backend transport if implementation chooses responses support in this slice. Not selected in this slice; chat completion requests map into backend Responses payload.
- [x] T040 Ensure existing OpenAI endpoint proxy remains Platform `/v1` for non-selected routes.
- [x] T041 Ensure DB-managed runtime model rows from HO-1285 can carry the transport config.
- [x] T042 Ensure API-key-only routes continue to use existing provider behavior.

## Phase 6 - Regression and Security

- [x] T043 Run targeted backend transport tests.
- [x] T044 Run existing OpenAI subscription resolver/routing tests.
- [x] T045 Run existing OpenAI provider tests.
- [x] T046 Run affected handler/config/provider tests.
- [x] T047 Run `go tool golangci-lint run` or repo lint command.
- [x] T048 Run `git diff --check origin/main...HEAD`.
- [x] T049 Verify PR diff contains only HO-1288 implementation files and this specs directory.

## Phase 7 - State Progression

- [x] T050 Update SpecKit implementation evidence.
- [ ] T051 Move Linear to `In Review` and run review gate after implementation tasks complete.

## Implementation Evidence

- RED failure recorded before implementation: `go test ./internal/config ./internal/proxy/handler -run 'TestOpenAIOAuthConfig_CodexBackendOverrides|TestLoad_OpenAISubscriptionTransportChatGPTCodexBackend|TestChatGPTCodexBackendTransport' -count=1` failed on missing config/transport fields.
- Targeted GREEN: `go test ./internal/config ./internal/proxy/handler -run 'TestOpenAIOAuthConfig_CodexBackendOverrides|TestLoad_OpenAISubscriptionTransportChatGPTCodexBackend|TestChatGPTCodexBackendTransport' -count=1`.
- Affected packages GREEN: `go test ./internal/config ./internal/provider/openai ./internal/provider/chatgptcodex ./internal/proxy/handler -count=1`.
- Full internal tests GREEN: `go test ./internal/... -count=1`.
- Lint GREEN: `go tool golangci-lint run` returned `0 issues`.
- Whitespace gate GREEN: `git diff --check origin/main...HEAD`.

## Scope Stop

After Todo planning, stop at `Waiting` with a docs-only draft PR. Waiting Merge is blocked until Linear moves through `In Progress`, `In Review`, `Waiting CI`, and CI/result gates.
