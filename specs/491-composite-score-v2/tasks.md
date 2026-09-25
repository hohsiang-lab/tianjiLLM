# Tasks: Composite Score V2

**Input**: Design documents from `/specs/491-composite-score-v2/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md

**Tests**: Failing tests are MANDATORY (Constitution Principle IV). Task 1 of every user story MUST be "Write failing tests" based on the plan's `## Failing Tests` section. No implementation task may begin until failing tests are written and confirmed to fail.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: No project initialization needed — all files exist. This phase is empty.

**Checkpoint**: Ready to proceed directly to foundational changes.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Change the `CompositeScore` function signature and formula. All user stories depend on this.

- [ ] T001 Change `CompositeScore` signature to accept 4 params (`util5h`, `util7d`, `util7dSonnet`, `threshold5h float64`) and implement `max(u5h/threshold5h, u7d/0.9, u7ds/0.9)` with sentinel handling (treat <0 as 0) in `internal/callback/composite.go`
- [ ] T002 Update `sentinelMaxComposite` from 4.0 to 2.0 and update godoc comments in `internal/proxy/handler/native_upstream.go`

**Checkpoint**: Foundation ready — `CompositeScore` has new signature, code won't compile until callers are updated. User story implementation can now begin.

---

## Phase 3: User Story 1 — Sonnet-aware selection (Priority: P1)

**Goal**: Token selector considers 7d Sonnet utilization. Tokens with high Sonnet usage are deprioritized.

**Independent Test**: Configure two tokens where one has high u7ds but low u7d. Verify the selector picks the other token.

### Tests for User Story 1 (MANDATORY - Principle IV)

- [ ] T003 [P] [US1] Write `TestCompositeScore_MaxNormalized` — verify `CompositeScore(0.06, 0.69, 0.48, 0.8) ≈ 0.767` in `internal/proxy/handler/native_upstream_test.go`
- [ ] T004 [P] [US1] Write `TestCompositeScore_MaxNormalized_AllZero` — verify `CompositeScore(0, 0, 0, 0.8) == 0` in `internal/proxy/handler/native_upstream_test.go`
- [ ] T005 [P] [US1] Write `TestCompositeScore_MaxNormalized_SonnetDominates` — verify `CompositeScore(0.1, 0.3, 0.85, 0.8) ≈ 0.944` in `internal/proxy/handler/native_upstream_test.go`
- [ ] T006 [P] [US1] Write `TestLowestUtilSelect_PreferLowSonnet` — token with u7ds=30% selected over u7ds=85% in `internal/proxy/handler/native_upstream_test.go`
- [ ] T007 [P] [US1] Write `TestLowestUtilSelect_7dAllStillDominates` — regression: when u7d > u7ds, behavior unchanged in `internal/proxy/handler/native_upstream_test.go`
- [ ] T008 [US1] Run `go test ./internal/proxy/handler/... -run "TestCompositeScore_MaxNormalized|TestLowestUtilSelect_Prefer|TestLowestUtilSelect_7dAll" -v` — confirm all tests FAIL

### Implementation for User Story 1

- [ ] T009 [US1] Update `lowestUtilizationSelect` to extract `Unified7dSonnetUtilization` from state, normalize sentinel to 0, and pass to `CompositeScore` along with `threshold` in `internal/proxy/handler/native_upstream.go`
- [ ] T010 [US1] Update existing `TestCompositeScore_Formula` assertions from quadratic to max-normalized expectations in `internal/proxy/handler/native_upstream_test.go`
- [ ] T011 [US1] Update existing `TestLowestUtilizationSelect_Equal7dLower5hWins` composite value comments and guard assertion in `internal/proxy/handler/native_upstream_test.go`
- [ ] T012 [US1] Update existing `TestSelectUpstreamThrottle_UsesLowestUtilization` to add `Unified7dSonnetUtilization` field in state setup in `internal/proxy/handler/native_upstream_test.go`
- [ ] T013 [US1] Update existing `TestLowestUtilizationSelect_Util7dSentinel` expected composite values in `internal/proxy/handler/native_upstream_test.go`
- [ ] T014 [US1] Run `go test ./internal/proxy/handler/... -run "TestCompositeScore|TestLowestUtil|TestSelectUpstream" -v` — confirm ALL tests PASS

**Checkpoint**: Token selection now considers 7d Sonnet. Core bug fixed.

---

## Phase 4: User Story 2 — 7d Sonnet gate (Priority: P1)

**Goal**: Hard gate filters out tokens with u7ds >= 0.90 before score evaluation.

**Independent Test**: Configure one token with u7ds >= 90%. Verify HTTP 429 returned.

### Tests for User Story 2 (MANDATORY - Principle IV)

- [ ] T015 [P] [US2] Write `TestSelectUpstream_7dSonnetGate_Skips` — token with u7ds=0.92 filtered, other selected in `internal/proxy/handler/native_upstream_test.go`
- [ ] T016 [P] [US2] Write `TestSelectUpstream_7dSonnetGate_AllThrottled` — both tokens u7ds >= 0.90 → `allTokensThrottledError` with 7d_s reset time in `internal/proxy/handler/native_upstream_test.go`
- [ ] T017 [P] [US2] Write `TestSelectUpstream_7dSonnetGate_SentinelSkipped` — token with u7ds=-1 is NOT filtered by gate in `internal/proxy/handler/native_upstream_test.go`
- [ ] T018 [P] [US2] Write `TestSelectUpstream_AllowedWarning_Bypasses7dSonnetGate` — token with u7ds=0.95 + status=allowed_warning passes gate in `internal/proxy/handler/native_upstream_test.go`
- [ ] T019 [US2] Run `go test ./internal/proxy/handler/... -run "TestSelectUpstream_7dSonnetGate|TestSelectUpstream_AllowedWarning_Bypasses7dSonnetGate" -v` — confirm all tests FAIL

### Implementation for User Story 2

- [ ] T020 [US2] Add 7d_s gate check in `selectUpstreamWithThrottle`: if `Unified7dSonnetUtilization >= 0 && >= gate7d` → set throttled=true, log with reason=7d_sonnet_gate in `internal/proxy/handler/native_upstream.go`
- [ ] T021 [US2] Add `Unified7dSonnetReset` to `trackNearestReset` call when token is throttled in `internal/proxy/handler/native_upstream.go`
- [ ] T022 [US2] Run `go test ./internal/proxy/handler/... -run "TestSelectUpstream_7dSonnetGate|TestSelectUpstream_AllowedWarning" -v` — confirm ALL tests PASS

**Checkpoint**: Hard safety net for Sonnet window. No token can be selected with u7ds >= 90%.

---

## Phase 5: User Story 3 — UI score display (Priority: P2)

**Goal**: UI displays max-normalized score with meaningful color thresholds.

**Independent Test**: View UI with known values, verify score and color match expectations.

### Tests for User Story 3 (MANDATORY - Principle IV)

- [ ] T023 [P] [US3] Update `TestCcCompositeScoreStyle_Boundaries` — change thresholds to green < 0.5, orange < 0.8, red >= 0.8 in `internal/ui/pages/usage_claude_code_test.go`
- [ ] T024 [P] [US3] Update `TestLoadClaudeCodeData_CompositeScore` — update expected score to max-normalized with 3 windows in `internal/ui/handler_claude_code_test.go`
- [ ] T025 [US3] Run `go test ./internal/ui/... -run "TestCcCompositeScoreStyle|TestLoadClaudeCodeData_CompositeScore" -v` — confirm tests FAIL

### Implementation for User Story 3

- [ ] T026 [US3] Update `ccCompositeScoreStyle` color boundaries: `< 0.5` green, `< 0.8` orange, `>= 0.8` red in `internal/ui/pages/usage_claude_code_templ.go`
- [ ] T027 [US3] Update `loadClaudeCodeData` to pass `sonnetUsage` (normalized, nil→0) and `threshold` to `CompositeScore` in `internal/ui/handler_claude_code.go`
- [ ] T028 [US3] Run `templ generate` to regenerate templ Go files after modifying `.templ` source (if color style is in `.templ` file)
- [ ] T029 [US3] Run `go test ./internal/ui/... -run "TestCcCompositeScoreStyle|TestLoadClaudeCodeData" -v` — confirm ALL tests PASS

**Checkpoint**: UI shows meaningful scores. Operators can interpret token health at a glance.

---

## Phase 6: User Story 4 — Missing data graceful degradation (Priority: P2)

**Goal**: Tokens with missing Sonnet data are handled gracefully — no crash, no incorrect penalization.

**Independent Test**: Create token with u7ds=-1, verify score computed correctly without Sonnet influence.

### Tests for User Story 4 (MANDATORY - Principle IV)

- [ ] T030 [P] [US4] Write `TestCompositeScore_SonnetSentinel` — verify `CompositeScore(0.3, 0.4, -1, 0.8) → 0.444` (u7ds treated as 0) in `internal/proxy/handler/native_upstream_test.go`
- [ ] T031 [P] [US4] Write `TestCompositeScore_AllSentinel` — all inputs <= 0 → score = 0 in `internal/proxy/handler/native_upstream_test.go`
- [ ] T032 [P] [US4] Write `TestLowestUtilSelect_Util7dSonnetSentinel` — token with u7ds=-1 beats token with u7ds=0.5 in `internal/proxy/handler/native_upstream_test.go`
- [ ] T033 [P] [US4] Update `TestLoadClaudeCodeData_CompositeScore_OneNilField` — nil sonnetUsage treated as 0 in `internal/ui/handler_claude_code_test.go`
- [ ] T034 [US4] Run `go test ./internal/... -run "TestCompositeScore_SonnetSentinel|TestCompositeScore_AllSentinel|TestLowestUtilSelect_Util7dSonnetSentinel|TestLoadClaudeCodeData_CompositeScore_OneNilField" -v` — confirm tests FAIL (new tests) or reflect updated expectations

### Implementation for User Story 4

- [ ] T035 [US4] Sentinel handling is already implemented in T001 (CompositeScore) and T009 (lowestUtilizationSelect). Verify no additional code needed — run all US4 tests.
- [ ] T036 [US4] Run `go test ./internal/... -run "TestCompositeScore_Sentinel|TestLowestUtilSelect_Util7dSonnet|TestLoadClaudeCodeData_CompositeScore_OneNilField" -v` — confirm ALL tests PASS

**Checkpoint**: System handles missing data gracefully. All sentinel paths verified.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final validation across all stories.

- [ ] T037 Run full test suite `go test ./internal/callback/... ./internal/proxy/handler/... ./internal/ui/... -v` — confirm zero failures
- [ ] T038 Run `make lint` — confirm no lint errors
- [ ] T039 Run `make check` — confirm lint + test + build all pass
- [ ] T040 Update `lowestUtilizationSelect` godoc comment to describe max-normalized formula in `internal/proxy/handler/native_upstream.go`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: Empty — skip
- **Phase 2 (Foundational)**: T001 + T002 — BLOCKS all user stories (changes function signature)
- **Phase 3 (US1)**: Depends on Phase 2. Core selection logic.
- **Phase 4 (US2)**: Depends on Phase 2. Independent of US1 (gate layer vs score layer).
- **Phase 5 (US3)**: Depends on Phase 2. Independent of US1/US2 (UI layer).
- **Phase 6 (US4)**: Depends on Phase 2 + US1 implementation (sentinel handling in T001/T009). Test-only phase.
- **Phase 7 (Polish)**: Depends on all user stories complete.

### User Story Dependencies

- **US1 (P1)**: Depends on Phase 2 only. No cross-story dependencies.
- **US2 (P1)**: Depends on Phase 2 only. Can run in parallel with US1 (different code paths: gate vs score).
- **US3 (P2)**: Depends on Phase 2 only. Can run in parallel with US1/US2 (UI layer, different files).
- **US4 (P2)**: Depends on US1 implementation (T001 + T009 contain sentinel logic). Must run after US1.

### Within Each User Story

- Tests MUST be written and confirmed to FAIL before implementation
- Implementation follows test list from plan.md `## Failing Tests`
- Story complete when all tests pass

### Parallel Opportunities

- **Phase 2**: T001 and T002 can run in parallel (different files)
- **US1 + US2 + US3**: Can start in parallel after Phase 2 (different code paths)
- **Within each US**: All test tasks marked [P] can run in parallel
- **US4**: Must wait for US1 implementation (depends on sentinel handling code)

---

## Parallel Example: User Story 1

```bash
# Launch all failing tests in parallel (different test functions, same file):
Task T003: "TestCompositeScore_MaxNormalized"
Task T004: "TestCompositeScore_MaxNormalized_AllZero"
Task T005: "TestCompositeScore_MaxNormalized_SonnetDominates"
Task T006: "TestLowestUtilSelect_PreferLowSonnet"
Task T007: "TestLowestUtilSelect_7dAllStillDominates"
```

---

## Implementation Strategy

### MVP First (User Story 1 + 2 Only)

1. Complete Phase 2: Foundational (T001-T002)
2. Complete Phase 3: US1 — Sonnet-aware selection (T003-T014)
3. Complete Phase 4: US2 — 7d_s gate (T015-T022)
4. **STOP and VALIDATE**: Run `go test ./internal/proxy/handler/... -v`
5. Deploy — core bug fix is live

### Incremental Delivery

1. Phase 2 → Foundation (signature change)
2. US1 → Sonnet-aware scoring (core fix)
3. US2 → Hard gate safety net (defense in depth)
4. US3 → UI update (operator visibility)
5. US4 → Sentinel verification (robustness)
6. Polish → Full suite green

---

## Notes

- All changes are in existing files — no new files created
- Total: 40 tasks across 7 phases
- US1: 12 tasks (5 test + 1 verify-fail + 5 implementation + 1 verify-pass)
- US2: 8 tasks (4 test + 1 verify-fail + 2 implementation + 1 verify-pass)
- US3: 7 tasks (2 test + 1 verify-fail + 3 implementation + 1 verify-pass)
- US4: 7 tasks (4 test + 1 verify-fail + 1 verify-implementation + 1 verify-pass)
- Foundational: 2 tasks
- Polish: 4 tasks
- Parallel opportunities: US1, US2, US3 can all start simultaneously after Phase 2
