# Analyze: HO-2370 Mirror Codex subscription model catalog

## Coverage Matrix

| Requirement | Spec | Plan | Tasks | Status |
| --- | --- | --- | --- | --- |
| Fetch upstream Codex catalog | FR-001 | Summary, Phase 1 | T004, T007, T012, T022 | Covered |
| Cache without per-request refresh | FR-002 | Phase 1, US2 tests | T016, T021 | Covered |
| Credential/entitlement-aware cache | FR-003 | Data model, US2 tests | T017, T023 | Covered |
| Runtime aliases remain boundary | FR-004 | US1 AS-3 | T009, T013 | Covered |
| Map upstream metadata fields | FR-005 | US1 tests | T008, T012-T015 | Covered |
| Preserve explicit auto-compact null | FR-006 | Edge cases | T010, T014 | Covered |
| Fresh/LKG/static fallback ladder | FR-007 | US3 tests | T025, T026, T029, T031 | Covered |
| Degraded signal without secrets | FR-008 | US3 AS-3 | T027, T030 | Covered |
| Credential invalidation | FR-009 | US2 AS-3 | T018, T024 | Covered |
| Preserve HO-1331 response shape | FR-010 | US4 | T032-T036 | Covered |
| Regression coverage breadth | FR-011 | Failing Tests | T001-T041 | Covered |
| Live verification | FR-012 | Quickstart | T041 | Covered |

## Consistency Findings

| ID | Category | Severity | Location(s) | Summary | Recommendation |
|----|----------|----------|-------------|---------|----------------|
| A1 | Ambiguity | MEDIUM | `spec.md` FR-003, `plan.md` US2 tests | Multi-credential alias semantics require an implementation choice: select one credential, intersect capabilities, or explicit aggregate. | Keep as implementation design decision, but require `TestOpenAISubscriptionCodexCatalogSeparatesCredentialEntitlements` before code. |
| U1 | Underspecification | LOW | `plan.md` Phase 1 | Persistent last-known-good storage path may use existing credential info or a new table. | Task T006 forces this decision before implementation; sqlc-first rule is explicit. |

## Constitution Alignment

- Principle IV is satisfied: each user story has concrete failing tests listed before implementation.
- Principle VII is satisfied: any database persistence requires SQL/sqlc, not raw SQL.
- Principle III/VI are satisfied for Todo scope: repo reality, official OpenAI docs, stock Codex source, and web evidence are captured in `research.md`.

## Unmapped Tasks

None. All tasks map to setup, a user story, or final verification.

## Metrics

- Total Functional Requirements: 12
- Total Tasks: 41
- Requirements with task coverage: 12/12
- Coverage: 100%
- Ambiguity Count: 1
- Duplication Count: 0
- Fatal Issues: 0
- Critical Issues: 0
- Critical Issues Count: 0

## Structured Todo Evidence

- Approach recorded: yes, upstream Codex catalog mirror/cache enriching Tianji runtime aliases.
- Surfaces recorded: yes, `internal/provider/chatgptcodex`, `internal/proxy/handler`, optional sqlc-backed `internal/db`, and package tests.
- Task groups recorded: yes, RED tests, parser, cache, enrichment, fallback, regression, live verification.
- Behavior recorded: yes, fresh/LKG/static fallback and runtime route boundary are specified.
- UI alignment recorded: UI N/A; backend/API/catalog metadata issue, no UI surface.
- Clarification recorded: no clarification markers remain.
- No clarification markers remain.
- Missing part: none. Evidence: Linear long-term-fix scope, repo code scan, HO-1331 artifacts, stock Codex source, OpenAI docs, and explicit tests/tasks cover the Todo planning questions.
- Unclear items recorded: none for Todo. Multi-credential semantics are a planned implementation test gate, not owner input.
- unclear: none for Todo owner input.
- Pre-implementation missing parts: none for Todo planning; In Progress must still write RED tests before code.
- Repo/code confirmation recorded: `rg` and direct file reads confirmed current code and test surfaces.
- Implementation-detail clarity recorded: plan and tasks name exact packages/files, behavior, tests, and verification commands.

## Next Actions

- Proceed to owner review of docs-only draft PR.
- Do not run `/speckit-implement` until the issue is moved from Waiting to In Progress by the normal pipeline gate.
