# Analyze: HO-1289 chat to Codex Responses payload

## Result

- Fatal: 0
- Critical: 0
- Owner input required: 0

## Coverage Matrix

| Requirement | Spec | Plan | Tasks | Status |
|---|---:|---:|---:|---|
| Pure mapper boundary | FR-001 | D1 | T021-T023 | Covered |
| Model normalization | FR-002 | D5 | T014, T024 | Covered |
| Messages to `instructions` / `input` | FR-003 | D2-D4 | T005-T013, T027-T031 | Covered |
| Developer/system/user tests | FR-004 | D3 | T007-T010 | Covered |
| Text content mapping | FR-005 | D4 | T005, T012, T025 | Covered |
| Compatible option preservation | FR-007 | D5 | T015-T016, T032 | Covered |
| Streaming deterministic behavior | FR-008 | D5 | T016 | Covered |
| Unsupported-field diagnostics | FR-009, FR-010 | D5 | T017, T022-T023, T033 | Covered |
| No credential leakage | FR-011 | Risk Register | T018, T034 | Covered |
| OpenAI provider no regression | FR-012 | D1 | T019, T037, T040 | Covered |
| Offline tests | FR-013 | Verification | T039-T043 | Covered |

## Consistency Checks

- `spec.md` keeps HO-1289 scoped to payload mapping only。
- `plan.md` does not mutate `internal/provider/openai` as the primary design。
- `tasks.md` starts with RED tests before production edits。
- `research.md` records repo reality, official docs boundary, and `openai/codex` wire-shape evidence。
- Waiting revision rechecked HO-1288 merged code: `Transport.BuildRequest` currently forwards `req.Messages` into `input`, and `chatgpt_codex_backend` streaming currently returns `unsupported_streaming` before upstream。
- `contracts/` defines valid output and unsupported field behavior。
- No UI mockscreen is required because this is backend payload transformation work。

## Scope Confirmation

Default scope is sufficient and owner input required is 0:

1. Add a pure chat-to-Codex Responses payload mapper。
2. Normalize backend model strings。
3. Convert leading `system` / `developer` messages to `instructions`。
4. Convert user/assistant conversation messages to typed Responses `input`。
5. Preserve compatible options only through an allowlist。
6. Reject or diagnostically ignore unsupported fields。
7. Keep normal OpenAI chat provider and merged HO-1288 transport responsibilities separate。
8. Replace only the current HO-1288 direct message-forwarding request body, not transport headers/credentials/retry/response adaptation。

## Implementation Evidence

- RED observed: targeted `TestChatGPTCodexBackendTransport_*` tests failed because HO-1288 forwarded raw `messages` into `input`, omitted compatible options, and sent unsupported `tools` to upstream。
- Mapper added at `internal/provider/chatgptcodex/payload.go` and integrated through `Transport.BuildRequest`。
- Leading `system` / `developer` messages now map to top-level `instructions`; conversation messages map to typed `input` message items。
- Compatible options preserved: `temperature`, `top_p`, `max_tokens` / `max_completion_tokens` as `max_output_tokens`, `metadata`, and safe `user` metadata。
- Unsupported fields fail before upstream with `invalid_request_error`; diagnostics do not include unsupported values or credential-like content。
- Normal OpenAI provider regression is covered by `internal/provider/openai` tests asserting `/v1/chat/completions` still emits `messages`, not Codex `input` / `instructions`。
- Verification passed: `go test ./internal/provider/chatgptcodex ./internal/proxy/handler ./internal/provider/openai ./internal/model -count=1`。
- Verification passed: `go tool golangci-lint run ./internal/provider/chatgptcodex ./internal/proxy/handler ./internal/provider/openai ./internal/model` returned 0 issues。
- Verification passed: `git diff --check origin/main...HEAD`。
