# Tasks: OAuth Token 智能限流

**Input**: Design documents from `/specs/079-oauth-token-throttle/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md

**Tests**: Failing tests are MANDATORY (Constitution Principle IV). Task 1 of every user story MUST be "Write failing tests" based on the plan's `## Failing Tests` section. No implementation task may begin until failing tests are written and confirmed to fail.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Rename existing function to make room for new throttle method

- [x] T001 Rename `selectUpstream` → `roundRobinSelect` in `internal/proxy/handler/native_upstream.go` (same signature, same logic, pure rename)
- [x] T002 Update call site in `internal/proxy/handler/native_format.go` line 29: change `selectUpstream(` → `roundRobinSelect(` (temporary — will be replaced in US1)

**Checkpoint**: All existing tests pass. No behavior change.

---

## Phase 2: Foundational

**Purpose**: Add new types and helpers needed by all user stories

- [x] T003 Add `allTokensThrottledError` struct (with `resetAt time.Time` and `Error() string`) in `internal/proxy/handler/native_upstream.go`
- [x] T004 Add `parseUnixResetTime(s string) time.Time` helper in `internal/proxy/handler/native_upstream.go`

**Checkpoint**: Foundation ready — compiles, no new behavior yet.

---

## Phase 3: User Story 1 — 自动跳过高利用率 Token (Priority: P1)

**Goal**: Token selection filters out OAuth tokens with 5h/7d utilization >= threshold

**Independent Test**: Configure 3 OAuth tokens, set 1 to 85% utilization in store, verify requests only route to other 2.

### Tests for User Story 1 (MANDATORY - Principle IV)

> **MANDATORY: Write these tests FIRST from plan's ## Failing Tests section. Confirm they FAIL before any implementation task.**

- [x] T005 [P] [US1] Write `TestSelectUpstreamThrottle_Skips5hOverThreshold` in `internal/proxy/handler/native_upstream_test.go` — 3 tokens, token A 5h=0.85, assert selected is NOT A
- [x] T006 [P] [US1] Write `TestSelectUpstreamThrottle_Skips7dOverThreshold` in `internal/proxy/handler/native_upstream_test.go` — token B 7d=0.90, assert selected is NOT B
- [x] T007 [P] [US1] Write `TestSelectUpstreamThrottle_RecoversBelowThreshold` in `internal/proxy/handler/native_upstream_test.go` — token A 5h=0.60, assert A is selectable
- [x] T008 [P] [US1] Write `TestSelectUpstreamThrottle_SkipsRateLimitedStatus` in `internal/proxy/handler/native_upstream_test.go` — token A status="rate_limited", assert skipped
- [x] T009 [P] [US1] Write `TestSelectUpstreamThrottle_UnknownStateIsAvailable` in `internal/proxy/handler/native_upstream_test.go` — token not in store, assert available
- [x] T010 [P] [US1] Write `TestSelectUpstreamThrottle_SentinelNeg1IsAvailable` in `internal/proxy/handler/native_upstream_test.go` — utilization=-1, assert available
- [x] T011 [P] [US1] Write `TestSelectUpstreamThrottle_NonOAuthNotThrottled` in `internal/proxy/handler/native_upstream_test.go` — non-OAuth key, assert never throttled
- [x] T012 [P] [US1] Write `TestSelectUpstreamThrottle_DeduplicatesByAPIKey` in `internal/proxy/handler/native_upstream_test.go` — 3 upstreams same key, assert deduplicated

> **Run**: `go test ./internal/proxy/handler/... -run "TestSelectUpstreamThrottle" -v` — all 8 tests MUST compile and FAIL.

### Implementation for User Story 1

- [x] T013 [US1] Implement `selectUpstreamWithThrottle` method on `*Handlers` in `internal/proxy/handler/native_upstream.go` — filter logic: dedup by APIKey, check store for 5h/7d utilization >= threshold, skip rate_limited status, delegate to `roundRobinSelect` for available pool
- [x] T014 [US1] Replace `roundRobinSelect(providerName, upstreams)` call in `internal/proxy/handler/native_format.go` line 29 with `h.selectUpstreamWithThrottle(providerName, upstreams)` — handle returned error (defer 429 handling to US2)

> **Run**: `go test ./internal/proxy/handler/... -run "TestSelectUpstreamThrottle" -v` — all 8 tests MUST PASS.

**Checkpoint**: Throttle selection works. High-utilization tokens are skipped. Existing tests still pass.

---

## Phase 4: User Story 2 — 所有 Token 耗尽返回 429 (Priority: P1)

**Goal**: When all tokens exceed threshold, return HTTP 429 with Retry-After header

**Independent Test**: Set all token utilizations above threshold, send request, verify 429 + Retry-After.

### Tests for User Story 2 (MANDATORY - Principle IV)

> **MANDATORY: Write these tests FIRST from plan's ## Failing Tests section. Confirm they FAIL before any implementation task.**

- [x] T015 [P] [US2] Write `TestSelectUpstreamThrottle_AllThrottled_ReturnsError` in `internal/proxy/handler/native_upstream_test.go` — all tokens over threshold, assert returns `allTokensThrottledError`
- [x] T016 [P] [US2] Write `TestSelectUpstreamThrottle_AllThrottled_NearestReset` in `internal/proxy/handler/native_upstream_test.go` — error contains nearest reset time
- [x] T017 [P] [US2] Write `TestSelectUpstreamThrottle_SingleTokenThrottled_Returns429` in `internal/proxy/handler/native_upstream_test.go` — only 1 token, over threshold, assert error
- [x] T018 [P] [US2] Write `TestSelectUpstreamThrottle_ConfigurableThreshold` in `internal/proxy/handler/native_upstream_test.go` — threshold=0.5, token at 0.6 throttled, at 0.4 available

> **Run**: `go test ./internal/proxy/handler/... -run "TestSelectUpstreamThrottle_(All|Single|Configurable)" -v` — all 4 tests MUST compile and FAIL.

### Implementation for User Story 2

- [x] T019 [US2] Add 429 + Retry-After response handling in `internal/proxy/handler/native_format.go` — when `selectUpstreamWithThrottle` returns `*allTokensThrottledError`, write 429 with `Retry-After` header and `model.ErrorResponse` body

> **Run**: `go test ./internal/proxy/handler/... -run "TestSelectUpstreamThrottle" -v` — all 12 tests MUST PASS.

**Checkpoint**: Full throttle + 429 flow works end-to-end. US1 + US2 independently functional.

---

## Phase 5: User Story 3 — Discord 告警通知 (Priority: P2)

**Goal**: Send Discord alerts when token utilization crosses threshold or status becomes rate_limited

**Independent Test**: Configure Discord webhook, set token to 85% utilization, verify webhook receives alert message.

### Tests for User Story 3 (MANDATORY - Principle IV)

> **MANDATORY: Write these tests FIRST from plan's ## Failing Tests section. Confirm they FAIL before any implementation task.**

- [x] T020 [P] [US3] Write `TestCheckAndAlertOAuth_5hOverThreshold` in `internal/callback/discord_ratelimit_test.go` — 5h utilization >= threshold, assert webhook called
- [x] T021 [P] [US3] Write `TestCheckAndAlertOAuth_Cooldown` in `internal/callback/discord_ratelimit_test.go` — same token within 1h, assert webhook NOT called again
- [x] T022 [P] [US3] Write `TestCheckAndAlertOAuth_RateLimitedStatus` in `internal/callback/discord_ratelimit_test.go` — status="rate_limited", assert webhook called
- [x] T023 [P] [US3] Write `TestCheckAndAlertOAuth_NilAlerter` in `internal/callback/discord_ratelimit_test.go` — nil alerter, assert no panic
- [x] T024 [P] [US3] Write `TestCheckAndAlertOAuth_7dOverThreshold` in `internal/callback/discord_ratelimit_test.go` — 7d utilization >= threshold, assert webhook called

> **Run**: `go test ./internal/callback/... -run "TestCheckAndAlertOAuth" -v` — all 5 tests MUST compile and FAIL.

### Implementation for User Story 3

- [x] T025 [US3] Restore `CheckAndAlertOAuth` method in `internal/callback/discord_ratelimit.go` — from reverted commit eec5ca2, add 7d utilization check
- [x] T026 [US3] Restore `sendOAuthAlertIfNotCooling` method in `internal/callback/discord_ratelimit.go` — from reverted commit eec5ca2
- [x] T027 [US3] Wire `h.DiscordAlerter.CheckAndAlertOAuth(rlState)` in `internal/proxy/handler/native_format.go` — both 200 path (after line 134) and non-200 path (after line 121), guarded by `h.DiscordAlerter != nil && anthropic.IsOAuthToken(apiKey)`

> **Run**: `go test ./internal/callback/... -run "TestCheckAndAlertOAuth" -v` — all 5 tests MUST PASS.

**Checkpoint**: All 3 user stories independently functional. Full feature complete.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [x] T028 Run full test suite: `make test` — verify zero regressions
- [x] T029 Run linter: `make lint` — verify zero new warnings
- [x] T030 Run full check: `make check` — lint + test + build all pass (templ CLI unavailable, Go lint+test+build OK)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (rename complete)
- **US1 (Phase 3)**: Depends on Phase 2 (types exist)
- **US2 (Phase 4)**: Depends on US1 (selectUpstreamWithThrottle exists)
- **US3 (Phase 5)**: Depends on Phase 2 only — can run in parallel with US1/US2
- **Polish (Phase 6)**: Depends on all user stories complete

### Within Each User Story

1. Tests MUST be written and FAIL before implementation
2. Implementation makes tests pass
3. Story complete before moving to next priority

### Parallel Opportunities

```bash
# Phase 1: sequential (2 tasks, same files)
T001 → T002

# Phase 2: parallel (different concerns in same file, but small — sequential OK)
T003 → T004

# US1 Tests: ALL parallel (same file, different functions)
T005, T006, T007, T008, T009, T010, T011, T012

# US1 Implementation: sequential
T013 → T014

# US2 Tests: ALL parallel
T015, T016, T017, T018

# US2 Implementation: single task
T019

# US3 Tests: ALL parallel (different file from US1/US2)
T020, T021, T022, T023, T024

# US3 Implementation: sequential
T025 → T026 → T027

# US3 can run in parallel with US1+US2 (different files)
```

---

## Implementation Strategy

### MVP First (US1 + US2)

1. Complete Phase 1: Setup (rename)
2. Complete Phase 2: Foundational (types)
3. Complete Phase 3: US1 (throttle selection)
4. Complete Phase 4: US2 (429 handling)
5. **STOP and VALIDATE**: `make test` passes, throttle + 429 works
6. Deploy MVP — Discord alerts can follow

### Full Feature

7. Complete Phase 5: US3 (Discord alerts)
8. Complete Phase 6: Polish
9. Full validation: `make check`

---

## Notes

- [P] tasks = different files or independent test functions, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Verify tests fail before implementing
- Commit after each phase checkpoint
- Total: 30 tasks (4 setup, 17 tests, 6 implementation, 3 polish)
