# Analyze: HO-2368 Codex session sticky reuse across primary reset

**Date**: 2026-07-09
**Artifacts**: `spec.md`, `plan.md`, `tasks.md`, `research.md`, `data-model.md`, `contracts/codex-session-sticky-policy.md`

## Summary

- Fatal issues: 0
- Critical issues: 0
- High issues: 0
- Medium issues: 0
- Low issues: 1

## Findings

| ID | Category | Severity | Location(s) | Summary | Recommendation |
|----|----------|----------|-------------|---------|----------------|
| L1 | Tooling | LOW | `research.md` Decision 5 | Official SpecKit helper scripts reject Sima's required `HO-2368-...` branch naming because they require `NNN-feature`. | Keep manual template-filled artifacts for this issue; do not patch unrelated tooling in HO-2368. |

## Coverage Summary

| Requirement Key | Has Task? | Task IDs | Notes |
|-----------------|-----------|----------|-------|
| FR-001 session reuse despite primary reset | Yes | T006, T007, T008, T009 | Core bug fix is covered by failing test and implementation tasks. |
| FR-002 fallback behavior preserved | Yes | T004, T010, T020 | Existing fallback test is explicitly kept. |
| FR-003 selectable gate still releases credential | Yes | T011, T012, T014, T015 | Primary and secondary gate coverage required. |
| FR-004 unknown metadata conservative behavior | Yes | T013, T020 | Existing missing-current-snapshot behavior remains in focused validation. |
| FR-005 disabled/removed/filtering preserved | Yes | T003, T015, T023 | Scope check prevents filtering changes; tests inspect current pipeline. |
| FR-006 raw session id redaction | Yes | T016, T018, T019 | Log redaction remains covered. |
| FR-007 failing-tests-first coverage | Yes | T006, T007, T011, T012, T013, T016, T017 | Tasks require failing baseline before implementation. |
| FR-008 non-goals unchanged | Yes | T010, T015, T023, T024 | Diff-scope and PR evidence tasks cover non-goals. |
| SC-001 primary reset session reuse test | Yes | T006, T020 | Direct test coverage. |
| SC-002 gate reselect tests | Yes | T011, T012, T020 | Direct test coverage. |
| SC-003 fallback behavior unchanged | Yes | T004, T010, T020 | Existing test retained. |
| SC-004 log redaction | Yes | T016, T019 | Direct log coverage. |
| SC-005 focused handler command | Yes | T020 | Verification command named. |

## Constitution Alignment Issues

None.

## Unmapped Tasks

None. Setup and scope tasks support artifact/readiness gates and do not map one-to-one to product requirements.

## Metrics

- Total buildable requirements: 13
- Requirements with task coverage: 13
- Coverage: 100%
- Total tasks: 24
- Ambiguity count: 0
- Duplication count: 0
- Fatal issues count: 0
- Critical issues count: 0

## UI / Source Evidence

UI is N/A. Linear HO-2368 explicitly says "No UI change"; repo confirmation found the owning surface in backend routing code and focused Go tests only. No route, visual surface, Figma, mockscreen, or Playwright snapshot baseline is required.

Playwright snapshot applicability: not-suitable. The issue is backend routing policy in `internal/proxy/handler/openai_subscription_routing.go`; the repo has Playwright E2E tests, but no stable changed UI route/state exists for this behavior. Expected evidence is Go unit tests and routing/log verification.

## Missing Parts

Missing part: none. Linear issue provides desired behavior and non-goals; project registry resolves to `hohsiang-lab/tianjiLLM`; repo confirmation identifies implementation and test surfaces; no external contract, UI source, DB migration, config decision, or assignee clarification remains for In Progress.

## Next Actions

Proceed to docs-only draft PR and Todo -> Waiting guard. Implementation must wait for `Waiting -> In Progress` authorization.
