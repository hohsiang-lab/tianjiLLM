# Tasks: Subscription wildcard routing regression verification

**Input**: SpecKit artifacts in `specs/1292-add-regression-verification-for-subscription-wildcard-routing/`
**Prerequisites**: Linear HO-1292 must move to `In Progress` before production or test implementation.

## Phase 0: Governance

- [X] T001 Confirm Linear HO-1292 is `In Progress` before editing production or test files.
- [X] T002 Confirm work continues in `worktrees/tianjiLLM/HO-1292-add-regression-verification-for-subscription`.
- [X] T003 Re-read Linear description, `spec.md`, `plan.md`, and current Codex transport code before implementation.
- [X] T004 Confirm branch diff before implementation contains only HO-1292 SpecKit docs.

## Phase 1: RED tests first

- [X] T005 Add `TestChatGPTCodexBackendWildcard_RoutesSubscriptionOpenAIWildcardToCodexBackend`.
- [X] T006 Add mocked Codex backend request recorder asserting `/backend-api/codex/responses`.
- [X] T007 Add Platform wrong-route recorder asserting zero `/v1/chat/completions` calls for subscription wildcard.
- [X] T008 Add bearer sentinel assertion proving subscription access token is not sent to Platform chat completions.
- [X] T009 Add table-driven wildcard diagnostics tests for Codex backend 401 auth failure.
- [X] T010 Add table-driven wildcard diagnostics tests for Codex backend 403 missing-scope failure.
- [X] T011 Add table-driven wildcard diagnostics tests for Codex backend 429 quota/rate-limit failure.
- [X] T012 Add API-key-backed `openai/*` wildcard regression proving normal `/chat/completions` route remains unchanged.
- [X] T013 Run targeted tests and record RED results before any implementation fix.

Implementation note: after adding the RED-first regression tests, `go test ./internal/proxy/handler -run 'TestChatGPTCodexBackendWildcard|TestOpenAIAPIKeyWildcard|TestChatGPTCodexBackendTransport' -count=1` passed before any production fix. The current HO-1288..HO-1291 implementation already routes subscription-backed `openai/*` through `chatgpt_codex_backend`; HO-1292's issue-owned delta is regression coverage plus deployment verification notes.

## Phase 2: Implementation only if RED exposes a real gap

- [X] T014 If subscription wildcard does not set `ChatGPTCodexBackend`, patch only the route/transport selection boundary.
- [X] T015 If wrong-route prevention is missing, add the minimal guard required to prevent subscription bearer material reaching Platform chat completions.
- [X] T016 If diagnostics are rewritten, patch only error/status propagation needed for 401/403/429 through wildcard routing.
- [X] T017 If API-key wildcard regresses, restore API-key route behavior without changing subscription Codex transport.
- [X] T018 Keep credential resolver, OAuth lifecycle, Models UI selector, and provider payload mapper unchanged unless a failing test directly proves they are the root cause.

Implementation note: T014-T017 required no production patch because the new handler-boundary tests passed against current code. T018 was preserved; credential resolver, OAuth lifecycle, Models UI selector, and provider payload mapper were not changed.

## Phase 3: Deployment verification note

- [X] T019 Add or update quickstart docs with OpenClaw `openai/*` rollout verification.
- [X] T020 Document safe path/status checks for Codex backend routing without printing bearer values.
- [X] T021 Document failure classification: wrong Platform route, 401 auth, 403 missing scope, 429 quota, API-key fallback.
- [X] T022 Add negative secret-safety checklist for tokens/API keys/raw credential JSON.

## Phase 4: Regression and security verification

- [X] T023 Run `go test ./internal/proxy/handler -run 'TestChatGPTCodexBackendWildcard|TestOpenAIAPIKeyWildcard|TestChatGPTCodexBackendTransport' -count=1`.
- [X] T024 Run `go test ./internal/provider/chatgptcodex ./internal/provider/openai ./internal/config -count=1`.
- [X] T025 Run `go test ./test/contract -run 'TestWildcard|TestResponsesCreate' -count=1`.
- [X] T026 Run `git diff --check origin/main...HEAD`.
- [X] T027 Verify PR diff contains only HO-1292 docs/tests and any minimal in-scope implementation fix.
- [X] T028 Confirm no test, doc, log, or PR text prints access token, refresh token, API key, bearer header, encrypted credential value, or raw credential JSON.

Verification note: targeted handler/provider/config/contract tests passed. Diff scope is HO-1292 SpecKit artifacts plus `internal/proxy/handler/chatgpt_codex_wildcard_test.go`; no production implementation files changed. Secret-safety scan found only fake sentinel strings and negative/redaction assertions; failure messages avoid printing bearer values.

## Phase 5: State progression

- [ ] T029 After implementation tasks complete, move Linear to `In Review` and run review gate.
- [ ] T030 After review pass and CI policy handling, move through `Waiting CI` and then `Waiting Merge` only when gates pass.

## Scope Stop

After Todo planning, stop at `Waiting` with a docs-only draft PR. Implementation remains blocked until Linear moves to `In Progress`.
