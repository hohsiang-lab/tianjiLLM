# Tasks: Enforce Per-Key Rate Limits & Budget Controls

**Input**: Design documents from `/specs/204-enforce-key-limits/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Tests**: Failing tests are MANDATORY (Constitution Principle IV). Task 1 of every user story MUST be "Write failing tests" based on the plan's `## Failing Tests` section. No implementation task may begin until failing tests are written and confirmed to fail.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Shared infrastructure changes that all user stories depend on

- [x] T001 Expand `TokenInfo` struct in `internal/proxy/middleware/db_validator.go` — add RpmLimit (*int64), TpmLimit (*int64), MaxBudget (*float64), Spend (float64), MaxParallelRequests (*int32), Models ([]string), Expires (pgtype.Timestamptz)
- [x] T002 Implement limit inheritance chain in `internal/proxy/middleware/db_validator.go` — ValidateToken() resolves key→team→org fallback: if key limit is nil and teamID != nil, call GetTeam(); if team limit also nil and orgID != nil, call GetOrganization(); if budgetID != nil, call GetBudget() for MaxParallelRequests
- [x] T003 [P] Add new context keys in `internal/proxy/middleware/helpers.go` — maxBudgetKey, spendKey (rpmLimitKey, tpmLimitKey, modelGroupKey, maxParallelLimitKey already exist)

---

## Phase 2: Foundational (Auth Context Injection)

**Purpose**: Auth middleware injects all limit values to context — BLOCKS all user stories

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Modify auth middleware virtual key path in `internal/proxy/middleware/auth.go` — after ValidateToken(), inject all TokenInfo limit fields to request context via context.WithValue: rpmLimitKey, tpmLimitKey, maxParallelLimitKey (cast int32→int), maxBudgetKey, spendKey
- [x] T005 Register budget middleware in `internal/proxy/server.go` — add NewBudgetMiddleware to middleware chain: Auth → **Budget** → Parallel → DynamicRate → CacheControl
- [x] T006 Fix Gemini/Batches middleware gap in `internal/proxy/server.go` — apply full llmMiddleware chain (Parallel + DynamicRate + CacheControl + Budget) to `/v1beta/models/*` and `/anthropic/v1/*` route groups (currently only have Auth)

**Checkpoint**: Auth injects limits to context, budget middleware is in the chain, all routes have full middleware coverage

---

## Phase 3: User Story 1 — RPM/TPM Rate Limiting (Priority: P1) 🎯 MVP

**Goal**: RPM and TPM limits set on a key are enforced — exceeding requests get 429

**Independent Test**: Set RPM=5 on a key, send 6 requests in 1 minute, 6th returns 429

### Tests for User Story 1 (MANDATORY — Principle IV) 🔴

- [x] T007 [P] [US1] Write failing test `TestAuthMiddleware_InjectsRPMLimit` in `internal/proxy/middleware/auth_test.go` — context contains rpmLimitKey=10 when key has rpm_limit=10
- [x] T008 [P] [US1] Write failing test `TestAuthMiddleware_InjectsTPMLimit` in `internal/proxy/middleware/auth_test.go` — context contains tpmLimitKey=200000 when key has tpm_limit=200000
- [x] T009 [P] [US1] Write failing test `TestAuthMiddleware_FallbackToTeamRPM` in `internal/proxy/middleware/auth_test.go` — key rpm_limit=nil, team rpm_limit=20 → context has 20
- [x] T010 [P] [US1] Write failing test `TestAuthMiddleware_FallbackToOrgTPM` in `internal/proxy/middleware/auth_test.go` — key=nil, team=nil, org tpm_limit=500000 → context has 500000
- [x] T011 [P] [US1] Write failing test `TestAuthMiddleware_NullLimitsNoInjection` in `internal/proxy/middleware/auth_test.go` — all null → rpmLimitKey=0, passes through
- [x] T012 [P] [US1] Write failing test `TestDynamicRateLimit_RPMExceeded` in `internal/proxy/middleware/dynamic_ratelimit_test.go` — 11th request returns 429 when RPM=10
- [x] T013 [P] [US1] Write failing test `TestDynamicRateLimit_TPMExceeded` in `internal/proxy/middleware/dynamic_ratelimit_test.go` — request rejected when TPM counter >= limit
- [x] T014 [P] [US1] Write failing test `TestDynamicRateLimit_WindowReset` in `internal/proxy/middleware/dynamic_ratelimit_test.go` — after 60s TTL, new request allowed
- [x] T014a [P] [US1] Write failing test `TestDynamicRateLimit_SetsRateLimitHeaders` in `internal/proxy/middleware/dynamic_ratelimit_test.go` — response contains X-RateLimit-Limit-Requests, X-RateLimit-Remaining-Requests, X-RateLimit-Reset-Requests headers when RPM limit is set (FR-011)
- [x] T015 [US1] Run `go test ./internal/proxy/middleware/... -run "TestAuthMiddleware_Injects|TestAuthMiddleware_Fallback|TestAuthMiddleware_Null|TestDynamicRateLimit_RPM|TestDynamicRateLimit_TPM|TestDynamicRateLimit_Window|TestDynamicRateLimit_Sets" -v` — confirm all tests compile and FAIL

### Implementation for User Story 1

- [x] T016 [US1] Implement RPM/TPM context injection in `internal/proxy/middleware/auth.go` — deref *int64 to int64 (or 0 if nil), WithValue for rpmLimitKey and tpmLimitKey
- [x] T017 [US1] Add post-call TPM counter update in `internal/spend/tracker.go` — `INCRBY tianji:dynamic_tpm:{keyHash} totalTokens` + Expire 60s in Record() method; inject redis.UniversalClient into Tracker struct
- [x] T018 [US1] Run tests from T015 — confirm all PASS

**Checkpoint**: RPM/TPM limits enforced. Key with RPM=10 blocks 11th request/minute.

---

## Phase 4: User Story 2 — Max Budget (Priority: P1)

**Goal**: Keys with max_budget set are blocked when spend >= max_budget

**Independent Test**: Set max_budget=$1 on a key, spend until $1+, next request returns 429

### Tests for User Story 2 (MANDATORY — Principle IV) 🔴

- [x] T019 [P] [US2] Write failing test `TestBudgetMiddleware_SpendBelowBudget` in `internal/proxy/middleware/budget_test.go` — spend=$99.50, max_budget=$100 → 200 OK
- [x] T020 [P] [US2] Write failing test `TestBudgetMiddleware_SpendExceedsBudget` in `internal/proxy/middleware/budget_test.go` — spend=$100.01, max_budget=$100 → 429
- [x] T021 [P] [US2] Write failing test `TestBudgetMiddleware_NullBudgetPassThrough` in `internal/proxy/middleware/budget_test.go` — max_budget=nil → 200 OK
- [x] T022 [P] [US2] Write failing test `TestBudgetMiddleware_FallbackToTeamBudget` in `internal/proxy/middleware/budget_test.go` — key budget=nil, team budget=$500, team spend=$501 → 429
- [x] T023 [P] [US2] Write failing test `TestBudgetMiddleware_ReadsFromContext` in `internal/proxy/middleware/budget_test.go` — middleware reads spend/max_budget from context, not DB
- [x] T023a [P] [US2] Write failing test `TestSpendTracker_UpdatesRedisSpendCache` in `internal/spend/tracker_test.go` — after Record(), Redis key `tianji:spend:{keyHash}` is incremented by cost (FR-006)
- [x] T024 [US2] Run `go test ./internal/proxy/middleware/... ./internal/spend/... -run "TestBudgetMiddleware_|TestSpendTracker_UpdatesRedis" -v` — confirm all tests compile and FAIL

### Implementation for User Story 2

- [x] T025 [US2] Refactor budget middleware in `internal/proxy/middleware/budget.go` — change from direct DB query to reading maxBudgetKey/spendKey from context; add Redis spend cache check (`tianji:spend:{keyHash}`) with max(dbSpend, cacheSpend); fail-open on Redis error
- [x] T026 [US2] Add post-call Redis spend cache update in `internal/spend/tracker.go` — `INCRBYFLOAT tianji:spend:{keyHash} cost` in Record() method
- [x] T027 [US2] Run tests from T024 — confirm all PASS

**Checkpoint**: Budget limits enforced. Key with max_budget=$100 and spend=$100+ is blocked.

---

## Phase 5: User Story 3 — Models Restriction (Priority: P1)

**Goal**: Keys with models list set can only use those models; requests to other models get 403

**Independent Test**: Set models=["qwen3-30b"] on key, request claude-opus → 403, request qwen3-30b → pass

### Tests for User Story 3 (MANDATORY — Principle IV) 🔴

- [x] T028 [P] [US3] Write failing test `TestAuthMiddleware_ModelNotAllowed` in `internal/proxy/middleware/auth_test.go` — models=["qwen3-30b"], request body model="claude-opus-4-5" → 403
- [x] T029 [P] [US3] Write failing test `TestAuthMiddleware_ModelAllowed` in `internal/proxy/middleware/auth_test.go` — models=["qwen3-30b"], request body model="qwen3-30b" → pass
- [x] T030 [P] [US3] Write failing test `TestAuthMiddleware_EmptyModelsAllowAll` in `internal/proxy/middleware/auth_test.go` — models=[] → any model allowed
- [x] T031 [P] [US3] Write failing test `TestAuthMiddleware_ModelWildcardMatch` in `internal/proxy/middleware/auth_test.go` — models=["qwen3-*"], request body "qwen3-30b" → pass
- [x] T032 [P] [US3] Write failing test `TestAuthMiddleware_BodyBufferingPreservesBody` in `internal/proxy/middleware/auth_test.go` — auth reads body for model, downstream handler can still read full body
- [x] T033 [P] [US3] Write failing test `TestAuthMiddleware_ModelFromURLPath` in `internal/proxy/middleware/auth_test.go` — /openai/deployments/gpt-4 → extracts model "gpt-4" from path
- [x] T034 [US3] Run `go test ./internal/proxy/middleware/... -run "TestAuthMiddleware_Model|TestAuthMiddleware_Empty|TestAuthMiddleware_Body" -v` — confirm all tests compile and FAIL

### Implementation for User Story 3

- [x] T035 [US3] Implement body buffering + model extraction in `internal/proxy/middleware/auth.go` — io.ReadAll + bytes.NewReader reset; partial JSON decode for model field; extractModelFromPath() fallback for URL-based model names
- [x] T036 [US3] Implement `isModelAllowed(model string, allowed []string) bool` in `internal/proxy/middleware/auth.go` — uses wildcard.Match for pattern support; returns 403 if not allowed
- [x] T037 [US3] Run tests from T034 — confirm all PASS

**Checkpoint**: Models restriction enforced. Key limited to qwen3-* cannot use claude-opus.

---

## Phase 6: User Story 4 — Max Parallel Requests (Priority: P2)

**Goal**: Keys with max_parallel_requests set are blocked when concurrent requests exceed limit

**Independent Test**: Set max_parallel=2, send 3 concurrent streaming requests, 3rd returns 429

### Tests for User Story 4 (MANDATORY — Principle IV) 🔴

- [x] T038 [P] [US4] Write failing test `TestAuthMiddleware_InjectsMaxParallel` in `internal/proxy/middleware/auth_test.go` — context has maxParallelLimitKey when budget has value
- [x] T039 [P] [US4] Write failing test `TestParallel_ExceedsLimit` in `internal/proxy/middleware/parallel_test.go` — 4th concurrent request returns 429 when limit=3
- [x] T040 [P] [US4] Write failing test `TestParallel_NullLimitPassThrough` in `internal/proxy/middleware/parallel_test.go` — no limit → all pass
- [x] T041 [US4] Run `go test ./internal/proxy/middleware/... -run "TestAuthMiddleware_InjectsMaxParallel|TestParallel_Exceeds|TestParallel_Null" -v` — confirm all tests compile and FAIL

### Implementation for User Story 4

- [x] T042 [US4] Implement max_parallel context injection in `internal/proxy/middleware/auth.go` — cast BudgetTable.MaxParallelRequests (*int32) to int, WithValue for maxParallelLimitKey
- [x] T043 [US4] Run tests from T041 — confirm all PASS

**Checkpoint**: Parallel limits enforced. Key with max_parallel=3 blocks 4th concurrent request.

---

## Phase 7: User Story 5 — Key Expiry (Priority: P2)

**Goal**: Expired keys are rejected with 403

**Independent Test**: Set expires to a past date, send request → 403

### Tests for User Story 5 (MANDATORY — Principle IV) 🔴

- [x] T044 [P] [US5] Write failing test `TestAuthMiddleware_ExpiredKey` in `internal/proxy/middleware/auth_test.go` — expires=past → 403
- [x] T045 [P] [US5] Write failing test `TestAuthMiddleware_ValidExpiry` in `internal/proxy/middleware/auth_test.go` — expires=future → pass
- [x] T046 [P] [US5] Write failing test `TestAuthMiddleware_NullExpiryNeverExpires` in `internal/proxy/middleware/auth_test.go` — expires=null → pass
- [x] T047 [US5] Run `go test ./internal/proxy/middleware/... -run "TestAuthMiddleware_Expired|TestAuthMiddleware_ValidExpiry|TestAuthMiddleware_NullExpiry" -v` — confirm all tests compile and FAIL

### Implementation for User Story 5

- [x] T048 [US5] Implement expiry check in `internal/proxy/middleware/auth.go` — after ValidateToken(), if info.Expires.Valid && time.Now().After(info.Expires.Time) → return 403 "key expired"
- [x] T049 [US5] Run tests from T047 — confirm all PASS

**Checkpoint**: Expired keys are rejected. Key with expires=2026-03-01 returns 403.

---

## Phase 8: User Story 6 — Router Backoff + Middleware Gap (Priority: P1)

**Goal**: Router retry uses exponential backoff; Gemini/Batches routes have full middleware chain

**Independent Test**:
- 6a: Mock upstream 429 + Retry-After:5, verify Router waits ≥5s
- 6b: Send request to /v1beta/models/* with RPM limit set, verify rate limit enforced

### Tests for User Story 6 (MANDATORY — Principle IV) 🔴

- [x] T050 [P] [US6] Write failing test `TestRouter_RetryWithExponentialBackoff` in `internal/router/router_test.go` — 1st retry ~1s, 2nd ~2s, 3rd ~4s
- [x] T051 [P] [US6] Write failing test `TestRouter_RespectsRetryAfterHeader` in `internal/router/router_test.go` — upstream 429 + Retry-After:5 → waits ≥5s
- [x] T052 [P] [US6] Write failing test `TestRouter_RetryAfterCapAt300s` in `internal/router/router_test.go` — Retry-After:3600 → capped to 300s, reject without retry
- [x] T053 [P] [US6] Write failing test `TestRouter_AllDeploymentsCooldown_ReturnsRetryAfter` in `internal/router/router_test.go` — all cooldown → 429 + Retry-After header
- [x] T054 [P] [US6] Write failing test `TestRouter_UsesRetryPolicySettings` in `internal/router/router_test.go` — RetryPolicy.RetryAfterSeconds used as backoff base
- [x] T055 [P] [US6] Write failing test `TestFallback_DelayBetweenAttempts` in `internal/router/fallback_test.go` — fallback attempts have > 0 delay
- [x] T056 [P] [US6] Write failing test `TestRouter_BackoffWithJitter` in `internal/router/router_test.go` — multiple retries have random jitter (not identical delays)
- [x] T057 [P] [US6] Write failing test `TestRouter_MaxBackoffCap` in `internal/router/router_test.go` — 10+ consecutive failures, backoff ≤ 30s
- [x] T057a [P] [US6] Write failing test `TestRouter_ExhaustsRetriesReturns429` in `internal/router/router_test.go` — all retries exhausted → returns 429 to client, does not loop infinitely (AS-6.4)
- [x] T058 [P] [US6] Write failing test `TestGeminiRoute_HasFullMiddlewareChain` in `internal/proxy/server_test.go` — /v1beta/models/* has Parallel + DynamicRate + CacheControl + Budget
- [x] T059 [P] [US6] Write failing test `TestAnthropicBatchRoute_HasFullMiddlewareChain` in `internal/proxy/server_test.go` — /anthropic/v1/* has full middleware chain
- [x] T060 [US6] Run `go test ./internal/router/... ./internal/proxy/... -run "TestRouter_Retry|TestRouter_Backoff|TestRouter_AllDeployments|TestRouter_Uses|TestRouter_Max|TestRouter_Exhausts|TestFallback_Delay|TestGeminiRoute_|TestAnthropicBatchRoute_" -v` — confirm all tests compile and FAIL

### Implementation for User Story 6

- [x] T061 [US6] Implement exponential backoff in `internal/router/router.go` — in Route() retry loop, add `time.Sleep(backoff)` with base=1s, factor=2, max=30s, jitter=0-25%; parse Retry-After header from upstream response; use RetryPolicy.RetryAfterSeconds as base when configured
- [x] T062 [US6] Add per-request context timeout in `internal/router/router.go` — use RetryPolicy.TimeoutSeconds to set context deadline
- [x] T063 [US6] Implement fallback delay in `internal/router/fallback.go` — add short delay (500ms) between fallback attempts in GeneralFallback() and ContentPolicyFallback()
- [x] T064 [US6] Run tests from T060 — confirm all PASS

**Checkpoint**: Router retries with backoff; Gemini/Batches routes fully protected by middleware.

---

## Phase 9: Edge Cases & Cross-Cutting

**Purpose**: Edge case tests and final validation

### Edge Case Tests (MANDATORY — Principle IV) 🔴

- [x] T065 [P] Write failing test `TestAuthMiddleware_ZeroRPMBlocksAll` in `internal/proxy/middleware/auth_test.go` — rpm_limit=0 → all blocked
- [x] T066 [P] Write failing test `TestBudgetMiddleware_SpendEqualsMax` in `internal/proxy/middleware/budget_test.go` — spend == max_budget → 429
- [x] T067 [P] Write failing test `TestDynamicRateLimit_RedisUnavailableFailOpen` in `internal/proxy/middleware/dynamic_ratelimit_test.go` — Redis down → request allowed
- [x] T068 [P] Write failing test `TestBudgetMiddleware_RedisUnavailableFailOpen` in `internal/proxy/middleware/budget_test.go` — Redis down → request allowed
- [x] T068a [P] Write failing test `TestMiddlewareChain_MultipleLimitsTrigger` in `internal/proxy/middleware/budget_test.go` — RPM exceeded + budget exceeded simultaneously → first middleware in chain (budget) returns 429 (edge case: multi-limit trigger order)
- [x] T068b [P] Write failing test `TestParallel_DecrementOnStreamEnd` in `internal/proxy/middleware/parallel_test.go` — streaming request increments counter on start, decrements on response writer Close; verify counter returns to 0 after stream ends
- [x] T069 Run edge case tests — confirm all compile and FAIL

### Implementation

- [x] T070 Handle edge case: 0 vs null in `internal/proxy/middleware/auth.go` — rpm_limit=0 (explicitly set) blocks all; null = no limit
- [x] T071 Handle edge case: spend == max_budget in `internal/proxy/middleware/budget.go` — use >= comparison (reject when equal)
- [x] T072 Verify fail-open behavior in `internal/proxy/middleware/dynamic_ratelimit.go` and `budget.go` — Redis errors allow request through (already implemented, confirm with tests)
- [x] T073 Run all edge case tests — confirm all PASS
- [x] T074 Run full test suite: `make check` — confirm zero regressions
- [x] T075 Run quickstart.md validation flow

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — BLOCKS all user stories
- **Phase 3-7 (US1-US5)**: All depend on Phase 2; US1-US3 (P1) before US4-US5 (P2)
- **Phase 8 (US6)**: Router backoff independent of Phase 2; middleware gap depends on Phase 2
- **Phase 9 (Edge Cases)**: Depends on all stories

### User Story Dependencies

- **US1 (RPM/TPM)**: Phase 2 → US1 (no other story deps)
- **US2 (Budget)**: Phase 2 → US2 (no other story deps)
- **US3 (Models)**: Phase 2 → US3 (no other story deps)
- **US4 (Parallel)**: Phase 2 → US4 (no other story deps)
- **US5 (Expiry)**: Phase 2 → US5 (no other story deps)
- **US6a (Backoff)**: Independent — can start anytime
- **US6b (Middleware gap)**: Phase 2 → US6b

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Implementation tasks in dependency order
- Story complete when all tests PASS

### Parallel Opportunities

- Phase 1: T001 + T003 can run in parallel (T002 depends on T001)
- Phase 2: T004, T005, T006 can run in parallel
- Phase 3-7: All [P] test tasks within a story can run in parallel
- US1, US2, US3 can run in parallel after Phase 2 (different files)
- US6a (router backoff) can run in parallel with everything

---

## Parallel Example: User Story 1

```bash
# Launch all US1 tests in parallel (different test files):
Task T007-T011: auth_test.go tests (parallel within same file)
Task T012-T014: dynamic_ratelimit_test.go tests (parallel within same file)

# After tests confirmed failing, implementation:
Task T016: auth.go context injection
Task T017: tracker.go TPM counter (parallel — different file)
```

---

## Implementation Strategy

### MVP First (Phase 1 + 2 + US1 only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T006)
3. Complete Phase 3: US1 RPM/TPM (T007-T018)
4. **STOP and VALIDATE**: RPM/TPM limits work end-to-end
5. Deploy — this alone stops the "hoh lobster" scenario

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. US1 (RPM/TPM) → **MVP** — rate limits work
3. US2 (Budget) → budget limits work
4. US3 (Models) → model restrictions work
5. US4 (Parallel) + US5 (Expiry) → P2 features
6. US6 (Backoff + Middleware gap) → system hardening
7. Edge Cases → final polish

### Parallel Team Strategy

With multiple developers after Phase 2:
- Developer A: US1 (RPM/TPM) + US2 (Budget) — same middleware focus
- Developer B: US3 (Models) — body buffering, different code path
- Developer C: US6 (Router backoff) — completely independent code path

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
