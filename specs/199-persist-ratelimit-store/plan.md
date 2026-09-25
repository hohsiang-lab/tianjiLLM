# Implementation Plan: Persist RateLimitStore to Database

**Branch**: `199-persist-ratelimit-store` | **Date**: 2026-03-26 | **Spec**: [spec.md](spec.md)

## Summary

RateLimitStore 是純 in-memory，pod 重啟歸零。`lowestUtilizationSelect` 把 store miss 的 token 當成 idle（composite=0），導致已滿的 token 被優先選中。

修正：每 30 秒 periodic flush dirty keys 到 PostgreSQL（新表 `OAuthTokenRateLimitState`），啟動時從 DB 預載。In-memory store 仍然是 hot path 唯一讀取源。參考 Grafana Alerting 的 `persister_async.go` 模式。

## Technical Context

**Language/Version**: Go 1.26
**Primary Dependencies**: pgx/v5 (PostgreSQL), sqlc (query codegen)
**Storage**: PostgreSQL（新表 `OAuthTokenRateLimitState`）
**Testing**: `go test` + testify + in-memory DB mock
**Constraints**: DB 不在 hot path。寫入 async。DB 不可用時退化為 in-memory。

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | N/A | LiteLLM 不持久化 rate limit 到 DB，我們做得更好 |
| II. Feature Parity | N/A | 超出 LiteLLM 功能範圍 |
| III. Research Before Build | ✅ PASS | research.md 4 個決策，參考 Grafana/sing-box/Istio/Zitadel |
| IV. Failing-Tests-First | ✅ PASS | 17 個 tests 涵蓋全部 acceptance scenarios |
| V. Go Best Practices | ✅ PASS | ticker + ctx.Done pattern, dirty map + mutex, sqlc codegen |
| VI. No Stale Knowledge | ✅ PASS | 從 codebase 分析得出 |
| VII. sqlc-First | ✅ PASS | 新表用 sqlc migration + query codegen |

## Project Structure

```text
internal/db/schema/
└── 015_oauth_token_ratelimit_state.up.sql  # NEW — migration

internal/db/queries/
└── oauth_token_ratelimit_state.sql         # NEW — sqlc queries

internal/callback/
├── ratelimit_store.go                      # MODIFY — add dirty tracking
├── ratelimit_flusher.go                    # NEW — periodic flush + startup preload
└── ratelimit_flusher_test.go               # NEW — tests

cmd/tianji/
└── main.go                                 # MODIFY — wire flusher + preload
```

## Failing Tests

### User Story 1 — Startup Preload

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestPreload_LoadsRecentStates` | `ratelimit_flusher_test.go` | DB 有 2 筆 fresh 記錄 → 啟動後 store.Get() 回傳這 2 筆 | AS-1.1 |
| `TestPreload_IgnoresExpiredStates` | `ratelimit_flusher_test.go` | DB 有 1 筆 updated_at 2 小時前 → 啟動後 store.Get() 回傳 not found | AS-1.2 |
| `TestPreload_EmptyDB` | `ratelimit_flusher_test.go` | DB 沒有記錄 → 啟動正常，store 為空 | AS-1.3 |
| `TestPreload_CorrectFieldMapping` | `ratelimit_flusher_test.go` | DB row 的 5h/7d/sonnet utilization 正確映射到 AnthropicOAuthRateLimitState 的對應欄位 | AS-1.1 (field mapping) |

### User Story 2 — Periodic Flush

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestFlush_WritesDirtyKeys` | `ratelimit_flusher_test.go` | store.Set() 兩次不同 key → flush → DB 有 2 筆 upsert | AS-2.1 |
| `TestFlush_OnlyDirtyKeys` | `ratelimit_flusher_test.go` | store.Set() key A 兩次 → flush → DB 只有 1 筆（key A）| AS-2.1 (dedup) |
| `TestFlush_429UpdatesDB` | `ratelimit_flusher_test.go` | store.Set() 5h_util=1.00 → flush → DB 記錄 5h_util=1.00 | AS-2.2 |
| `TestFlush_DBUnavailable` | `ratelimit_flusher_test.go` | DB 連線斷開 → flush 失敗 → dirty keys 保留 → store 不受影響 | AS-2.3 |
| `TestFlush_GracefulShutdown` | `ratelimit_flusher_test.go` | cancel context → 最後一次 flush 執行 → DB 有最新數據 | AS-2.4 |
| `TestFlush_ClearsDirtyAfterSuccess` | `ratelimit_flusher_test.go` | flush 成功後 dirty set 為空 → 下次 flush 不寫入 | FR-009 |
| `TestFlush_RetainsDirtyOnFailure` | `ratelimit_flusher_test.go` | flush 失敗 → dirty set 保留 → 下次 flush 重試 | Edge: flush retry |

### User Story 3 — Multi-Pod

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestMultiPod_PreloadSeesOtherPodWrites` | `ratelimit_flusher_test.go` | 直接寫 DB 模擬 Pod A → 新 flusher preload → store 有 Pod A 的數據 | AS-3.1 |
| `TestMultiPod_ConcurrentUpsert` | `ratelimit_flusher_test.go` | 兩個 goroutine 同時 upsert 同一 key → 無 error，last-write-wins | AS-3.2 |

### Dirty Tracking Tests

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestDirtyTracking_SetMarksDirty` | `ratelimit_store_test.go` | store.Set("k") → store.DirtyKeys() 包含 "k" | FR-009 |
| `TestDirtyTracking_ClearDirty` | `ratelimit_store_test.go` | store.Set("k") → store.ClearDirty(["k"]) → DirtyKeys() 為空 | FR-009 |
| `TestDirtyTracking_SnapshotAndClear` | `ratelimit_store_test.go` | Set 3 keys → SnapshotDirty() → 回傳 3 個 key + state → DirtyKeys() 為空 | FR-009 |

### Edge Cases

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestFlush_NoDirtyKeys` | `ratelimit_flusher_test.go` | 沒有 dirty keys → flush 不執行 DB 操作 | EC: no-op flush |
| `TestPreload_DBConnectionError` | `ratelimit_flusher_test.go` | DB 不可用 → preload 失敗 log warning → store 為空 → 系統正常啟動 | EC: DB migration fail |

### Verification

```bash
make generate  # sqlc codegen
go test ./internal/callback/... -run "TestPreload|TestFlush|TestMultiPod|TestDirtyTracking" -v
```

## Key Design Decisions

### 1. Dirty tracking 在 InMemoryRateLimitStore 裡

不創建新的 wrapper struct。直接在 `InMemoryRateLimitStore` 加 `dirty map[string]bool` + `dirtyMu sync.Mutex`。Set() 時標記 dirty。新增 `SnapshotDirty() map[string]AnthropicOAuthRateLimitState` 方法：讀取 dirty keys 的 state，清空 dirty set，返回 snapshot。

### 2. Flusher 是獨立的 goroutine 組件

`RateLimitFlusher` 不嵌入 store，而是一個獨立的組件，由 main.go wire。它持有 store 和 db 的引用。這遵循依賴注入原則，方便測試。

### 3. 啟動 preload 在 main.go 裡

不在 flusher 的 Start() 裡 preload。在 main.go 裡顯式呼叫 `PreloadRateLimitStore(ctx, db, store, ttl)`。這樣 preload 失敗不會阻止 flusher 啟動。

### 4. 只持久化路由決策需要的欄位

DB 表只存 unified 系列欄位（5h/7d/sonnet utilization + status + reset）+ org_id。不存 legacy headers（RequestsLimit 等）和 UI-only 欄位（RepresentativeClaim 等）。減少表寬度和寫入量。

## Complexity Tracking

沒有 constitution violation。所有改動遵循現有模式：
- sqlc migration + query codegen（跟 OAuthTokenMetadata 同模式）
- Ticker + ctx.Done goroutine（跟現有 Prune goroutine 同模式）
- Dirty tracking 用 sync.Mutex + map（標準 Go 模式）
