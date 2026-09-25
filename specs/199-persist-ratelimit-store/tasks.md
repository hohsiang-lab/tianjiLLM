# Tasks: Persist RateLimitStore to Database

**Input**: Design documents from `/specs/199-persist-ratelimit-store/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Failing tests are MANDATORY (Constitution Principle IV). Task 1 of every user story MUST be "Write failing tests" based on the plan's `## Failing Tests` section.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to

---

## Phase 1: Setup (DB Schema & Codegen)

**Purpose**: DB migration + sqlc codegen — 所有 user story 的基礎

- [x] T001 Create DB migration `internal/db/schema/015_oauth_token_ratelimit_state.up.sql` per contracts/sqlc-queries.md
- [x] T002 Create sqlc query file `internal/db/queries/oauth_token_ratelimit_state.sql` with UpsertOAuthTokenRateLimitState and GetRecentOAuthTokenRateLimitStates
- [x] T003 Run `make generate` to produce sqlc Go code and verify compilation

**Checkpoint**: DB schema ready, sqlc types generated

---

## Phase 2: Foundational (Dirty Tracking in RateLimitStore)

**Purpose**: In-memory dirty tracking — prerequisite for both flush and preload

**⚠️ CRITICAL**: Flush and preload depend on dirty tracking API

### Tests for Foundational (MANDATORY - Principle IV) 🔴

- [x] T004 [P] Write failing test `TestDirtyTracking_SetMarksDirty` in `internal/callback/ratelimit_store_test.go` — store.Set("k") → DirtyKeys() contains "k"
- [x] T005 [P] Write failing test `TestDirtyTracking_ClearDirty` in `internal/callback/ratelimit_store_test.go` — Set("k") → ClearDirty(["k"]) → DirtyKeys() empty
- [x] T006 [P] Write failing test `TestDirtyTracking_SnapshotAndClear` in `internal/callback/ratelimit_store_test.go` — Set 3 keys → SnapshotDirty() returns 3 entries → DirtyKeys() empty

### Implementation for Foundational

- [x] T007 Add dirty tracking to `InMemoryRateLimitStore` in `internal/callback/ratelimit_store.go` — add `dirty map[string]bool` + `dirtyMu sync.Mutex`, mark dirty in Set(), add `DirtyKeys()`, `SnapshotDirty()`, `ClearDirty()` methods
- [x] T008 Verify T004-T006 tests pass with `go test ./internal/callback/... -run TestDirtyTracking -v`

**Checkpoint**: Dirty tracking API working, tests green

---

## Phase 3: User Story 1 - Startup Preload (Priority: P1) 🎯 MVP

**Goal**: Pod 重啟後從 DB 載入 token utilization 數據，第一個請求就選對 token

**Independent Test**: 啟動服務 → store 有 DB 中的 fresh 數據 → lowestUtilizationSelect 選到正確 token

### Tests for User Story 1 (MANDATORY - Principle IV) 🔴

- [x] T009 [P] [US1] Write failing test `TestPreload_LoadsRecentStates` in `internal/callback/ratelimit_flusher_test.go` — DB 有 2 筆 fresh 記錄 → preload 後 store.Get() 回傳這 2 筆
- [x] T010 [P] [US1] Write failing test `TestPreload_IgnoresExpiredStates` in `internal/callback/ratelimit_flusher_test.go` — DB 有 1 筆 updated_at 2 小時前 → preload 後 store.Get() not found
- [x] T011 [P] [US1] Write failing test `TestPreload_EmptyDB` in `internal/callback/ratelimit_flusher_test.go` — DB 空 → preload 正常，store 為空
- [x] T012 [P] [US1] Write failing test `TestPreload_CorrectFieldMapping` in `internal/callback/ratelimit_flusher_test.go` — DB row 的 5h/7d/sonnet util 正確映射到 AnthropicOAuthRateLimitState
- [x] T013 [P] [US1] Write failing test `TestPreload_DBConnectionError` in `internal/callback/ratelimit_flusher_test.go` — DB 不可用 → preload 失敗 log warning → store 空 → 系統正常

### Implementation for User Story 1

- [x] T014 [US1] Implement `PreloadRateLimitStore(ctx, db, store, ttl)` function in `internal/callback/ratelimit_flusher.go` — SELECT recent rows, convert to AnthropicOAuthRateLimitState, Set into store
- [x] T015 [US1] Wire preload in `cmd/tianji/main.go` — call PreloadRateLimitStore after creating store, before starting server
- [x] T016 [US1] Verify T009-T013 tests pass with `go test ./internal/callback/... -run TestPreload -v`

**Checkpoint**: Pod 啟動時從 DB 載入 utilization 數據。可獨立驗證。

---

## Phase 4: User Story 2 - Periodic Flush (Priority: P1)

**Goal**: 每 30 秒 flush dirty keys 到 DB + graceful shutdown flush

**Independent Test**: Set token state → 等 30 秒 → DB 有記錄；cancel context → DB 有最新數據

### Tests for User Story 2 (MANDATORY - Principle IV) 🔴

- [x] T017 [P] [US2] Write failing test `TestFlush_WritesDirtyKeys` in `internal/callback/ratelimit_flusher_test.go` — Set 2 different keys → flush → DB 有 2 筆
- [x] T018 [P] [US2] Write failing test `TestFlush_OnlyDirtyKeys` in `internal/callback/ratelimit_flusher_test.go` — Set key A 兩次 → flush → DB 只 1 筆
- [x] T019 [P] [US2] Write failing test `TestFlush_429UpdatesDB` in `internal/callback/ratelimit_flusher_test.go` — Set 5h_util=1.00 → flush → DB 記錄 1.00
- [x] T020 [P] [US2] Write failing test `TestFlush_DBUnavailable` in `internal/callback/ratelimit_flusher_test.go` — DB 斷 → flush 失敗 → dirty keys 保留 → store 不受影響
- [x] T021 [P] [US2] Write failing test `TestFlush_GracefulShutdown` in `internal/callback/ratelimit_flusher_test.go` — cancel ctx → 最後一次 flush → DB 有最新數據
- [x] T022 [P] [US2] Write failing test `TestFlush_ClearsDirtyAfterSuccess` in `internal/callback/ratelimit_flusher_test.go` — flush 成功 → dirty set 空
- [x] T023 [P] [US2] Write failing test `TestFlush_RetainsDirtyOnFailure` in `internal/callback/ratelimit_flusher_test.go` — flush 失敗 → dirty set 保留
- [x] T024 [P] [US2] Write failing test `TestFlush_NoDirtyKeys` in `internal/callback/ratelimit_flusher_test.go` — 沒 dirty keys → flush 不執行 DB 操作

### Implementation for User Story 2

- [x] T025 [US2] Implement `RateLimitFlusher` struct in `internal/callback/ratelimit_flusher.go` — holds store + db refs, `flush()` method reads SnapshotDirty and batch upserts
- [x] T026 [US2] Implement `RateLimitFlusher.Start(ctx)` in `internal/callback/ratelimit_flusher.go` — ticker loop 30s + ctx.Done flush
- [x] T027 [US2] Wire flusher in `cmd/tianji/main.go` — create flusher, start goroutine, pass context for graceful shutdown
- [x] T028 [US2] Verify T017-T024 tests pass with `go test ./internal/callback/... -run TestFlush -v`

**Checkpoint**: Dirty keys flush 到 DB 每 30 秒 + shutdown flush。可獨立驗證。

---

## Phase 5: User Story 3 - Multi-Pod Sync (Priority: P2)

**Goal**: 新啟動的 pod 看到其他 pod 寫入的 token 狀態

**Independent Test**: 直接寫 DB 模擬 Pod A → 新 preload → store 有數據

### Tests for User Story 3 (MANDATORY - Principle IV) 🔴

- [x] T029 [P] [US3] Write failing test `TestMultiPod_PreloadSeesOtherPodWrites` in `internal/callback/ratelimit_flusher_test.go` — 直接 INSERT DB → preload → store.Get() 有數據
- [x] T030 [P] [US3] Write failing test `TestMultiPod_ConcurrentUpsert` in `internal/callback/ratelimit_flusher_test.go` — 2 goroutines 同時 upsert 同一 key → 無 error

### Implementation for User Story 3

- [x] T031 [US3] Verify T029-T030 tests pass — US1 的 preload 和 US2 的 flush 已實作，multi-pod 場景是它們的組合，不需要新代碼
- [x] T032 [US3] Run full integration test: `go test ./internal/callback/... -run "TestPreload|TestFlush|TestMultiPod" -v`

**Checkpoint**: Multi-pod 場景驗證完成

---

## Phase 6: Polish & Cross-Cutting

- [x] T033 Remove debug logging from `internal/callback/ratelimit_store.go` (line 116-122: DEBUG oauth-headers)
- [x] T034 Remove debug logging from `internal/proxy/handler/native_upstream.go` (lowestUtilizationSelect + selectUpstreamWithThrottle debug logs)
- [x] T035 Run `make lint` and fix any issues
- [x] T036 Run `make test` to verify no regressions
- [x] T037 Run quickstart.md validation commands

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 — dirty tracking needs sqlc types for test fixtures
- **Phase 3 (US1 Preload)**: Depends on Phase 2 — preload uses store.Set()
- **Phase 4 (US2 Flush)**: Depends on Phase 2 — flush uses SnapshotDirty()
- **Phase 5 (US3 Multi-Pod)**: Depends on Phase 3 + 4 — combination of preload + flush
- **Phase 6 (Polish)**: Depends on all phases

### User Story Dependencies

- **US1 (Preload)** and **US2 (Flush)** can run in parallel after Phase 2 — they touch different functions
- **US3 (Multi-Pod)** depends on both US1 and US2 — it's their combined behavior

### Parallel Opportunities

```
Phase 2: T004, T005, T006 可以並行（不同 test functions）
Phase 3: T009-T013 可以並行（不同 test functions）
Phase 4: T017-T024 可以並行（不同 test functions）
Phase 3 + Phase 4 可以並行（US1 preload 和 US2 flush 改不同函數）
```

---

## Implementation Strategy

### MVP (Phase 1-3: Setup + Dirty Tracking + Preload)

1. Phase 1: DB migration + sqlc codegen
2. Phase 2: Dirty tracking in store
3. Phase 3: Startup preload
4. **STOP**: Pod 重啟後第一個請求就能選對 token ✅

### Full (Phase 1-5)

1. MVP above
2. Phase 4: Periodic flush — 數據持續寫入 DB
3. Phase 5: Multi-pod — 驗證跨 pod 場景
4. Phase 6: 清理 debug log + lint

---

## Notes

- 總共 37 個 tasks
- 17 個 failing tests（Constitution Principle IV）
- US1: 8 tasks (5 tests + 3 impl)
- US2: 12 tasks (8 tests + 4 impl)
- US3: 4 tasks (2 tests + 2 verification)
- Tests 用 mock DB 或 in-process PostgreSQL (pgxpool test helper)
- 每個 checkpoint 後 commit
