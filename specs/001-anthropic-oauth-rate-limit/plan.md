# Plan: HO-74 — Usage 頁顯示 Anthropic Rate Limit 即時狀態

**Spec**: specs/001-anthropic-oauth-rate-limit/spec.md  
**Linear Issue**: HO-74  
**Branch**: 001-anthropic-oauth-rate-limit

---

## 技術方案總覽

### 架構設計

```
Anthropic Response Headers
         │
         ▼
native_format.go (FR-007)
  ParseAnthropicOAuthRateLimitHeaders(headers, tokenKey)
         │
         ▼
InMemoryRateLimitStore.Set(key, state)   ← per-token, TTL 5min
         │
    ┌────┴────┐
    │         │
    ▼         ▼
Discord    GET /ui/api/rate-limit-state (FR-008)
Alerter         │
(不動)          ▼
           Usage 頁面 RateLimitWidget (FR-011)
           └─ 30s JS polling (FR-013)
```

### 核心原則
- **不改動** `AnthropicRateLimitState` 與 `DiscordRateLimitAlerter`（backward compat）
- **新建** `AnthropicOAuthRateLimitState` 獨立 struct，避免耦合
- **in-memory store** 以 interface 抽象，方便未來替換 Redis
- **tokenKey** 生成邏輯集中於 proxy handler，不散落各處

---

## Phase 規劃

### Phase 1：OAuth Rate Limit State Layer（後端 core）
**目標**：建立 store + parser，不觸碰任何 UI 或 handler 路由

**檔案異動**：
- **新建** `internal/callback/oauth_ratelimit.go`
  - `AnthropicOAuthRateLimitState` struct（TokenKey, UpdatedAt + 所有 header 欄位）
  - `RateLimitCacheKey(tokenKey string) string`
  - `ParseAnthropicOAuthRateLimitHeaders(h http.Header, tokenKey string) AnthropicOAuthRateLimitState`
  - `RateLimitStore` interface（Set / Get / GetAll / Prune）
  - `InMemoryRateLimitStore` struct（sync.RWMutex + map）

- **新建** `internal/callback/oauth_ratelimit_test.go`
  - 測試 ParseAnthropicOAuthRateLimitHeaders
  - 測試 InMemoryRateLimitStore Set/GetAll/Prune

**驗收**：`go test ./internal/callback/...` pass

---

### Phase 2：Proxy Handler 整合
**目標**：native_format.go 在 Anthropic response 後將 rate limit state 存入 store

**檔案異動**：
- **修改** `internal/proxy/handler/native_format.go`
  - 提取 tokenKey：若 `anthropic.IsOAuthToken(apiKey)` → `sha256(apiKey)[:12]`；否則 `"default"`
  - 呼叫 `callback.ParseAnthropicOAuthRateLimitHeaders(resp.Header, tokenKey)`
  - 呼叫 `h.RateLimitStore.Set(cacheKey, state)`（nil guard）
  - 保留現有 `h.DiscordAlerter.CheckAndAlert(state)` 不動

- **修改** `internal/proxy/handler/handler.go`（或 server init struct）
  - `Handler` struct 加入 `RateLimitStore callback.RateLimitStore`

- **修改** `internal/proxy/server.go`（或 main.go）
  - 建立 `InMemoryRateLimitStore`
  - 啟動 Prune goroutine（`time.Ticker` 每 1 分鐘）
  - 注入至 proxy handler + UI handler

**驗收**：手動測試 Anthropic 請求後，store 有資料

---

### Phase 3：UI API Endpoint
**目標**：`GET /ui/api/rate-limit-state` 回傳 JSON

**檔案異動**：
- **新建** `internal/ui/handler_ratelimit.go`
  - `handleRateLimitState(w, r)` handler
  - 呼叫 `h.RateLimitStore.GetAll()`，JSON encode 回傳
  - 空 store 時回傳 `[]`

- **修改** `internal/ui/routes.go`
  - 加入 `r.Get("/ui/api/rate-limit-state", h.handleRateLimitState)`

- **修改** UI `Handler` struct（`internal/ui/handler.go` 或類似）
  - 加入 `RateLimitStore callback.RateLimitStore`

**驗收**：`curl /ui/api/rate-limit-state` 回傳 JSON array

---

### Phase 4：Usage 頁面 Widget
**目標**：Usage 頁顯示 rate limit widget，30 秒 polling 更新

**檔案異動**：
- **修改** `internal/ui/pages/usage.templ`
  - 新增 `RateLimitWidgetData` struct（`[]AnthropicOAuthRateLimitState`）
  - 新增 `rateLimitWidget` templ component：
    - 無資料時：不顯示（或顯示「無 Anthropic 請求」）
    - 單 token：不顯示 token key header（FR-014）
    - 多 token：每 card 顯示 TokenKey（前 12 字元）
    - 每張 card：Input/Output/Requests limit / remaining / reset
    - UpdatedAt：「X 秒前更新」
    - -1 sentinel → 顯示 N/A

- **修改** `internal/ui/handler_usage.go`
  - `handleUsage` 加入 `h.RateLimitStore.GetAll()` 取資料，傳入 template

- **修改** `internal/ui/pages/usage.templ`（或 JS asset）
  - 加入 30 秒 polling：`setInterval(() => fetch('/ui/api/rate-limit-state').then(...), 30000)`
  - 收到 response 後更新 widget DOM

- 跑 `templ generate`（**必要**，templ 需 codegen）

**驗收**：
- Usage 頁面顯示 widget
- 30 秒後自動更新（可用 DevTools 確認）
- -1 欄位顯示 N/A

---

## 受影響檔案清單

| 檔案 | 異動類型 | Phase |
|------|---------|-------|
| `internal/callback/oauth_ratelimit.go` | **新建** | 1 |
| `internal/callback/oauth_ratelimit_test.go` | **新建** | 1 |
| `internal/proxy/handler/native_format.go` | 修改 | 2 |
| `internal/proxy/handler/handler.go`（或 server struct） | 修改 | 2 |
| `internal/proxy/server.go`（或 main.go） | 修改 | 2 |
| `internal/ui/handler_ratelimit.go` | **新建** | 3 |
| `internal/ui/routes.go` | 修改 | 3 |
| `internal/ui/handler.go`（或 UI Handler struct） | 修改 | 3 |
| `internal/ui/pages/usage.templ` | 修改 | 4 |
| `internal/ui/handler_usage.go` | 修改 | 4 |

---

## 注意事項

### ⚠️ API Key Backward Compat（C-01, FR-016）
- `AnthropicRateLimitState` struct **不動**
- `ParseAnthropicRateLimitHeaders` **不動**
- `DiscordRateLimitAlerter.CheckAndAlert` **不動**
- 非 OAuth token（API key）的 rate limit state 以 `"default"` 為 key 存入，UI 顯示「Default API Key」
- `native_format.go` 的 nil guard 必須保留（`h.RateLimitStore != nil`）

### ⚠️ templ codegen
- 修改 `.templ` 後**必須**跑 `templ generate`，否則 Go build 會失敗
- CI 應該有 `templ generate` step，確認不要漏掉

### ⚠️ tokenKey sha256
- 不能把完整 token 存進 key 或 log
- 只用 `sha256(token)[:12]` 作為識別（足夠 unique，不洩漏 secret）
- 若未來有 token alias 系統，可替換此邏輯（interface 已抽象）

### ⚠️ 並行安全
- `InMemoryRateLimitStore` 必須 `sync.RWMutex`
- `Set` 用 write lock，`Get` / `GetAll` 用 read lock

### ⚠️ MVP 邊界
- **不做** Redis（TTL in-memory 即可）
- **不做** WebSocket
- **不做** rate limit 歷史趨勢

---

## FR → Phase 對照

| FR | 說明 | Phase |
|----|------|-------|
| FR-001 | AnthropicOAuthRateLimitState struct | 1 |
| FR-002 | RateLimitCacheKey | 1 |
| FR-003 | ParseAnthropicOAuthRateLimitHeaders | 1 |
| FR-004 | RateLimitStore interface | 1 |
| FR-005 | InMemoryRateLimitStore | 1 |
| FR-006 | Token key 提取邏輯 | 2 |
| FR-007 | native_format.go 整合 | 2 |
| FR-008 | handleRateLimitState handler | 3 |
| FR-009 | routes.go 路由 | 3 |
| FR-010 | RateLimitStore 注入 UI handler | 3 |
| FR-011 | RateLimitWidget templ component | 4 |
| FR-012 | handleUsage 整合 widget data | 4 |
| FR-013 | 前端 30 秒 polling | 4 |
| FR-014 | 單 token 退化顯示 | 4 |
| FR-015 | Prune goroutine | 2 |
| FR-016 | API key backward compat | 2 |

---

## Addendum（v3）— Multi-Token Routing & 429 Rate Limit Parsing

> 本節擴充 Phase 2，涵蓋 FR-017 ~ FR-019 的實作細節。

### Phase 2 擴充

#### 2a：resolveAllNativeUpstreams + selectUpstream

**新增** `internal/proxy/handler/native_format.go`：

```go
// nativeUpstream holds the resolved base URL and API key for one upstream entry.
type nativeUpstream struct {
    BaseURL string
    APIKey  string
}

// resolveAllNativeUpstreams returns ALL config entries matching providerName.
// Returns empty slice if none found.
func (h *Handlers) resolveAllNativeUpstreams(providerName string) []nativeUpstream {
    var results []nativeUpstream
    for _, m := range h.Config.ModelList {
        parts := strings.SplitN(m.TianjiParams.Model, "/", 2)
        if len(parts) >= 1 && parts[0] == providerName {
            apiKey := ""
            if m.TianjiParams.APIKey != nil {
                apiKey = *m.TianjiParams.APIKey
            }
            base := ""
            if m.TianjiParams.APIBase != nil {
                base = *m.TianjiParams.APIBase
            }
            if base == "" {
                base = defaultBaseURL(providerName)
            }
            results = append(results, nativeUpstream{BaseURL: base, APIKey: apiKey})
        }
    }
    return results
}

// upstreamCounter is a package-level counter for round-robin selection.
var upstreamCounter atomic.Uint64

// selectUpstream picks an upstream from the list using round-robin.
// Safe for concurrent use via atomic.Uint64.
func selectUpstream(upstreams []nativeUpstream) nativeUpstream {
    idx := upstreamCounter.Add(1) - 1
    return upstreams[idx%uint64(len(upstreams))]
}
```

**修改** `nativeProxy`：
- 改呼叫 `resolveAllNativeUpstreams` → `selectUpstream`
- proxy closure（Director + ModifyResponse）捕獲被選中的 `upstream.APIKey`（not config 第一個）
- `tokenKey` 計算改用 `selectedUpstream.APIKey`
- `resolveNativeUpstream` 保留作為 backward-compat wrapper：`return resolveAllNativeUpstreams()[0]`（外部不應再新增呼叫點）

#### 2b：429 路徑也解析 rate limit headers（FR-019）

**修改** `ModifyResponse` 中的 non-200 早期 return 路徑：

目前結構：
```go
if resp.StatusCode != http.StatusOK {
    // ... log error to DB ...
    return nil   // ← 這裡漏掉了 rate limit 解析
}
// rate limit 解析只在 200 才執行
if providerName == "anthropic" {
    state := callback.ParseAnthropicRateLimitHeaders(resp.Header)
    ...
}
```

修正後：在 non-200 path 的 `return nil` 之前，插入 rate limit 解析：

```go
if resp.StatusCode != http.StatusOK {
    // ... 現有 error log 邏輯 ...

    // FR-019：429（及其他非 200）response 也要解析 rate limit headers
    // 這是最重要的訊號：token 已耗盡時 Anthropic 仍會帶 rate limit headers
    if providerName == "anthropic" && h.RateLimitStore != nil {
        rlState := callback.ParseAnthropicOAuthRateLimitHeaders(resp.Header, tokenKey)
        h.RateLimitStore.Set(callback.RateLimitCacheKey(tokenKey), rlState)
    }

    return nil
}
```

注意：`DiscordAlerter.CheckAndAlert` **不**移到此處，維持只在 200 path 觸發。

#### 受影響檔案（Phase 2 擴充）

| 檔案 | 異動 |
|------|------|
| `internal/proxy/handler/native_format.go` | 新增 `nativeUpstream`, `resolveAllNativeUpstreams`, `selectUpstream`, `upstreamCounter`；修改 `nativeProxy` 使用 round-robin；修改 `ModifyResponse` non-200 path 加 rate limit 解析 |

#### FR → Phase 對照（新增）

| FR | 說明 | Phase |
|----|------|-------|
| FR-017 | resolveAllNativeUpstreams | 2（擴充） |
| FR-018 | selectUpstream round-robin | 2（擴充） |
| FR-019 | 429 path rate limit 解析 | 2（擴充） |
