# Tasks: HO-1321 Codex Responses WebSocket adapter

## Phase 1 - RED Coverage

- [x] T001 Re-read Linear HO-1321, this SpecKit package, and HO-1319 artifacts before implementation.
- [x] T002 Add failing handler test for Codex prewarm `response.create` with `generate:false`, `tools: []`, and `OpenAI-Beta: responses_websockets=2026-02-06`.
- [x] T003 Assert the prewarm path does not forward `generate` to ChatGPT Codex backend.
- [x] T004 Assert prewarm emits or records a response id usable by same-connection follow-up.
- [x] T005 Add failing handler test for follow-up `response.create` with `previous_response_id=<warmup id>` and `input: []`.
- [x] T006 Assert follow-up does not forward an empty/incomplete backend request; backend must receive a valid reconstructed payload.
- [x] T007 Add failing handler test for a Codex-like WebSocket frame larger than 32KiB.
- [x] T008 Add failure assertion for missing cached `previous_response_id` under `store:false`/local-only state.
- [x] T009 Add no-regression assertion for existing small sequential WebSocket frame behavior.

## Phase 2 - WebSocket Capacity

- [x] T010 Set a bounded WebSocket read limit after `websocket.Accept` in `/v1/responses`.
- [x] T011 Document the chosen limit in code or test name with the Codex frame rationale.
- [x] T012 Confirm oversized payloads still fail safely above the chosen bound without leaking request content.

## Phase 3 - Prewarm Adapter

- [x] T013 Introduce a connection-local Responses WebSocket adapter state helper.
- [x] T014 Detect `generate:false` frames before building backend requests.
- [x] T015 Strip/consume `generate` at the Tianji boundary; never forward it to `chatgptcodex.Transport`.
- [x] T016 Store warmup request state and warmup response id in connection-local cache.
- [x] T017 Return WebSocket-compatible warmup events or pass through safe backend warmup events according to the selected design.

## Phase 4 - State Chaining

- [x] T018 Resolve `previous_response_id` against connection-local cache.
- [x] T019 Rebuild backend-valid payload for `previous_response_id + input: []` follow-up from cached warmup state.
- [x] T020 Merge non-empty incremental input safely when present.
- [x] T021 Return redacted `previous_response_not_found`-style error when state is absent and no full input context exists.
- [x] T022 Keep one-in-flight sequential semantics on the connection.

## Phase 5 - Route/Regression Guardrails

- [x] T023 Preserve DB-managed `openai/*` + `openai/gpt-5.5` routing through `chatgpt_codex_backend`.
- [x] T024 Preserve HTTP `/v1/responses` behavior from HO-1319.
- [x] T025 Preserve `/v1/chat/completions` streaming behavior from HO-1314.
- [x] T026 Confirm no direct OpenAI/API-key route is redirected through Codex backend unintentionally.
- [x] T027 Confirm no token, refresh token, account secret, or bearer header appears in test fixtures, failures, PR body, or logs.

## Phase 6 - Verification

- [x] T028 Run `go test ./internal/proxy/handler -run 'CreateResponseWebSocket|Responses|Codex' -count=1`.
- [ ] T029 Run `GOWORK=off go test -tags e2e -count=1 -run 'Codex.*Responses|CodexSubscriptionRoute' ./test/e2e`. Blocked locally: repo default e2e DSN points at host `localhost:5433`, but this machine already has an unrelated postgres listener on `5433`, so the test process does not reach the issue-scoped compose postgres.
- [x] T030 Run package/service tests required by touched files.
- [x] T031 Run `git diff --check`.
- [ ] T032 Live verify true `codex exec` against Tianji returns `OK` without `Reconnecting...` noise. Blocked until this branch is deployed to the Tianji live route; local `codex exec` config does not target the PR branch.
- [ ] T033 Record verification evidence in PR and Linear/thread without secrets.
