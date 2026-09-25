# Analyze: HO-1321 Codex Responses WebSocket adapter

## Coverage Matrix

| Requirement | Spec | Plan | Tasks | Status |
| --- | --- | --- | --- | --- |
| Prewarm `generate:false` is consumed locally | US1, FR-003, SC-001 | Prewarm Adapter | T002-T004, T013-T017 | Covered |
| Follow-up state chaining from warmup works | US2, FR-004..FR-007, SC-002 | State Chaining | T005-T006, T018-T021 | Covered |
| Large Codex frames are accepted | US3, FR-002, SC-003 | WebSocket Capacity | T007, T010-T012 | Covered |
| True Codex CLI has no reconnect noise | US4, FR-012, SC-004 | Verification | T032-T033 | Covered |
| Existing HO-1319/HO-1314 behavior remains green | US5, FR-009..FR-010, SC-005 | Route/Regression Guardrails | T023-T026, T028-T031 | Covered |

## Repo Reality Alignment

- `internal/proxy/handler/responses.go` currently has no `SetReadLimit`.
- `internal/proxy/handler/responses.go` currently handles each frame independently and keeps no connection-local previous-response state.
- `internal/provider/chatgptcodex/transport.go` currently copies raw payload fields and does not consume `generate`.
- Existing tests cover small sequential frames, but not Codex prewarm, empty-input follow-up, or >32KiB frames.

## Consistency Checks

- Todo phase remains docs-only.
- Tasks are unchecked because implementation has not started.
- Spec rejects HTTP fallback success as acceptance.
- Plan follows official WebSocket state semantics without adding DB/global persistence.
- Live verification requires true `codex exec`.

## Fatal Issues

0

## Critical Issues

0

## Owner Input Required

0 for Todo scope.
