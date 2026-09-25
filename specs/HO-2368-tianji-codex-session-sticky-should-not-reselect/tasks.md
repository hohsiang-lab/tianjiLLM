# Tasks: HO-2368 Codex session sticky reuse across primary reset

**Input**: Design documents from `/specs/HO-2368-tianji-codex-session-sticky-should-not-reselect/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Failing tests are MANDATORY (Constitution Principle IV). Task 1 of every user story writes failing tests and confirms they fail before implementation.

## Phase 1: Setup (Shared Context)

**Purpose**: Confirm scope and branch identity before implementation.

- [X] T001 Verify worktree identity and clean baseline with `git status -sb` in `/root/.openclaw/workspace-sima/worktrees/tianjiLLM/HO-2368-tianji-codex-session-sticky-should-not-reselect`
- [X] T002 Read `specs/HO-2368-tianji-codex-session-sticky-should-not-reselect/spec.md`, `plan.md`, and `contracts/codex-session-sticky-policy.md`
- [X] T003 Inspect owning code in `internal/proxy/handler/openai_subscription_routing.go` and tests in `internal/proxy/handler/openai_subscription_routing_test.go`

---

## Phase 2: Foundational (Policy Boundary)

**Purpose**: Establish session-vs-fallback boundary before story changes.

- [X] T004 [P] Confirm current fallback primary-reset behavior in `TestOpenAISubscriptionRouting_StickyCodexPrimaryWindowResetReevaluates` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T005 [P] Confirm existing session route key shape from `openAISubscriptionCodexSessionRouteKey` and log redaction shape from `safeOpenAISubscriptionTrackLogValue` in `internal/proxy/handler/openai_subscription_routing.go`

---

## Phase 3: User Story 1 - Session affinity survives primary reset changes (Priority: P1) MVP

**Goal**: Session-aware sticky tracks reuse a known/selectable selected credential even when only primary reset changes.

**Independent Test**: The new primary-reset-changed session test fails before implementation and passes after the reuse policy split.

### Tests for User Story 1 (MANDATORY)

- [X] T006 [P] [US1] Write failing `TestCodexSessionSticky_PrimaryResetChangedDoesNotReselectWhenStillSelectable` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T007 [US1] Run `go test ./internal/proxy/handler -run 'TestCodexSessionSticky_PrimaryResetChangedDoesNotReselectWhenStillSelectable|TestOpenAISubscriptionRouting_StickyCodexPrimaryWindowResetReevaluates' -count=1` and record that the new test fails before production changes

### Implementation for User Story 1

- [X] T008 [US1] Add a session-aware route-key helper or equivalent local branch in `internal/proxy/handler/openai_subscription_routing.go`
- [X] T009 [US1] Update `codexStickyCanReuseWithTrack` in `internal/proxy/handler/openai_subscription_routing.go` so known/selectable session-aware candidates reuse despite `PrimaryResetAt` changes
- [X] T010 [US1] Preserve fallback `PrimaryResetAt` equality behavior for non-session route keys in `internal/proxy/handler/openai_subscription_routing.go`

**Checkpoint**: US1 proves the reported primary-reset-only session switch is fixed while fallback behavior remains unchanged.

---

## Phase 4: User Story 2 - Session stickiness still yields to unusable credentials (Priority: P1)

**Goal**: Session affinity does not bypass primary or weekly usage gates.

**Independent Test**: Gate-specific tests fail before implementation when needed and pass after implementation, proving non-selectable selected credentials reselect.

### Tests for User Story 2 (MANDATORY)

- [X] T011 [P] [US2] Write failing `TestCodexSessionSticky_StillReselectsWhenPrimaryGateExceeded` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T012 [P] [US2] Write failing `TestCodexSessionSticky_StillReselectsWhenSecondaryGateExceeded` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T013 [US2] Run `go test ./internal/proxy/handler -run 'TestCodexSessionSticky_StillReselectsWhenPrimaryGateExceeded|TestCodexSessionSticky_StillReselectsWhenSecondaryGateExceeded|TestOpenAISubscriptionRouting_StickyCodexMissingCurrentSnapshotReevaluatesToKnownCandidate' -count=1` and record failing-test baseline for new tests

### Implementation for User Story 2

- [X] T014 [US2] Ensure `codexStickyCanReuseWithTrack` still returns false for known `Selectable=false` metadata in `internal/proxy/handler/openai_subscription_routing.go`
- [X] T015 [US2] Verify primary and secondary gate tests pass without changing disabled/missing credential filtering or `filterOpenAISubscriptionCodexUsageSelectable`

**Checkpoint**: US2 proves session stickiness releases unsafe selected credentials.

---

## Phase 5: User Story 3 - Observability explains session-specific reuse decisions (Priority: P2)

**Goal**: Logs remain safe and no longer describe session-aware primary-reset-only reuse as a reselect.

**Independent Test**: Log-focused tests prove no raw session id leaks and no session-aware primary-reset-only reselect reason appears when reuse is allowed.

### Tests for User Story 3 (MANDATORY)

- [X] T016 [P] [US3] Write failing `TestCodexSessionSticky_PrimaryResetChangedDoesNotLogReselectReasonWhenReused` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T017 [US3] Run `go test ./internal/proxy/handler -run 'TestCodexSessionSticky_PrimaryResetChangedDoesNotLogReselectReasonWhenReused|TestCodexSessionSticky_LogsDoNotExposeRawSessionID' -count=1` and record failing-test baseline for the new log test

### Implementation for User Story 3

- [X] T018 [US3] Add or adjust session-aware reuse logging in `internal/proxy/handler/openai_subscription_routing.go` only if needed for observability, using `safeOpenAISubscriptionTrackLogValue`
- [X] T019 [US3] Verify raw session ids remain absent from logs in `internal/proxy/handler/openai_subscription_routing_test.go`

**Checkpoint**: US3 proves verification can distinguish fixed session reuse from fallback re-evaluation.

---

## Phase 6: Verification & Handoff

**Purpose**: Validate focused scope and prepare review.

- [X] T020 Run `go test ./internal/proxy/handler -run 'Codex.*Sticky|OpenAISubscriptionRouting_StickyCodex|CodexSession' -count=1`
- [X] T021 Run `go test ./internal/proxy/handler -count=1`
- [X] T022 Run `git diff --check origin/main...HEAD`
- [X] T023 Verify `git diff --name-only origin/main...HEAD` contains only HO-2368 routing/test changes plus `specs/HO-2368-tianji-codex-session-sticky-should-not-reselect/`
- [X] T024 Update PR description with implementation evidence and note UI N/A, DB/config/rendezvous non-goals, and Missing part status

## Dependencies & Execution Order

- Phase 1 must complete before any test or code work.
- Phase 2 confirms boundaries before US1.
- US1 and US2 are both P1; implement US1 first because it introduces the policy split, then US2 verifies the gates remain intact.
- US3 depends on US1's reuse behavior.
- Phase 6 runs after all selected user stories are complete.

## Parallel Opportunities

- T004 and T005 can run in parallel.
- T006, T011, T012, and T016 can be drafted independently in the same test file but should be committed as coherent failing-test work.
- No implementation task should run before its related failing test has been observed failing.

## Implementation Strategy

1. Write and run failing tests for US1.
2. Implement the smallest session-aware reuse policy split.
3. Add gate regression tests for US2 and verify they pass under the same policy.
4. Add observability regression for US3.
5. Run focused handler validation and diff-scope checks.

## Scope Stop

Todo planning stops at these artifacts and draft PR. Production implementation starts only after Linear moves from `Todo` to `Waiting` and then an assignee-authorized start moves the issue to `In Progress`.
