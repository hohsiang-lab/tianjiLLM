# Implementation Plan: Sync Model Pricing Button (P1 MVP)

**Feature**: `003-sync-pricing-button`
**Scope**: P1 only — manual button sync, DB persist, hot-reload, fallback chain

---

## Phase 1: DB Schema & sqlc Queries

### 1.1 Migration File

**File**: `internal/db/schema/011_model_pricing.up.sql`

建立 `"ModelPricing"` table（詳見 `data-model.md`）。

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

### 1.3 Generate

```bash
make generate   # runs sqlc generate
```

---

## Phase 2: Calculator 改造

**File**: `internal/pricing/pricing.go`

### 2.1 新增 `embedded` map

將現有的三層改為：

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
            Provider:            e.Provider,
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
    // 1. overrides
    // 2. models (DB-synced)
    // 3. embedded (build-time fallback)
    // 每層都嘗試 exact match + strip prefix
}
```

---

## Phase 3: Sync Service Logic

**File**: `internal/pricing/sync.go`（新檔案）

### 3.1 `SyncFromUpstream` 函數

```go
func SyncFromUpstream(ctx context.Context, db *db.Queries, calc *Calculator, upstreamURL string) (int, error)
```

流程：
1. HTTP GET upstream URL（timeout 30s，dedicated client）
2. Parse JSON，跳過 `sample_spec`
3. 開 DB transaction（`db.Pool().Begin(ctx)`）
4. 迴圈 upsert 每個 model entry（用 `WithTx` pattern）
5. Commit transaction
6. 呼叫 `calc.ReloadFromDB(...)` 更新 in-memory
7. 回傳 synced model count

**Error handling**:
- HTTP 失敗 → return error，不動 DB
- JSON parse 失敗 → return error
- DB write 失敗 → rollback tx，return error
- 個別 model entry 欄位缺失 → skip + log warning，繼續

---

## Phase 4: HTTP Handler + 路由

### 4.1 Handler

**File**: `internal/ui/handler_models.go`（新增方法）

```go
func (h *UIHandler) handleSyncPricing(w http.ResponseWriter, r *http.Request) {
    if h.DB == nil {
        // render error toast: "Database not configured"
        return
    }
    upstreamURL := os.Getenv("PRICING_UPSTREAM_URL")
    if upstreamURL == "" {
        upstreamURL = defaultUpstreamURL
    }
    count, err := pricing.SyncFromUpstream(r.Context(), h.DB, h.Pricing, upstreamURL)
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
    DB        *db.Queries
    Config    *config.ProxyConfig
    Cache     cache.Cache
    MasterKey string
    Pricing   *pricing.Calculator  // 新增
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
    class="btn btn-secondary"
>
    <span id="sync-spinner" class="htmx-indicator spinner"></span>
    Sync Pricing
</button>
```

- 若 `DB == nil`，button 加 `disabled` + tooltip "Database required"
- P2 才做 loading state 的完整 UX（P1 用 HTMX indicator 即可）

---

## Phase 6: Startup Flow 改動

**File**: `cmd/tianji/main.go`

在 `UIHandler` 初始化後、`proxy.NewServer` 之前：

```go
// Load DB pricing into calculator on startup
if queries != nil {
    pricingCalc := pricing.Default()
    entries, err := queries.ListModelPricing(ctx)
    if err != nil {
        log.Printf("warn: failed to load DB pricing: %v", err)
    } else if len(entries) > 0 {
        pricingCalc.ReloadFromDB(entries)
        log.Printf("loaded %d model prices from database", len(entries))
    }
    uiHandler.Pricing = pricingCalc
}
```

---

## 檔案變更清單

| 檔案 | 動作 | 說明 |
|------|------|------|
| `internal/db/schema/011_model_pricing.up.sql` | **新增** | Migration |
| `internal/db/queries/model_pricing.sql` | **新增** | sqlc queries |
| `internal/db/*.go` (generated) | **生成** | `make generate` |
| `internal/pricing/pricing.go` | **修改** | 三層 lookup, `ReloadFromDB` |
| `internal/pricing/sync.go` | **新增** | `SyncFromUpstream` |
| `internal/ui/handler_models.go` | **修改** | `handleSyncPricing` |
| `internal/ui/handler.go` | **修改** | `UIHandler.Pricing` 欄位 |
| `internal/ui/routes.go` | **修改** | 新增 POST 路由 |
| `internal/ui/pages/models.templ` | **修改** | Sync button |
| `cmd/tianji/main.go` | **修改** | Startup DB pricing load |

## 實作順序建議

1. Phase 1（DB + sqlc）→ 2（Calculator）→ 3（Sync logic）→ 4（Handler）→ 5（UI）→ 6（Startup）
2. 每個 phase 可獨立 commit + test
3. Phase 1-3 無 UI 依賴，可先完成並驗證 logic
