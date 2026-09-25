# Tasks: Proactive OpenAI Subscription Credential Refresh

**Input**: Design documents from `/specs/HO-2091-proactively-refresh-idle-openai-subscription/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/proactive-refresh.md

**Tests**: Failing tests are MANDATORY. Task 1 of every user story writes tests and confirms they fail before implementation tasks begin.

**Organization**: Tasks are grouped by user story so each story can be independently implemented and tested.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare generated DB access and shared test harness.

- [X] T001 Add failing proactive-refresh candidate query expectation in `internal/db/queries/credential.sql`
- [X] T002 Run `make generate` to update `internal/db/credential.sql.go` and `internal/db/interface.go`
- [X] T003 [P] Add shared proactive refresh test helpers in `internal/proxy/handler/openai_subscription_refresh_test.go`
- [X] T004 [P] Add scheduler job result test helper in `internal/scheduler/jobs_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Define shared eligibility and metadata semantics before any user story implementation.

- [X] T005 Add OpenAI subscription proactive buffer constant and selectable-status helper in `internal/proxy/handler/openai_subscription_refresh.go`
- [X] T006 Add safe reconnect-required metadata fields to `OpenAISubscriptionCredentialInfo` in `internal/proxy/handler/credentials.go`
- [X] T007 Add refresh failure classification helper for `refresh_token_invalidated` in `internal/proxy/handler/openai_subscription_refresh.go`
- [X] T008 Add route candidate stale/non-selectable reason helper in `internal/proxy/handler/openai_subscription_routing.go`

**Checkpoint**: Foundation ready; user stories can begin.

---

## Phase 3: User Story 1 - Idle Credentials Refresh Before Expiry (Priority: P1) MVP

**Goal**: Active idle credentials refresh before expiry with a 30-minute proactive buffer.

**Independent Test**: Run the proactive job once against mock credentials and assert refresh/skip/write counts.

### Tests for User Story 1 (MANDATORY)

- [X] T009 [P] [US1] Write failing test `TestOpenAISubscriptionProactiveRefreshJob_RefreshesIdleCredentialInsideBuffer` in `internal/proxy/handler/openai_subscription_refresh_test.go`
- [X] T010 [P] [US1] Write failing test `TestOpenAISubscriptionProactiveRefreshJob_SkipsCredentialOutsideBuffer` in `internal/proxy/handler/openai_subscription_refresh_test.go`
- [X] T011 [P] [US1] Write failing test `TestOpenAISubscriptionProactiveRefreshJob_RegisteredEveryFiveMinutesWhenDBConfigured` in `internal/scheduler/jobs_test.go` or command wiring test
- [X] T012 [US1] Run focused `go test` for T009-T011 and confirm tests fail for missing implementation

### Implementation for User Story 1

- [X] T013 [US1] Implement candidate scan/decrypt/skip logic in `internal/proxy/handler/openai_subscription_refresh.go`
- [X] T014 [US1] Implement `OpenAISubscriptionProactiveRefreshJob` in `internal/scheduler/jobs.go`
- [X] T015 [US1] Register the job at 5-minute cadence in `cmd/tianji/main.go`
- [X] T016 [US1] Run focused US1 tests and confirm they pass

**Checkpoint**: Idle active credentials refresh before expiry; outside-buffer credentials are skipped.

---

## Phase 4: User Story 2 - No Stampede Across Pods Or Request-Time Resolution (Priority: P1)

**Goal**: Proactive refresh coordinates with request-time refresh and concurrent pod jobs.

**Independent Test**: Concurrent proactive/request resolution produces one upstream refresh and lock skips are counted as skips.

### Tests for User Story 2 (MANDATORY)

- [X] T017 [P] [US2] Write failing test `TestOpenAISubscriptionProactiveRefreshJob_SharesSingleflightWithRequestResolution` in `internal/proxy/handler/openai_subscription_refresh_test.go`
- [X] T018 [P] [US2] Write failing test `TestOpenAISubscriptionProactiveRefreshJob_DistributedLockSkipsSecondRunner` in `internal/scheduler/jobs_test.go`
- [X] T019 [US2] Run focused `go test` for T017-T018 and confirm tests fail for missing lock integration

### Implementation for User Story 2

- [X] T020 [US2] Route proactive refresh through the existing normal credential refresh `singleflight` key in `internal/proxy/handler/openai_subscription_refresh.go`
- [X] T021 [US2] Wrap the scheduler job with existing Redis distributed lock when Redis is configured in `cmd/tianji/main.go`
- [X] T022 [US2] Ensure lock-not-acquired results are logged/counted as skipped in `internal/scheduler/jobs.go`
- [X] T023 [US2] Run focused US2 tests and confirm they pass

**Checkpoint**: Same credential has no proactive/request-time stampede in tested concurrency surfaces.

---

## Phase 5: User Story 3 - Reconnect Required State Is Persisted And Visible (Priority: P1)

**Goal**: Invalidated OpenAI sessions become clear non-selectable reconnect-required credentials.

**Independent Test**: Mock `refresh_token_invalidated` persists first failure timestamp/reason and UI/action text says reconnect required.

### Tests for User Story 3 (MANDATORY)

- [X] T024 [P] [US3] Write failing test `TestOpenAISubscriptionProactiveRefreshJob_RefreshTokenInvalidatedMarksReconnectRequired` in `internal/proxy/handler/openai_subscription_refresh_test.go`
- [X] T025 [P] [US3] Write failing test `TestOpenAISubscriptionProactiveRefreshJob_TransientFailureDoesNotClaimReconnectRequired` in `internal/proxy/handler/openai_subscription_refresh_test.go`
- [X] T026 [P] [US3] Write failing test `TestOpenAISubscriptionProactiveRefreshJob_SuccessClearsTransientFailureMetadata` in `internal/proxy/handler/openai_subscription_refresh_test.go`
- [X] T027 [US3] Run focused `go test` for T024-T026 and confirm tests fail for missing metadata semantics

### Implementation for User Story 3

- [X] T028 [US3] Persist first/latest failure timestamps and reason in `internal/proxy/handler/openai_subscription_refresh.go`
- [X] T029 [US3] Persist operator message `OpenAI session ended; reconnect required` for invalidated sessions in `internal/proxy/handler/openai_subscription_refresh.go`
- [X] T030 [US3] Clear transient failure metadata on successful refresh in `internal/proxy/handler/credentials.go`
- [X] T031 [US3] Surface reconnect-required message in `internal/proxy/handler/openai_subscription_lifecycle.go` and `internal/ui/handler_credentials.go`
- [X] T032 [US3] Run focused US3 tests and confirm they pass

**Checkpoint**: Operators can distinguish reconnect-required from transient refresh failure.

---

## Phase 6: User Story 4 - Stale Route Candidates Stop Repeating Noise (Priority: P2)

**Goal**: Routing skips failed/deleted credentials and stale route references are visible.

**Independent Test**: Route configs containing failed/missing credentials produce skipped candidates and visible stale-reference diagnostics.

### Tests for User Story 4 (MANDATORY)

- [X] T033 [P] [US4] Write failing test `TestResolveOpenAISubscriptionCandidates_RefreshFailedExcluded` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T034 [P] [US4] Write failing test `TestModelsUI_OpenAISubscriptionStaleReferencesVisible` in `internal/ui/handler_models_test.go`
- [X] T035 [P] [US4] Write failing test `TestOpenAISubscriptionRouting_AllFailedCandidatesReturnsClearUnusableError` in `internal/proxy/handler/openai_subscription_routing_test.go`
- [X] T036 [P] [US4] Write failing test `TestOpenAISubscriptionCredentialFailureMetadata_RedactsSecrets` in `internal/proxy/handler/openai_subscription_lifecycle_test.go`
- [X] T037 [US4] Run focused `go test` for T033-T036 and confirm tests fail for missing routing/UI behavior

### Implementation for User Story 4

- [X] T038 [US4] Exclude `refresh_failed` and reconnect-required credentials in `internal/proxy/handler/openai_subscription_refresh.go` and `internal/proxy/handler/openai_subscription_routing.go`
- [X] T039 [US4] Add stale reference diagnostics to model data loading in `internal/ui/handler_models.go`
- [X] T040 [US4] Render stale OpenAI subscription references in `internal/ui/pages/models.templ` (existing auth cell renders the new status text; no template delta required)
- [X] T041 [US4] Regenerate templ output for `internal/ui/pages/models_templ.go` (N/A: no template delta)
- [X] T042 [US4] Ensure aggregate unusable errors remain sanitized in `internal/proxy/handler/openai_subscription_routing.go`
- [X] T043 [US4] Run focused US4 tests and confirm they pass

**Checkpoint**: Failed/deleted/stale candidates do not create repeated route selection noise.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Verify full affected surface and keep evidence clean.

- [X] T044 [P] Run `go test ./internal/proxy/handler/... ./internal/scheduler/... ./internal/ui/... -count=1`
- [X] T045 [P] Run `make generate` and verify no unexpected sqlc drift remains
- [X] T046 [P] Run templ generation if UI templates changed (N/A: no UI template changed)
- [X] T047 Run `git diff --check origin/main...HEAD`
- [X] T048 Update `specs/HO-2091-proactively-refresh-idle-openai-subscription/analyze.md` after implementation if task coverage changes
- [X] T049 Update PR/Linear evidence with exact test commands and redaction proof
- [X] T050 Record UI alignment evidence for the existing credentials/models operator surfaces; no new mockscreen is required unless implementation introduces a new layout or interaction pattern

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: no dependencies.
- **Foundational (Phase 2)**: depends on Setup and blocks all user stories.
- **US1 and US2**: both P1; US1 should land first because it creates the job behavior, then US2 hardens concurrency.
- **US3**: depends on US1 refresh execution and failure path.
- **US4**: depends on US3 non-selectable state.
- **Polish**: depends on selected user stories being complete.

### Parallel Opportunities

- T003/T004 can run in parallel.
- Test-writing tasks inside each user story can run in parallel before implementation.
- UI stale-reference work and routing exclusion work can run in parallel after US3 metadata state is stable.

## Implementation Strategy

### MVP First

1. Complete Setup and Foundational tasks.
2. Complete US1 to prove idle credentials refresh before expiry.
3. Complete US2 before rollout so job concurrency does not create refresh stampedes.
4. Complete US3/US4 to make failures operator-actionable and quiet stale-route noise.

### Stop Conditions

- Stop before production code while Linear HO-2091 remains Todo or Waiting.
- Stop if cross-pod lock cannot be implemented with existing Redis `redsync` or a tested DB claim path.
- Stop if a metadata shape would require exposing or storing secret material.
- Stop if UI stale-reference handling would silently mutate route config without operator-visible evidence.
- Stop if implementation introduces a new UI layout or interaction pattern without mockscreen/source-design evidence.
