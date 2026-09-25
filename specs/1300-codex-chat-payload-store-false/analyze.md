# Analyze: HO-1300 Codex backend chat payload `store:false`

## Result

- Fatal: 0
- Critical: 0
- Owner input required: 0
- Governance blocker: production/test implementation cannot start until Linear moves to `In Progress`.

## Linear Scope Coverage

| Linear explicit behavior | Spec coverage | Plan/tasks coverage |
| --- | --- | --- |
| Codex transport always sends `store:false` | US1, FR-001..FR-003, SC-001 | T005, T010, T015-T016 |
| Client `store:false` does not trip `ExtraParams` rejection | US2, FR-004, SC-002 | T006, T009, T017 |
| Client `store:true` is rejected locally | US3, FR-005..FR-006, SC-003 | T007, T011, T018 |
| Normal OpenAI API-key provider unchanged | US4, FR-008, SC-004 | T013, T020-T021, T025 |
| No secrets logged or printed | FR-009, quickstart secret checklist | T028 |

## Issue Explicit Behavior Map

| Issue noun / verb | Artifact mapping | Alignment result |
| --- | --- | --- |
| `Store must be set to false` | `spec.md` Summary/US1, `plan.md` Repo Reality | Covered |
| `Payload builder does not include store` | `spec.md` FR-001..FR-003, `plan.md` Payload builder | Covered |
| `store:false` captured in `ExtraParams` | `spec.md` US2, `data-model.md` Client `store` Extra Parameter | Covered |
| `store:true` rejected locally | `spec.md` US3, `contracts/codex-store-false.md` | Covered |
| `Do not change normal OpenAI API-key provider behavior` | `spec.md` US4/FR-008, `plan.md` OpenAI no-regression | Covered |
| `No secrets or raw subscription tokens` | `quickstart.md` Secret-Safety Checklist, `tasks.md` T028 | Covered |

## Cross-Artifact Consistency

- `spec.md` defines the bug fix as Codex-specific.
- `plan.md` avoids global request parsing changes to preserve OpenAI provider pass-through.
- `tasks.md` starts with RED tests and keeps implementation blocked until `In Progress`.
- `research.md` records memory, repo, official docs, Context7, and grep-app evidence.
- `data-model.md` defines only the payload field and Codex-local `store` validation rules.
- `contracts/` defines accepted/rejected client `store` cases.

## Out-of-Scope Challenge

- Credential lifecycle stale model references are real but not owned by HO-1300; this issue starts after credential resolution succeeds.
- UI selector behavior is owned by HO-1291 and already merged.
- Wildcard routing regression is owned by HO-1292 and already covered; HO-1300 adds store-specific wildcard coverage only because production uses `openai/*`.
- Native `/v1/responses` subscription behavior is not owned here; HO-1300 targets `/v1/chat/completions` routed through ChatGPT Codex backend.

## E2E Coverage Map

- PR layer: TianjiLLM Codex backend payload and chat handler validation.
- Runtime boundary: `/v1/chat/completions` -> route selection -> subscription credential resolver -> `chatgptcodex.BuildPayload` -> mocked Codex backend.
- Test command: `go test ./internal/provider/chatgptcodex ./internal/proxy/handler ./internal/provider/openai -count=1`.
- CI evidence: full PR workflow after implementation.
- Gap/follow-up: live production confirmation remains post-merge/deploy evidence and must avoid printing secrets.

## Final Scope Confirmation

HO-1300 Todo planning is internally consistent and needs no owner input. Move to `Waiting` after docs-only diff passes, draft PR exists, and the thread is notified with Linear and PR links.
