# Tasks: Sync Model Pricing Button

**Feature**: `003-sync-pricing-button`
**Scope**: P1 only
**Created**: 2026-02-25

---

## Phase 1: DB Schema & sqlc Queries

### T001 — Migration: up SQL

- **描述**: 建立 `011_model_pricing.up.sql`，含 `ModelPricing` table + indexes
- **檔案**: `internal/db/schema/011_model_pricing.up.sql`
- **依賴**: 無
- **驗收條件**:
  - Table schema 與 `data-model.md` 完全一致
  - 包含 `idx_model_pricing_provider` 和 `idx_model_pricing_synced_at` indexes
  - `CREATE TABLE IF NOT EXISTS` 語法正確
  - 可對空 DB 執行無報錯

### T002 — Migration: down SQL

- **描述**: 建立 `011_model_pricing.down.sql`，`DROP TABLE IF EXISTS "ModelPricing"`
- **檔案**: `internal/db/schema/011_model_pricing.down.sql`
- **依賴**: 無
- **驗收條件**:
  - 執行 down migration 後 `ModelPricing` table 不存在
  - 可重複執行（IF EXISTS）

### T003 — sqlc query file

- **描述**: 建立 `model_pricing.sql`，含 `UpsertModelPricing`、`ListModelPricing`、`DeleteAllModelPricing`
- **檔案**: `internal/db/queries/model_pricing.sql`
- **依賴**: T001
- **驗收條件**:
  - SQL 語法正確，與 plan §1.2 一致
  - `UpsertModelPricing` 使用 `ON CONFLICT (model_name) DO UPDATE`
  - `ListModelPricing` 回傳所有 rows，ORDER BY model_name
  - `DeleteAllModelPricing` 刪除所有 rows

### T004 — sqlc generate

- **描述**: 執行 `make generate`，確認 sqlc 生成的 Go 程式碼無錯誤
- **檔案**: `internal/db/*.go`（generated）
- **依賴**: T001, T003
- **驗收條件**:
  - `make generate` 成功無報錯
  - 生成的 Go code 包含 `UpsertModelPricing`、`ListModelPricing`、`DeleteAllModelPricing` 方法
  - 生成的 `upsertModelPricingSQL` 常量可供 Phase 3 使用
  - `go build ./...` 通過

---

## Phase 2: Calculator 改造

### T005 — Calculator struct: embedded map + 三層初始化

- **描述**: 將 Calculator 從兩層改為三層（embedded / models / overrides）。`Default()` 初始化時 parse `model_prices.json` → `embedded`，copy → `models`
- **檔案**: `internal/pricing/pricing.go`
- **依賴**: 無
- **驗收條件**:
  - Calculator struct 包含 `embedded`、`models`、`overrides` 三個 map
  - `Default()` 初始化後 `embedded` 與 `models` 內容相同
  - `embedded` 在初始化後不可變
  - 既有行為不變（所有現有 test 通過）

### T006 — ReloadFromDB 方法

- **描述**: 新增 `ReloadFromDB(entries []db.ModelPricing)` 方法，整個替換 `models` map
- **檔案**: `internal/pricing/pricing.go`
- **依賴**: T004, T005
- **驗收條件**:
  - `ReloadFromDB` 用 write lock 替換 `models` map
  - 呼叫後 `lookup` 能查到新的 DB 資料
  - `embedded` 不受影響

### T007 — lookup 三層 fallback

- **描述**: 修改 `lookup` 方法，依序查 overrides → models → embedded，每層嘗試 exact match + strip prefix
- **檔案**: `internal/pricing/pricing.go`
- **依賴**: T005
- **驗收條件**:
  - overrides 優先於 models，models 優先於 embedded
  - 每層都支援 exact match + strip prefix
  - 三層都沒有時回傳 nil

### T008 — ModelCostMap merge 三層

- **描述**: 修改 `ModelCostMap()` 回傳三層 merge 結果（embedded → models → overrides，後者覆蓋前者）
- **檔案**: `internal/pricing/pricing.go`
- **依賴**: T005
- **驗收條件**:
  - 回傳的 map 包含三層所有 model
  - 同名 model 以 overrides > models > embedded 優先序
  - 回傳的是 copy，修改不影響原始資料

### T009 — Calculator unit tests

- **描述**: 為 Phase 2 的改造寫 unit tests
- **檔案**: `internal/pricing/pricing_test.go`
- **依賴**: T005, T006, T007, T008
- **驗收條件**:
  - 測試三層 fallback 順序（overrides > models > embedded）
  - 測試 `ReloadFromDB` 替換 models 後 lookup 正確
  - 測試 `ModelCostMap` merge 邏輯
  - 測試 strip prefix fallback
  - 測試 concurrent read/write 安全（用 goroutine race test）
  - `go test -race ./internal/pricing/...` 通過

---

## Phase 3: Sync Service

### T010 — SyncFromUpstream 函數骨架

- **描述**: 建立 `sync.go`，實作 `SyncFromUpstream(ctx, pool, queries, calc, upstreamURL) (int, error)` 的完整流程：HTTP fetch → parse → validate → batch upsert → ReloadFromDB
- **檔案**: `internal/pricing/sync.go`
- **依賴**: T004, T006
- **驗收條件**:
  - 使用 dedicated `http.Client`（timeout 30s），不用 `http.DefaultClient`
  - Parse JSON 時跳過 `sample_spec`
  - 使用 `pgx.Batch` 進行 batch upsert（重用 sqlc 生成的 SQL 常量）
  - 整個 batch 在 transaction 內（all-or-nothing）
  - 成功後呼叫 `calc.ReloadFromDB`
  - 回傳 synced model count

### T011 — Upstream validation

- **描述**: 實作 `validateUpstreamData`，確認 model count ≥ 50（扣除 sample_spec）
- **檔案**: `internal/pricing/sync.go`
- **依賴**: T010
- **驗收條件**:
  - model count < 50 時回傳 error，不寫 DB
  - model count ≥ 50 時通過
  - error message 包含實際 count 與期望 count

### T012 — Sync service unit tests

- **描述**: 為 sync service 寫 unit tests（mock HTTP + mock DB）
- **檔案**: `internal/pricing/sync_test.go`
- **依賴**: T010, T011
- **驗收條件**:
  - 測試正常 sync 流程（mock HTTP server 回傳 valid JSON）
  - 測試 HTTP 失敗場景（timeout、4xx、5xx）
  - 測試 JSON parse 失敗
  - 測試 validation 失敗（model count < 50）
  - 測試 `sample_spec` 被正確跳過
  - 測試個別 model entry 欄位缺失時 skip + 繼續
  - `go test -race ./internal/pricing/...` 通過

---

## Phase 4: HTTP Handler + 路由

### T013 — UIHandler 新增欄位

- **描述**: `UIHandler` struct 新增 `Pool *pgxpool.Pool`、`Pricing *pricing.Calculator`、`syncPricingMu sync.Mutex`
- **檔案**: `internal/ui/handler.go`
- **依賴**: 無
- **驗收條件**:
  - 新增三個欄位，型別正確
  - 既有程式碼編譯通過（`go build ./...`）

### T014 — handleSyncPricing handler

- **描述**: 實作 `handleSyncPricing` method：TryLock concurrent guard、DB nil check、呼叫 `SyncFromUpstream`、回傳 HTMX toast
- **檔案**: `internal/ui/handler_models.go`
- **依賴**: T010, T013
- **驗收條件**:
  - 使用 `syncPricingMu.TryLock()` 防止 concurrent sync，失敗回傳 409
  - `DB == nil` 時回傳適當 error
  - 成功時回傳 success toast（含 model count）
  - 失敗時回傳 error toast（含 error message）
  - 支援 `PRICING_UPSTREAM_URL` 環境變數，有 default URL

### T015 — 路由註冊

- **描述**: 在 Models group 下新增 `POST /models/sync-pricing` 路由
- **檔案**: `internal/ui/routes.go`
- **依賴**: T014
- **驗收條件**:
  - `POST /ui/models/sync-pricing` 路由存在且指向 `handleSyncPricing`
  - 受既有 admin auth middleware 保護

### T016 — Handler unit tests

- **描述**: 為 handleSyncPricing 寫 unit tests
- **檔案**: `internal/ui/handler_models_test.go`
- **依賴**: T014, T015
- **驗收條件**:
  - 測試 concurrent sync 返回 409
  - 測試 DB nil 返回 error
  - 測試正常 sync 返回 success toast
  - 測試 sync 失敗返回 error toast
  - `go test ./internal/ui/...` 通過

---

## Phase 5: UI Component

### T017 — Sync Pricing button（templ）

- **描述**: 在 Models 頁面 header 加入 Sync Pricing button，含 `hx-post`、`hx-disabled-elt="this"`、loading spinner
- **檔案**: `internal/ui/pages/models.templ`
- **依賴**: T015
- **驗收條件**:
  - Button 使用 `hx-post="/ui/models/sync-pricing"`
  - `hx-disabled-elt="this"` 確保 request 期間 button disabled
  - 包含 loading spinner（`htmx-indicator`）
  - `DB == nil` 時 button 帶 `disabled` attribute + tooltip "Database required"

### T018 — templ generate + 編譯驗證

- **描述**: 執行 `templ generate`，確認生成的 Go 程式碼編譯通過
- **檔案**: `internal/ui/pages/models_templ.go`（generated）
- **依賴**: T017
- **驗收條件**:
  - `templ generate` 無報錯
  - `go build ./...` 通過

---

## Phase 6: Startup Flow

### T019 — Startup: Pricing 初始化 + DB reload

- **描述**: 在 `main.go` 初始化 UIHandler 時，Pricing 無條件設為 `pricing.Default()`；DB 可用時呼叫 `ListModelPricing` + `ReloadFromDB`
- **檔案**: `cmd/tianji/main.go`
- **依賴**: T006, T013
- **驗收條件**:
  - `Pricing` 永遠 non-nil（不管 DB 是否可用）
  - DB 可用且有資料時，startup 執行 `ReloadFromDB`
  - DB 可用但 `ListModelPricing` 失敗時，log warning 但不 crash
  - DB 不可用時，不嘗試 load，不報錯
  - `Pool` 欄位正確傳入 UIHandler

### T020 — Integration test: 完整 sync 流程

- **描述**: End-to-end integration test：啟動 → sync → 驗證 DB 寫入 → 驗證 in-memory reload → 驗證 API cost calculation 使用新價格
- **檔案**: `internal/pricing/sync_integration_test.go`
- **依賴**: T010, T006, T004
- **驗收條件**:
  - 使用 test DB（或 dockertest/testcontainers）
  - 測試：sync → DB 有資料 → Calculator lookup 回傳 DB 價格
  - 測試：sync → restart（重新 `ListModelPricing` + `ReloadFromDB`）→ 價格一致
  - 測試：sync 失敗 → DB 資料不變（transaction rollback）
  - 測試：DB 不可用 → embedded fallback 正常運作
  - `go test -race -tags integration ./internal/pricing/...` 通過

### T021 — Integration test: UI 端到端

- **描述**: HTTP-level integration test：POST `/ui/models/sync-pricing` → 驗證 response + DB state
- **檔案**: `internal/ui/handler_models_integration_test.go`
- **依賴**: T014, T015, T019, T020
- **驗收條件**:
  - 測試 POST sync-pricing 返回 200 + toast HTML
  - 測試 concurrent POST 返回 409
  - 測試 DB nil 情況
  - `go test -race -tags integration ./internal/ui/...` 通過

---

## 依賴圖

```
T001 ──┬── T003 ──┐
T002   │          ├── T004 ──┬── T006 ──┬── T010 ──┬── T011 ── T012
       │          │          │          │          ├── T014 ── T015 ── T016, T017 ── T018
       │          │          │          │          └── T020
       │          │          ├── T019   │
       │          │          │          └── T021
T005 ──┴── T007   │
       ├── T008   │
       └── T009   │
T013 ──────┘
```

## Summary

| Phase | Tasks | 說明 |
|-------|-------|------|
| 1 — DB Schema & sqlc | T001–T004 | Migration + queries + generate |
| 2 — Calculator 改造 | T005–T009 | 三層 lookup, ReloadFromDB, unit tests |
| 3 — Sync Service | T010–T012 | HTTP fetch, validation, batch upsert, unit tests |
| 4 — Handler + 路由 | T013–T016 | handleSyncPricing, concurrent guard, unit tests |
| 5 — UI | T017–T018 | templ button, generate |
| 6 — Startup + Integration | T019–T021 | Startup flow, integration tests |

**Total**: 21 tasks
