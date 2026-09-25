# Tasks: HO-1340 Codex usage snapshot

**Input**: Design documents from `/specs/1340-add-codex-usage-snapshot-for-openai-subscription/`
**Prerequisites**: Linear state must move to In Progress before production implementation.

## Phase 1 - RED Coverage

- [x] T001 Re-read Linear HO-1340, this SpecKit package, and current OpenAI subscription credential resolution/UI code before implementation.
- [x] T002 Add failing provider helper test proving `GET /wham/usage` is built with bearer auth, account id header, and Codex usage headers.
- [x] T003 Add failing provider parser test for primary 5h, weekly, and additional Codex bucket normalization.
- [x] T004 Add failing provider parser test proving missing optional buckets do not drop known primary/weekly data.
- [x] T005 Add failing service test proving repeated calls inside TTL make one upstream request.
- [x] T006 Add failing service test proving 401 refreshes the credential once and retries once.
- [ ] T007 Add failing service test proving repeated 401 after refresh returns a safe auth error.
- [x] T008 Add failing service test proving 429 and 5xx activate exponential backoff with jitter and preserve last successful snapshot.
- [ ] T009 Add failing test proving missing, disabled, malformed, wrong-type, and DB-unavailable credentials return safe states.
- [ ] T010 Add failing no-proxy-fetch regression proving normal `/v1/chat/completions`, `/v1/responses`, and WebSocket Codex requests do not call `wham/usage`.
- [x] T011 Add failing credential detail UI test for rendering a successful Codex usage snapshot.
- [ ] T012 Add failing credential detail UI test for stale/backoff/no-data/error states.
- [x] T013 Add failing `/ui/usage?tab=codex` and `/ui/usage/tab?tab=codex` tests for all subscription credentials.
- [ ] T014 Add failing manual refresh UI/API tests proving cache/backoff is respected.
- [ ] T015 Add failing redaction tests proving no token, cookie, authorization header, encrypted blob, raw body, or token-shaped upstream error reaches DB, JSON, HTML, logs, or audit metadata.

## Phase 2 - Provider Helper

- [x] T016 Add `internal/provider/chatgptcodex/usage.go` with explicit request/client/snapshot/window/bucket types.
- [x] T017 Build the usage endpoint URL from ChatGPT Codex backend base URL, defaulting to `https://chatgpt.com/backend-api/wham/usage`.
- [x] T018 Apply `Authorization: Bearer <access_token>`, `ChatGPT-Account-Id` when non-empty, and Codex usage-compatible referer/originator headers.
- [x] T019 Parse successful upstream JSON into normalized non-secret snapshot structs.
- [x] T020 Normalize additional model buckets by stable bucket name.
- [x] T021 Map non-200 responses to safe reason codes without storing raw body.

## Phase 3 - Service, Cache, and Backoff

- [x] T022 Add OpenAI subscription Codex usage service near existing subscription lifecycle code.
- [x] T023 Reuse `resolveUsableOpenAISubscriptionBundle` / existing credential resolution for initial token access.
- [x] T024 Reuse the existing force-refresh helper for 401 refresh-once retry.
- [x] T025 Implement per-credential cache with TTL >= 60 seconds.
- [x] T026 Implement per-credential exponential backoff with jitter for 429 and 5xx.
- [x] T027 Preserve last successful normalized snapshot while backoff or transient errors are active.
- [x] T028 Keep missing/disabled/malformed credential handling safe and per-credential.
- [x] T029 Ensure service has injectable clock/jitter/client hooks for deterministic tests.
- [x] T030 Ensure service/cache never persists or exposes raw upstream request/response material.

## Phase 4 - Credential Detail UI

- [x] T031 Extend credential detail data model with Codex usage view data.
- [x] T032 Load the credential's Codex usage snapshot in `handleCredentialDetail` / `buildCredentialDetail` without blocking on proxy request paths.
- [x] T033 Render a Codex usage section near existing quota dimensions.
- [x] T034 Add manual refresh action for the credential detail page.
- [x] T035 Render stale, backoff, disabled, no-data, auth-error, and transient-error states safely.

## Phase 5 - `/ui/usage` Codex Tab

- [x] T036 Add `codex` to `activeTab` in `internal/ui/handler_usage.go`.
- [x] T037 Add a Codex usage loader that enumerates OpenAI subscription credentials from DB.
- [x] T038 Add `CodexUsageTabData` and normalized card/row structs under `internal/ui/pages`.
- [x] T039 Add `UsageCodexTab` templ component with all credential cards/rows.
- [x] T040 Add Codex tab trigger to `internal/ui/pages/usage.templ`.
- [x] T041 Add HTMX tab support for `/ui/usage/tab?tab=codex`.
- [x] T042 Add manual refresh action for each credential in the Codex tab.
- [x] T043 Regenerate templ output with `make ui`.

## Phase 6 - Safety and Regression Gates

- [ ] T044 Prove no `wham/usage` call is made during regular proxy requests.
- [x] T045 Prove all new UI/admin routes are protected by existing UI/session auth.
- [x] T046 Prove wrong-type credentials cannot be used for Codex usage fetch.
- [ ] T047 Prove raw upstream response body is not stored even when parse fails.
- [ ] T048 Prove logs/audit metadata contain only safe reason codes and redacted context.
- [x] T049 Prove existing Claude Code tab behavior remains unchanged.
- [x] T050 Prove existing OpenAI subscription credential lifecycle tests remain green.

## Phase 7 - Verification

- [x] T051 Run `go test ./internal/provider/chatgptcodex -run 'Usage|Wham|CodexUsage' -count=1`.
- [x] T052 Run `go test ./internal/proxy/handler -run 'OpenAISubscription.*Usage|CodexUsage|CredentialDetail' -count=1`.
- [x] T053 Run `go test ./internal/ui -run 'CodexUsage|CredentialDetail|UsageTab' -count=1`.
- [x] T054 Run `go test ./internal/ui/pages -run 'CodexUsage|Usage' -count=1`.
- [x] T055 Run existing OpenAI subscription routing/lifecycle tests affected by the service.
- [x] T056 Run `make ui` when templ files change.
- [x] T057 Run `git diff --check`.
- [ ] T058 Verify implementation PR diff contains production/test files plus this SpecKit package only.
- [ ] T059 After deploy, manually refresh Codex usage for a live non-secret credential and verify UI shows normalized fields only.
- [ ] T060 Record verification evidence in PR and Linear/thread without secrets.

## Scope Stop

Linear HO-1340 must move out of Todo before production implementation starts. Todo output is this docs-only SpecKit package plus scope confirmation.
