# Analyze: HO-1186 Credentials sidebar list/detail quota UI

## Result

- Fatal: 0
- Critical: 0
- Owner input required: 0

## Linear Scope Coverage

| Linear explicit behavior | Spec coverage | Plan/tasks coverage |
| --- | --- | --- |
| Add sidebar `Credentials` page | FR-001, US1 | T001, T010, T011 |
| Credential list page | FR-002 to FR-004, US1 | T002, T003, T014, T021 |
| Credential detail page | FR-005 to FR-006, US2 | T004, T005, T015, T022 |
| Account email | FR-004, FR-006 | T005, T016, T022 |
| Status | FR-004, FR-006, FR-009 | T005, T007, T018 |
| Utilization/quota usage | FR-007, FR-008, FR-013 | T006, T017, T023 |
| Reset time | FR-006, FR-007 | T006, T022 |
| `last_refresh_at` | FR-004, FR-006 | T005, T016, T022 |
| `last_error` | FR-004, FR-006, FR-012 | T005, T024, T030 |
| `disabled_reason` | FR-006, FR-009 | T005, T018, T032 |
| Cards/table/progress/status badge/reset UI | FR-010 plus UI stories | T020 to T024 |
| UI E2E list/detail | FR-014 | T001 to T005 |
| Quota/status rendering | FR-014 | T006, T007, T031, T032 |
| Safe metadata only; no token display | FR-011, FR-012, FR-015 | T008, T019, T030 |

## Cross-Artifact Consistency

- `spec.md` and `plan.md` both define read-only OpenAI subscription credential UI.
- `tasks.md` starts with failing E2E tests before route/page implementation.
- `data-model.md` and `contracts/credentials-ui.md` both forbid token material in view models and HTML.
- `research.md` records repo reality, Context7, grep.app, and official OpenAI evidence.
- `quickstart.md` gates implementation on Linear `In Progress`.

## Out-of-Scope Challenge

- Create/update/delete/test/refresh/disable UI is excluded because Linear only requests viewing list/detail quota UI and sibling API issues own lifecycle endpoints.
- Generic `api_key` credential UI is excluded because Linear goal says OpenAI subscription credentials and quota/status.
- Live OpenAI calls are excluded because HO-1181/HO-1185 own parser/lifecycle upstream behavior; HO-1186 should read stored state only.
- Billing quota/monthly allowance estimates are excluded because HO-1181 explicitly forbids local estimation.

## E2E Coverage Map

- PR layer: TianjiLLM admin UI (`internal/ui`)。
- Runtime boundary: browser -> `/ui/credentials` server-rendered templ pages -> DB safe fields + in-memory `RateLimitStore`。
- Test command: `go test ./test/e2e -tags e2e -run 'TestCredentials' -count=1`。
- Required assertions: sidebar nav, list row filtering, detail metadata, quota dimensions/status/reset, no token strings in DOM。
- Gap/follow-up: No UI action buttons in this issue; lifecycle actions remain API-only until a separate UI issue.

## Final Scope Confirmation

HO-1186 Todo planning is internally consistent and needs no owner question. Move to Waiting after draft PR exists and docs-only diff passes.
