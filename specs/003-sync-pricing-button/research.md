# Research: Sync Model Pricing Button

**Date**: 2026-02-25
**Feature**: `003-sync-pricing-button`

## 1. 現有架構分析

### 1.1 Pricing Calculator (`internal/pricing/pricing.go`)

- **Singleton pattern**: `Default()` 用 `sync.Once` 初始化
- **資料來源**: `//go:embed model_prices.json` 編譯時嵌入
- **結構**: `Calculator` 有兩層 map：
  - `models map[string]ModelInfo` — 嵌入的靜態資料
  - `overrides map[string]ModelInfo` — 執行時覆蓋（目前只有 `SetCustomPricing` 使用）
- **Lookup 順序**: overrides → models → strip provider prefix 再查
- **ModelInfo 欄位**: `InputCostPerToken`, `OutputCostPerToken`, `MaxInputTokens`, `MaxOutputTokens`, `MaxTokens`, `Mode`, `Provider`
- **鎖**: `sync.RWMutex` 已存在，支援 concurrent read

**關鍵洞察**: 現有的 `overrides` 機制就是我們的切入點。DB-synced 資料可以載入到 `models` map，而 `overrides` 保留給 custom pricing。或者我們加第三層 `dbPricing`，lookup chain: overrides → dbPricing → models(embedded)。

**決策**: 新增 `ReloadFromDB(entries []ModelPricing)` 方法，直接替換 `models` map 內容。嵌入資料作為 fallback 保留在新的 `embedded` map 中。Lookup chain: overrides → models(DB) → embedded。

### 1.2 DB Schema 慣例 (`internal/db/schema/`)

- 使用 PostgreSQL，表名用 `PascalCase` + 雙引號（`"BudgetTable"`, `"VerificationToken"`, `"ProxyModelTable"`）
- Migration 檔命名: `NNN_description.up.sql`，目前到 `010`
- 標準欄位: `created_at`, `created_by`, `updated_at`, `updated_by` (全部 `TIMESTAMPTZ`)
- Primary key 多為 `TEXT`

### 1.3 sqlc Query 慣例 (`internal/db/queries/`)

- 一個 `.sql` 檔對應一個 domain（`proxy_model.sql`, `budget.sql` 等）
- 註解格式: `-- name: MethodName :one/:many/:exec`
- 使用 `RETURNING *` 回傳完整 row
- `sqlc.yaml`: engine=postgresql, package=db, out=internal/db, sql_package=pgx/v5

### 1.4 UI 架構 (`internal/ui/`)

- **Router**: chi v5，所有 UI 路由在 `RegisterRoutes()` 中註冊
- **驗證**: `sessionAuth` middleware，保護所有管理頁面
- **Toast 機制**: `toast.VariantSuccess` / `toast.VariantError`，透過 templ component render
  - Pattern: handler render 一個 `*WithToast()` templ component
- **HTMX**: 已廣泛使用，button POST → server render partial → swap
- **Handler 結構**: `UIHandler` struct 持有 `DB *db.Queries`, `Config`, `Cache`, `MasterKey`

### 1.5 啟動流程 (`cmd/tianji/main.go`)

順序：
1. Load config
2. Init DB pool + migrations
3. Init cache
4. Init callbacks + spend tracker
5. Init router/guardrails/policy
6. Init `UIHandler{DB, Config, Cache, MasterKey}`
7. Create `proxy.NewServer` → start HTTP

**切入點**: 在 step 6 之後、step 7 之前，如果 DB 可用，load DB pricing into Calculator。

### 1.6 Spend Calculator (`internal/spend/`)

注意：`internal/spend/` 有自己的 `Calculator`（`spend.NewCalculator("")`），和 `internal/pricing/` 的 `Calculator` 是**不同**的。需要確認兩者的關係。

經查，`spend.Calculator` 內部使用 `pricing.Default()` 來取得價格。所以只要更新 `pricing.Calculator` 的資料，spend tracking 也會自動使用新價格。

## 2. Upstream JSON 格式

**URL**: `https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json`

格式：
```json
{
  "sample_spec": { ... },
  "gpt-4": {
    "max_tokens": 4096,
    "max_input_tokens": 8192,
    "max_output_tokens": 4096,
    "input_cost_per_token": 0.00003,
    "output_cost_per_token": 0.00006,
    "litellm_provider": "openai",
    "mode": "chat",
    ...extra fields we ignore...
  }
}
```

- Top-level key `sample_spec` 需跳過（現有程式碼已處理）
- 每個 model entry 的欄位是 `ModelInfo` struct 的超集
- JSON 非常大（~2000+ models），需要 transaction batch write

## 3. 技術決策

| 決策 | 選擇 | 理由 |
|------|------|------|
| DB table 命名 | `"ModelPricing"` | 遵循現有 PascalCase 慣例 |
| PK 設計 | `model_name TEXT PRIMARY KEY` | 一個 model name 對應一筆價格，upsert 天然 idempotent |
| Upsert 策略 | `ON CONFLICT (model_name) DO UPDATE` | Spec 明確要求 upsert，不刪除舊資料 |
| Transaction | 整個 sync 包在一個 tx 裡 | Spec 要求 all-or-nothing |
| Batch write | 用 sqlc 的 `:copyfrom` 或單一大 upsert | 需測試 sqlc 是否支援 batch upsert；fallback 用迴圈 |
| Calculator 改造 | 三層 lookup: overrides → dbModels → embedded | 保持 backward compat，embedded 永遠是 fallback |
| Handler 位置 | `internal/ui/handler_models.go` 新增方法 | 與 Models page 同檔，符合現有 pattern |
| 路由 | `POST /ui/models/sync-pricing` | 放在 Models group 下，sessionAuth 保護 |
| Pricing Calculator 注入 | `UIHandler` 新增 `Pricing *pricing.Calculator` 欄位 | handler 需要呼叫 `ReloadFromDB` |
