# Analyze: HO-1288 ChatGPT Codex backend transport

## Result

- Fatal: 0
- Critical: 0
- Owner input required: 0

## Coverage Matrix

| Requirement | Spec | Plan | Tasks | Status |
|---|---:|---:|---:|---|
| Dedicated ChatGPT Codex backend transport | FR-001, SC-001 | D1, Target Flow | T005, T019, T030 | Covered |
| Backend URL `/backend-api/codex/responses` | FR-002, SC-001 | D2, Target Flow | T005, T022 | Covered |
| Access token resolution and bearer auth | FR-004, FR-005 | D4 | T007, T025-T027 | Covered |
| Account ID header | FR-006, SC-003 | Target Flow | T008, T033 | Covered |
| `originator: codex_cli_rs` default and override | FR-007, SC-003 | D3 | T009-T010, T023 | Covered |
| No Platform `/v1/chat/completions` for backend transport | FR-014, SC-002 | Risk Register | T006 | Covered |
| API-key OpenAI provider unchanged | FR-012, FR-013, SC-004 | D1 | T017, T042, T045 | Covered |
| No API-key fallback for explicit subscription backend | FR-015, SC-005 | D4 | T015, T026 | Covered |
| Token/error redaction | FR-016, SC-005 | Risk Register | T016, T037 | Covered |
| Mocked/offline tests | FR-017, SC-006 | D5 | T011, T018, T043 | Covered |

## Consistency Checks

- `spec.md` separates ChatGPT Codex backend HTTP transport from Codex app-server login.
- `plan.md` keeps `internal/provider/openai` scoped to normal Platform `/v1` behavior.
- `data-model.md` avoids changing existing OAuth authorize `Originator` default.
- `tasks.md` starts with RED tests before production implementation.
- `contracts/` includes URL, headers, body, error, and non-goal boundaries.
- No UI mockscreen is required because HO-1288 is backend transport work.

## Scope Confirmation

Default scope is sufficient and owner input required is 0:

1. Add explicit ChatGPT Codex backend transport for subscription-backed models.
2. Default backend URL to `https://chatgpt.com/backend-api/codex/responses`.
3. Use stored subscription access token and account ID.
4. Send `Authorization`, `ChatGPT-Account-Id` when available, and default `originator: codex_cli_rs`.
5. Preserve API-key OpenAI provider and custom OpenAI-compatible behavior.
6. Prove no Platform `/v1/chat/completions` call when backend transport is selected.
7. Keep tests mocked/offline and no production code in Todo phase.
