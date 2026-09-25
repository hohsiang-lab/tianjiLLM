# Analyze: HO-1290 Codex response/error normalization

## Result

- Fatal: 0
- Critical: 0
- Owner input required: 0

## Issue Explicit Behavior Map

| Linear requirement | Spec | Plan | Tasks | Status |
|---|---:|---:|---:|---|
| Return compatible non-streaming chat completion response | FR-001-FR-007, SC-001 | D1-D2 | T005-T008, T016-T023 | Covered |
| Preserve useful upstream auth diagnostics | FR-008, FR-011, FR-012 | D3-D5 | T009, T012, T024-T029 | Covered |
| Preserve useful missing-scope diagnostics | FR-009, FR-011, FR-012 | D3-D5 | T010, T012, T024-T029 | Covered |
| Preserve useful quota diagnostics | FR-010-FR-012 | D3-D5 | T011-T012, T024-T029 | Covered |
| Avoid misleading generic proxy errors | FR-012 | D3 | T026-T028 | Covered |
| Existing Platform OpenAI behavior unchanged | FR-014, SC-004 | D4 | T014, T031, T036 | Covered |
| Acceptance tests for success/401/403/429 | SC-001-SC-003 | Verification Commands | T005-T015 | Covered |

## Consistency Checks

- `spec.md` narrows HO-1290 to non-streaming success + actionable error surface.
- `plan.md` identifies the concrete current handler bug chain from `TransformResponse` error to generic 502.
- `research.md` separates official public OpenAI Responses/Chat shapes from private ChatGPT Codex backend wire behavior.
- `data-model.md` maps backend usage/output to existing Tianji models without DB migration.
- `tasks.md` requires RED tests before production edits.
- No UI mockscreen is required because HO-1290 is backend/API behavior.

## Scope Confirmation

Default scope is sufficient and owner input required is 0:

1. Normalize successful Codex Responses output into `chat.completion`.
2. Preserve upstream 401/403/429 status and safe diagnostics.
3. Preserve upstream `error.code` for missing-scope/quota/auth debugging.
4. Keep generic malformed response failures as safe transform errors.
5. Preserve normal Platform OpenAI provider behavior.
6. Use mocked tests only; no real OpenAI/ChatGPT calls.

## Open Questions

None blocking Todo gate. Implementation may discover whether rate-limit/debug headers can be safely forwarded; if not, record the limitation in implementation evidence without expanding scope.
