# Implementation Plan: Sync Model Pricing Button (P1 MVP)

**Feature**: `003-sync-pricing-button`
**Scope**: P1 only — manual button sync, DB persist, hot-reload, fallback chain
**Reviewed by**: 魯班 (Eng), 要你命3000 (調度)
**Review date**: 2026-02-25

---

## Phase 1: DB Schema & sqlc Queries

### 1.1 Migration Files

**File**: `internal/db/schema/011_model_pricing.up.sql`

建立 `"ModelPricing"` table（詳見 `data-model.md`）。

**File**: `internal/db/schema/011_model_pricing.down.sql`

```sql
DROP TABLE IF EXISTS "ModelPricing";
```

### 1.2 sqlc Query File

**File**: `internal/db/queries/model_pricing.sql`

```sql
-- name: UpsertModelPricing :exec
INSERT INTO "ModelPricing" (
    model_name, input_cost_per_token, output_cost_per_token,
    max_input_tokens, max_output_tokens, max_tokens,
    mode, provider, source_url, synced_at
) VALUES (
    @model_name, @input_cost_per_token, @output_cost_per_token,
    @max_input_tokens, @max_output_tokens, @max_tokens,
    @mode, @provider, @source_url, NOW()
)
ON CONFLICT (model_name) DO UPDATE SET
    input_cost_per_token  = EXCLUDED.input_cost_per_token,
    output_cost_per_token = EXCLUDED.output_cost_per_token,
    max_input_tokens      = EXCLUDED.max_input_tokens,
    max_output_tokens     = EXCLUDED.max_output_tokens,
    max_tokens            = EXCLUDED.max_tokens,
    mode                  = EXCLUDED.mode,
    provider              = EXCLUDED.provider,
    source_url            = EXCLUDED.source_url,
    synced_at             = NOW(),
    updated_at            = NOW();

-- name: ListModelPricing :many
SELECT * FROM "ModelPricing"
ORDER BY model_name;

-- name: DeleteAllModelPricing :exec
DELETE FROM "ModelPricing";
```

> **Note**: `UpsertModelPricing` 的 SQL text 會被 Phase 3 的 `pgx.Batch` 重用。
> sqlc 生成的 `upsertModelPricingSQL` 常量可直接引用。

### 1.3 Generate

```bash
make generate   # runs sqlc generate
```

---

## Phase 2: Calculator 改造

**File**: `internal/pricing/pricing.go`

### 2.1 新增 `embedded` map

將現有的兩層改為三層：

```go
type Calculator struct {
    mu        sync.RWMutex
    embedded  map[string]ModelInfo  // build-time embedded (immutable after init)
    models    map[string]ModelInfo  // DB-synced data (replaceable)
    overrides map[string]ModelInfo  // runtime custom overrides
}
```

`Default()` 初始化時：
- Parse `model_prices.json` → 寫入 `embedded`
- Copy `embedded` → `models`（初始狀態等同現在）

### 2.2 新增 `ReloadFromDB` 方法

```go
func (c *Calculator) ReloadFromDB(entries []DBModelPricing) {
    newModels := make(map[string]ModelInfo, len(entries))
    for _, e := range entries {
        newModels[e.ModelName] = ModelInfo{
            InputCostPerToken:  e.InputCostPerToken,
            OutputCostPerToken: e.OutputCostPerToken,
            MaxInputTokens:     e.MaxInputTokens,
            MaxOutputTokens:    e.MaxOutputTokens,
            MaxTokens:          e.MaxTokens,
            Mode:               e.Mode,
            Provider:           e.Provider,
        }
    }
    c.mu.Lock()
    c.models = newModels
    c.mu.Unlock()
}
```

`DBModelPricing` 是一個簡單的 struct 或直接用 sqlc 生成的 `db.ModelPricing`。

### 2.3 修改 `lookup` — 三層 fallback

```go
func (c *Calculator) lookup(model string) *ModelInfo {
    c.mu.RLock()
    defer c.mu.RUnlock()
    // 1. overrides (runtime custom pricing)
    // 2. models (DB-synced)
    // 3. embedded (build-time fallback)
    // 每層都嘗試 exact match + strip prefix
}
```

### 2.4 修改 `ModelCostMap()` — merge 三層

現有 `ModelCostMap()` 只回傳 `c.models`。改造後應 merge 三層以回傳完整資料：

```go
func (c *Calculator) ModelCostMap() map[string]ModelInfo {
    c.mu.RLock()
    defer c.mu.RUnlock()
    merged := make(map[string]ModelInfo, len(c.embedded)+len(c.models)+len(c.overrides))
    // Layer 1: embedded (lowest priority)
    for k, v := range c.embedded {
        merged[k] = v
    }
    // Layer 2: DB-synced (overrides embedded)
    for k, v := range c.models {
        merged[k] = v
    }
    // Layer 3: runtime overrides (highest priority)
    for k, v := range c.overrides {
        merged[k] = v
    }
    return merged
}
```

> **隱性依賴注意**: `spend.NewCalculator("")` 內部使用 `pricing.Default()` singleton。
> 由於 `ReloadFromDB` 修改的是同一個 singleton instance 的 map，spend calculator
> 會自動使用新資料，無需額外 wiring。

---

## Phase 3: Sync Service Logic

**File**: `internal/pricing/sync.go`（新檔案）

### 3.1 `SyncFromUpstream` 函數

```go
func SyncFromUpstream(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries, calc *Calculator, upstreamURL string) (int, error)
```

流程：
1. HTTP GET upstream URL（timeout 30s，dedicated `http.Client`）
2. Parse JSON，跳過 `sample_spec`
3. **Validate**: 確認 model count ≥ 50（防止 upstream corrupted/empty）
4. 開 DB transaction（`pool.Begin(ctx)`）
5. **Batch upsert**（用 `pgx.Batch`，見 §3.2）
6. Commit transaction
7. 呼叫 `calc.ReloadFromDB(...)` 更新 in-memory
8. 回傳 synced model count

**Error handling**:
- HTTP 失敗 → return error，不動 DB
- JSON parse 失敗 → return error
- **Validation 失敗**（model count < 50）→ return error，不動 DB
- DB write 失敗 → rollback tx，return error
- 個別 model entry 欄位缺失 → skip + log warning，繼續

### 3.2 Batch Write（`pgx.Batch`）

> **為什麼不用迴圈 upsert**：2000+ models × 1 round-trip/query = 2-20 秒。
> `pgx.Batch` 把所有 upsert 打包成**一次** network round-trip，大幅提升性能。

```go
batch := &pgx.Batch{}
for name, info := range parsed {
    if name == "sample_spec" {
        continue
    }
    batch.Queue(upsertModelPricingSQL,
        name, info.InputCostPerToken, info.OutputCostPerToken,
        info.MaxInputTokens, info.MaxOutputTokens, info.MaxTokens,
        info.Mode, info.Provider, upstreamURL,
    )
}
br := tx.SendBatch(ctx, batch)
defer br.Close()
for i := 0; i < batch.Len(); i++ {
    if _, err := br.Exec(); err != nil {
        return 0, fmt.Errorf("batch upsert item %d: %w", i, err)
    }
}
```

- SQL text 重用 sqlc 生成的 `upsertModelPricingSQL` 常量
- 整個 batch 在同一個 transaction 內（all-or-nothing）
- 預估 2000 models 耗時 < 500ms（一次 round-trip）

### 3.3 Upstream Validation

參考 LiteLLM 的 `GetModelCostMap.validate_model_cost_map`：

```go
const minModelCount = 50

func validateUpstreamData(parsed map[string]json.RawMessage) error {
    // 扣除 sample_spec
    count := len(parsed)
    if _, ok := parsed["sample_spec"]; ok {
        count--
    }
    if count < minModelCount {
        return fmt.Errorf("upstream returned only %d models, expected at least %d (possible corruption)", count, minModelCount)
    }
    return nil
}
```

---

## Phase 4: HTTP Handler + 路由

### 4.1 Handler

**File**: `internal/ui/handler_models.go`（新增方法）

```go
func (h *UIHandler) handleSyncPricing(w http.ResponseWriter, r *http.Request) {
    // Concurrent sync protection
    if !h.syncPricingMu.TryLock() {
        // render error toast: "Sync already in progress"
        w.WriteHeader(http.StatusConflict)
        return
    }
    defer h.syncPricingMu.Unlock()

    if h.DB == nil {
        // render error toast: "Database not configured"
        return
    }
    upstreamURL := os.Getenv("PRICING_UPSTREAM_URL")
    if upstreamURL == "" {
        upstreamURL = defaultUpstreamURL
    }
    count, err := pricing.SyncFromUpstream(r.Context(), h.Pool, h.DB, h.Pricing, upstreamURL)
    if err != nil {
        // render error toast
        return
    }
    // render success toast: "Synced {count} models"
}
```

### 4.2 UIHandler 擴充

`UIHandler` 新增欄位：

```go
type UIHandler struct {
    DB             *db.Queries
    Pool           *pgxpool.Pool         // 新增：for batch operations
    Config         *config.ProxyConfig
    Cache          cache.Cache
    MasterKey      string
    Pricing        *pricing.Calculator   // 新增：always non-nil (singleton)
    syncPricingMu  sync.Mutex            // 新增：prevent concurrent sync
}
```

### 4.3 路由註冊

**File**: `internal/ui/routes.go`

在 Models group 下新增：

```go
r.Post("/models/sync-pricing", h.handleSyncPricing)
```

---

## Phase 5: UI Component

### 5.1 Sync Pricing Button

**File**: `internal/ui/pages/models.templ`（修改）

在 Models 頁面的 header 區域加入 button：

```html
<button
    hx-post="/ui/models/sync-pricing"
    hx-target="#toast-container"
    hx-swap="innerHTML"
    hx-indicator="#sync-spinner"
    hx-disabled-elt="this"
    class="btn btn-secondary"
>
    <span id="sync-spinner" class="htmx-indicator spinner"></span>
    Sync Pricing
</button>
```

- `hx-disabled-elt="this"` — HTMX 原生支援，request 期間自動 disable button
- 若 `DB == nil`，button 加 `disabled` + tooltip "Database required"

---

## Phase 6: Startup Flow 改動

**File**: `cmd/tianji/main.go`

在 `UIHandler` 初始化時：

```go
// Pricing calculator — always set (singleton), regardless of DB availability
pricingCalc := pricing.Default()

// Load DB pricing into calculator on startup (if DB available)
if queries != nil {
    entries, err := queries.ListModelPricing(ctx)
    if err != nil {
        log.Printf("warn: failed to load DB pricing: %v", err)
    } else if len(entries) > 0 {
        pricingCalc.ReloadFromDB(entries)
        log.Printf("loaded %d model prices from database", len(entries))
    }
}

uiHandler := &UIHandler{
    DB:      queries,
    Pool:    pool,
    Config:  cfg,
    Cache:   cache,
    MasterKey: masterKey,
    Pricing: pricingCalc,  // always non-nil
}
```

> **注意**: `Pricing` 無條件設為 `pricing.Default()`，不管 DB 是否可用。
> 這樣 handler 永遠有 non-nil Pricing，消除 nil pointer 風險。
> DB 可用時額外執行 `ReloadFromDB` 載入 DB 資料覆蓋 embedded。

---

## 檔案變更清單

| 檔案 | 動作 | 說明 |
|------|------|------|
| `internal/db/schema/011_model_pricing.up.sql` | **新增** | Migration (up) |
| `internal/db/schema/011_model_pricing.down.sql` | **新增** | Migration (down) |
| `internal/db/queries/model_pricing.sql` | **新增** | sqlc queries |
| `internal/db/*.go` (generated) | **生成** | `make generate` |
| `internal/pricing/pricing.go` | **修改** | 三層 lookup, `ReloadFromDB`, `ModelCostMap()` merge |
| `internal/pricing/sync.go` | **新增** | `SyncFromUpstream` + validation + `pgx.Batch` |
| `internal/ui/handler_models.go` | **修改** | `handleSyncPricing` + concurrent guard |
| `internal/ui/handler.go` | **修改** | `UIHandler` 新增 `Pool`, `Pricing`, `syncPricingMu` |
| `internal/ui/routes.go` | **修改** | 新增 POST 路由 |
| `internal/ui/pages/models.templ` | **修改** | Sync button |
| `cmd/tianji/main.go` | **修改** | Startup DB pricing load, `Pricing` always non-nil |

## Review 改善記錄

以下改動基於魯班工程 review（2026-02-25）：

| # | 問題 | 改善 | 影響 |
|---|------|------|------|
| 1 | 迴圈 upsert 2000+ models 太慢 | 改用 `pgx.Batch` 打包一次 round-trip | Phase 3 §3.2 |
| 2 | `uiHandler.Pricing` 可能 nil | 無條件設為 `pricing.Default()` | Phase 6 |
| 3 | 缺 upstream validation | 加 model count ≥ 50 檢查 | Phase 3 §3.3 |
| 4 | 缺 down migration | 加 `011_model_pricing.down.sql` | Phase 1 §1.1 |
| 5 | 缺 concurrent sync protection | `syncPricingMu.TryLock()` + 409 | Phase 4 §4.1 |
| 6 | `ModelCostMap()` 只回傳一層 | merge 三層 (embedded → models → overrides) | Phase 2 §2.4 |

## 實作順序建議

1. Phase 1（DB + sqlc）→ 2（Calculator）→ 3（Sync logic）→ 4（Handler）→ 5（UI）→ 6（Startup）
2. 每個 phase 可獨立 commit + test
3. Phase 1-3 無 UI 依賴，可先完成並驗證 logic
