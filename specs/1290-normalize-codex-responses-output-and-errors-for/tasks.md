# Tasks: Normalize Codex Responses output and errors

**Input**: `spec.md`, `plan.md`, Linear HO-1290, current `origin/main` repo reality.
**Prerequisite**: Linear HO-1290 must move to `In Progress` before editing production files.

## Phase 0 - Governance

- [x] T001 Confirm Linear HO-1290 is `In Progress` before production edits.
- [x] T002 Confirm work continues in `worktrees/tianjiLLM/HO-1290-normalize-codex-responses-output-and-errors`.
- [x] T003 Re-read Linear description, this specs directory, HO-1288 PR #162 context, and current `internal/proxy/handler/chat.go`, `internal/model/errors.go`, `internal/model/response.go`, and `internal/provider/openai/openai.go`.
- [x] T004 Confirm branch diff before implementation contains only HO-1290 SpecKit docs.

## Phase 1 - RED Tests First

- [x] T005 Add failing Codex adapter test for completed Responses JSON mapping to `model.ModelResponse`.
- [x] T006 Add failing test for aggregation of multiple `output_text` parts into one assistant message.
- [x] T007 Add failing test for usage mapping: `input_tokens`, `output_tokens`, `total_tokens`.
- [x] T008 Add failing test for missing assistant text returning a safe transform error.
- [x] T009 Add failing test proving backend HTTP 401 returns HTTP 401 to caller.
- [x] T010 Add failing test proving backend HTTP 403 missing-scope diagnostics return HTTP 403 to caller.
- [x] T011 Add failing test proving backend HTTP 429 quota diagnostics return HTTP 429 to caller.
- [x] T012 Add failing test proving upstream `error.code` is preserved.
- [x] T013 Add failing leakage test with sentinel access token, refresh token, bearer header, and raw credential JSON.
- [x] T014 Add OpenAI Platform provider regression test or extend existing test proving API-key error behavior remains unchanged.
- [x] T015 Run targeted RED commands and record expected failures in implementation evidence.

## Phase 2 - Adapter Implementation

- [x] T016 Add Codex backend response structs in the Codex transport/adapter package.
- [x] T017 Implement successful Responses-to-`model.ModelResponse` mapping.
- [x] T018 Normalize `object` to `chat.completion`.
- [x] T019 Normalize `created_at` to `created`.
- [x] T020 Normalize assistant text output to `choices[0].message.content`.
- [x] T021 Normalize finish reason to `stop` unless backend provides a better compatible value.
- [x] T022 Normalize usage fields into Tianji `Usage`.
- [x] T023 Return safe transform errors for malformed success bodies and no-text success bodies.

## Phase 3 - Error Propagation

- [x] T024 Extend `model.TianjiError` or add an equivalent typed upstream error to carry upstream `code`.
- [x] T025 Update Codex adapter error parsing to preserve status, message, type, code, provider, and model.
- [x] T026 Add handler helper that writes actionable `TianjiError` statuses instead of generic 502.
- [x] T027 Preserve existing OpenAI subscription reauthorization special case.
- [x] T028 Keep generic parse/shape failures mapped to safe 502 `internal_error`.
- [x] T029 Apply redaction before writing private backend error messages to response/logs.

## Phase 4 - Integration Boundary

- [x] T030 Integrate the adapter into the HO-1288 Codex backend transport path.
- [x] T031 Ensure API-key-backed `internal/provider/openai` still uses normal Platform chat completion parsing.
- [x] T032 Ensure the new status-preserving handler behavior does not make non-upstream local validation errors look like upstream 4xx.
- [x] T033 Preserve or document rate-limit/debug headers if current response writer abstractions expose them safely.

## Phase 5 - Regression and Verification

- [x] T034 Run Codex adapter tests.
- [x] T035 Run handler error propagation tests.
- [x] T036 Run existing OpenAI provider tests.
- [x] T037 Run affected provider/handler/model package tests.
- [x] T038 Run repo lint command if production files changed.
- [x] T039 Run `git diff --check origin/main...HEAD`.
- [x] T040 Verify PR diff contains only HO-1290 implementation files and this specs directory.

## Phase 6 - State Progression

- [x] T041 Update SpecKit implementation evidence.
- [ ] T042 Move Linear to `In Review` and run review gate after implementation tasks complete.

## Implementation Evidence

- RED: `go test ./internal/provider/chatgptcodex ./internal/provider/openai ./internal/proxy/handler -run 'TestTransformResponse_|TestWriteTransformError' -count=1` failed before implementation because `chatgptcodex.New`, `model.TianjiError.Code`, and `writeTransformError` did not exist.
- GREEN targeted: `go test ./internal/provider/chatgptcodex ./internal/provider/openai ./internal/proxy/handler ./internal/security/redact -run 'TestTransformResponse_|TestWriteTransformError|TestRedact' -count=1`.
- GREEN affected packages: `go test ./internal/model/... ./internal/provider/chatgptcodex ./internal/provider/openai/... ./internal/proxy/handler/... ./internal/security/redact -count=1`.
- Lint: `go tool golangci-lint run` returned `0 issues`.
- Review fix: rebased onto latest `origin/main` after HO-1288 merged, wired the normalization through the real `chatgptcodex.Transport.TransformResponse` and `handleChatGPTCodexBackendCompletion` path, and added handler-level 401/403/429 coverage.
- Rate-limit/debug headers: current non-streaming transform path has no safe typed response-header propagation boundary; HO-1290 preserves 429 status and safe JSON error fields, but does not add header forwarding.

## Scope Stop

After Todo planning, stop at `Waiting` with a docs-only draft PR. Waiting Merge is blocked until Linear moves through `In Progress`, `In Review`, `Waiting CI`, and CI gates.
