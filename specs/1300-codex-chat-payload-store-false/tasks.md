# Tasks: Codex backend chat payload `store:false`

**Input**: SpecKit artifacts in `specs/1300-codex-chat-payload-store-false/`
**Prerequisites**: Linear HO-1300 must move to `In Progress` before production or test implementation.

## Phase 0: Governance

- [ ] T001 Confirm Linear HO-1300 is `In Progress` before editing production or test files.
- [ ] T002 Confirm work continues in `worktrees/tianjiLLM/HO-1300-codex-chat-store-false`.
- [ ] T003 Re-read Linear description, `spec.md`, `plan.md`, current Codex payload code, and existing HO-1288/HO-1292 tests before implementation.
- [ ] T004 Confirm branch diff before implementation contains only HO-1300 SpecKit docs.

## Phase 1: RED tests first

- [ ] T005 Add `TestBuildPayload_IncludesStoreFalse`.
- [ ] T006 Add `TestBuildPayload_AllowsClientStoreFalse`.
- [ ] T007 Add `TestBuildPayload_RejectsClientStoreTrue`.
- [ ] T008 Add `TestBuildPayload_RejectsInvalidStoreType`.
- [ ] T009 Add handler/wildcard test proving client `store:false` succeeds through mocked `chatgpt_codex_backend`.
- [ ] T010 Add handler/wildcard test proving outgoing backend body contains `"store": false`.
- [ ] T011 Add handler/wildcard test proving client `store:true` returns HTTP 400 before upstream.
- [ ] T012 Add handler/wildcard test proving `store:false` plus another unknown parameter still rejects unknown parameters.
- [ ] T013 Add normal OpenAI provider no-regression test for `ExtraParams["store"] = false` pass-through.
- [ ] T014 Run targeted tests and record RED results before implementation fix.

## Phase 2: Minimal implementation

- [ ] T015 Add non-omitempty `Store bool` field to `chatgptcodex.Payload`.
- [ ] T016 Initialize Codex payload `Store` to `false`.
- [ ] T017 Add Codex-only validation that accepts `ExtraParams["store"] == false`.
- [ ] T018 Add Codex-only validation that rejects `store:true` and non-boolean `store`.
- [ ] T019 Preserve strict rejection for all other Codex `ExtraParams`.
- [ ] T020 Avoid changing `ChatCompletionRequest.knownFields` unless RED tests prove no safer Codex-local fix exists.
- [ ] T021 Avoid changing `internal/provider/openai` production behavior unless no-regression tests require an explicit guard.

## Phase 3: Verification

- [ ] T022 Run `gofmt` on changed Go files.
- [ ] T023 Run `go test ./internal/provider/chatgptcodex -run 'TestBuildPayload_.*Store' -count=1`.
- [ ] T024 Run `go test ./internal/proxy/handler -run 'TestChatGPTCodexBackend.*Store|TestChatGPTCodexBackendWildcard' -count=1`.
- [ ] T025 Run `go test ./internal/provider/openai -run 'TestTransformRequest_ExtraParams|TestOpenAI.*Store' -count=1`.
- [ ] T026 Run `go test ./internal/provider/chatgptcodex ./internal/proxy/handler ./internal/provider/openai -count=1`.
- [ ] T027 Run `git diff --check origin/main...HEAD`.
- [ ] T028 Verify no test, doc, log, or PR text prints access token, refresh token, API key, bearer header, encrypted credential value, or raw credential JSON.

## Phase 4: Review and state progression

- [ ] T029 Move Linear to `In Review` after implementation tasks are complete and local gates pass.
- [ ] T030 Run review gate and fix all fatal/critical findings.
- [ ] T031 Move to `Waiting CI` after review pass and push.
- [ ] T032 Move to `Waiting Merge` only after CI is green and worktree is clean.

## Scope Stop

After Todo planning, stop at `Waiting` with a docs-only draft PR. Implementation remains blocked until Linear moves to `In Progress`.
