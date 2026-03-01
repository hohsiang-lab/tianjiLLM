# Spec: HO-74 — Usage 頁顯示 Anthropic Rate Limit 即時狀態

**Linear Issue**: HO-74  
**Branch**: `001-anthropic-oauth-rate-limit`  
**Priority**: High  
**Scope**: Anthropic OAuth token rate limit state storage, API endpoint, Usage 頁面 widget

---

## Background

tianjiLLM proxy 已透過 `ParseAnthropicRateLimitHeaders`（`internal/callback/discord_ratelimit.go`）
解析 Anthropic response headers 中的 rate limit 資訊，並可透過 `DiscordRateLimitAlerter` 發警告。

但目前：
1. Rate limit state 只用於觸發 Discord alert，**不儲存、不供 UI 查詢**
2. 系統未來會有多把 Anthropic OAuth token，**無法分辨哪把 token 的配額用了多少**
3. Usage 頁面沒有任何 rate limit 可視性

**目標**：建立 per-token rate limit state store，並在 Usage 頁面以 widget 呈現即時狀態。

---

## Constraints

- **C-01**：不能破壞 API key 向後相容性（現有 x-api-key flow 不變）
- **C-02**：OAuth token 識別 key = sha256(token)[:12] 或 token alias
- **C-03**：Rate limit state 為 in-memory（MVP），TTL = 5 分鐘
- **C-04**：`-1` 為 sentinel（missing/parse error），UI 顯示 N/A
- **C-05**：Widget 以 30 秒 polling 更新，不用 WebSocket

---

## Functional Requirements

### FR-001：AnthropicOAuthRateLimitState struct
定義 per-token rate limit state struct，欄位與 AnthropicRateLimitState 一致，額外增加 TokenKey 與 UpdatedAt。

### FR-002：RateLimitCacheKey(tokenKey string) string
產生 in-memory store key，格式 ratelimit:{tokenKey}。

### FR-003：ParseAnthropicOAuthRateLimitHeaders(h http.Header, tokenKey string) AnthropicOAuthRateLimitState
重用 ParseAnthropicRateLimitHeaders 的 header parsing 邏輯，帶入 tokenKey + UpdatedAt = time.Now()。

### FR-004：RateLimitStore interface
定義 Set / Get / GetAll / Prune 方法。

### FR-005：InMemoryRateLimitStore 實作
thread-safe（sync.RWMutex）in-memory store，實作 RateLimitStore interface。TTL = 5 分鐘。

### FR-006：Token key 提取
OAuth token 時 tokenKey = sha256(apiKey)[:12]；非 OAuth 使用 "default"。

### FR-007：Native format handler 整合
native_format.go Anthropic response 後呼叫 ParseAnthropicOAuthRateLimitHeaders 並 RateLimitStore.Set。
現有 DiscordAlerter.CheckAndAlert 不動。

### FR-008：handleRateLimitState HTTP handler
GET /ui/api/rate-limit-state 回傳 JSON array，每元素為一把 token 的最新狀態。無資料時回傳 []。

### FR-009：Rate limit state API 路由
routes.go 加入 GET /ui/api/rate-limit-state。

### FR-010：RateLimitStore 注入 UI handler
Handler struct 加入 RateLimitStore 欄位，由 server init 建立並注入。

### FR-011：Usage 頁面 rate limit widget（templ）
usage.templ 加入 RateLimitWidget：每把 token 一個 card，顯示 Input/Output/Requests limit/remaining/reset，UpdatedAt 顯示幾秒前。

### FR-012：Rate limit widget 嵌入 Usage 頁
handleUsage 初始渲染從 RateLimitStore.GetAll() 取資料傳入 template。

### FR-013：前端 30 秒 polling
Usage 頁面 JS 每 30 秒 GET /ui/api/rate-limit-state，更新 widget DOM。

### FR-014：單 token 退化顯示
GetAll() 只回傳一筆時，widget 不顯示 token key 標籤（backward compatible）。

### FR-015：Prune 排程
Server 啟動時起 goroutine，每分鐘呼叫 RateLimitStore.Prune(5 * time.Minute)。

### FR-016：API key（非 OAuth）backward compat
非 OAuth token 的 rate limit headers 以 "default" 為 key 存入 store，UI 顯示「Default API Key」。
不影響現有 API key 功能。

---

## Out of Scope

- Redis persistence（MVP 只做 in-memory）
- WebSocket push
- Rate limit history / 趨勢圖
- 非 Anthropic provider 的 rate limit

---

## Acceptance Criteria

1. Anthropic OAuth response headers 解析後，state 存入 InMemoryRateLimitStore（per-token key）
2. GET /ui/api/rate-limit-state 回傳正確 JSON，多 token 時回傳 array
3. Usage 頁面顯示 rate limit widget，30 秒自動更新
4. 5 分鐘無 response 後，state 自動被 Prune 清除
5. 現有 API key（非 OAuth）flow 不受影響，所有既有測試 pass
6. AnthropicRateLimitState（discord alert 用）struct 不變，不影響 DiscordRateLimitAlerter
