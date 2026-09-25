# Tasks: Chat requests to Codex Responses payload

**Input**: `spec.md`, `plan.md`, Linear HO-1289, current `origin/main` repo reality。
**Prerequisite**: Linear HO-1289 must move to `In Progress` before editing production files。

## Phase 0 - Governance

- [x] T001 Confirm Linear HO-1289 is `In Progress` before production edits。
- [x] T002 Confirm work continues in `worktrees/tianjiLLM/HO-1289-chat-requests-to-codex-responses`。
- [x] T003 Re-read Linear description, this specs directory, merged HO-1288 files `internal/provider/chatgptcodex/transport.go`, `internal/proxy/handler/chatgpt_codex_backend.go`, `internal/model/request.go`, `internal/provider/openai/openai.go`, and `internal/proxy/handler/responses*.go`。
- [x] T004 Fetch `origin/main`, ensure worktree contains HO-1288 merge `5236bfb` before production edits, and confirm branch diff before implementation contains only HO-1289 SpecKit docs。

## Phase 1 - RED Tests First

- [x] T005 Add failing unit test for user text chat request -> Codex Responses `input` message item。
- [x] T006 Add failing unit test proving backend payload contains no legacy `messages` key。
- [x] T007 Add failing unit test for leading `system` message -> top-level `instructions`。
- [x] T008 Add failing unit test for leading `developer` message -> top-level `instructions`。
- [x] T009 Add failing unit test for mixed `system` + `developer` ordering。
- [x] T010 Add failing unit test for non-leading `system` / `developer` behavior。
- [x] T011 Add failing unit test for assistant text message mapping。
- [x] T012 Add failing unit test for supported content part array mapping。
- [x] T013 Add failing unit test for supported image content mapping or explicit unsupported diagnostic。
- [x] T014 Add failing unit test for model normalization。
- [x] T015 Add failing unit test for `max_tokens` / `max_completion_tokens` mapping。
- [x] T016 Add failing unit test for `temperature`, `top_p`, `metadata`, and `user` preservation plus `stream=true` unsupported/no-upstream behavior。
- [x] T017 Add failing unit tests for unsupported `tools`, `tool_choice`, `tool_calls`, `tool` role, `n > 1`, `logprobs`, `top_logprobs`, unsupported `response_format`, `stop`, and unknown `ExtraParams`。
- [x] T018 Add failing no-secret-leakage test with sentinel credential-like values in diagnostics。
- [x] T019 Add OpenAI provider regression test proving normal `/v1/chat/completions` body remains unchanged。
- [x] T020 Run targeted RED command and record expected failures。

## Phase 2 - Mapper Types and Diagnostics

- [x] T021 Add Codex Responses payload structs or deterministic map builder。
- [x] T022 Add structured unsupported-field diagnostic type。
- [x] T023 Add allowlist for compatible fields。
- [x] T024 Add model normalization helper and table tests。
- [x] T025 Add content conversion helpers for string and typed content parts。
- [x] T026 Add instruction aggregation helper。

## Phase 3 - Payload Mapping

- [x] T027 Map user messages into `input` message items with `input_text`。
- [x] T028 Map assistant messages into `input` message items with output-compatible text if accepted by backend fixtures。
- [x] T029 Map leading `system` / `developer` messages into `instructions`。
- [x] T030 Implement chosen behavior for non-leading `system` / `developer`。
- [x] T031 Map supported multimodal content or return explicit diagnostics。
- [x] T032 Map compatible options。
- [x] T033 Reject unsupported fields before building upstream HTTP request。
- [x] T034 Ensure mapper output never contains credential material。

## Phase 4 - Transport Integration Boundary

- [x] T035 Replace HO-1288 `chatgptcodex.Transport.BuildRequest` direct `input: req.Messages` payload with mapper output。
- [x] T036 Ensure route selection invokes mapper only for explicit Codex backend transport and preserves existing `unsupported_streaming` no-upstream response。
- [x] T037 Ensure normal OpenAI API-key routes still call existing OpenAI provider transform。
- [x] T038 Ensure mapper errors become safe OpenAI-compatible error responses。

## Phase 5 - Verification

- [x] T039 Run targeted mapper tests。
- [x] T040 Run OpenAI provider regression tests。
- [x] T041 Run affected handler/provider tests。
- [x] T042 Run repo lint command or `go tool golangci-lint run` for touched packages。
- [x] T043 Run `git diff --check origin/main...HEAD`。
- [x] T044 Verify PR diff contains only HO-1289 implementation files and this specs directory。

## Phase 6 - State Progression

- [x] T045 Update `tasks.md` and `analyze.md` with implementation evidence。
- [ ] T046 Move Linear to `In Review` after implementation and verification complete。

## Scope Stop

Todo planning 完成後 stop at `Waiting` with docs-only draft PR。Implementation and Waiting Merge are blocked until Linear moves through `In Progress`, review, CI, and merge gates。
