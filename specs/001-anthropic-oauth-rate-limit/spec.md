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

---

## Addendum（v3）— Multi-Token Routing & 429 Rate Limit Parsing

> 本節為 Norman 確認後新增（2026-03-01），涵蓋 FR-017 ~ FR-019 與對應 SC。

### User Story 5：多 OAuth token round-robin routing

**As a** 系統管理員，設定了多把 Anthropic OAuth token，
**I want** tianjiLLM 將請求均勻分配到各 token，並分別追蹤每把 token 的 rate limit 用量，
**So that** Usage 頁面能真正顯示每把 token 各自的剩餘配額，而非全部混在一起。

### FR-017：resolveAllNativeUpstreams — 回傳所有符合的 Anthropic entry
`resolveNativeUpstream` 目前只回傳 ModelList 中第一筆符合的 entry，導致多把 OAuth token 的請求永遠使用同一把 key。
新增 `resolveAllNativeUpstreams(providerName string) []nativeUpstream`（`nativeUpstream` 為 `{BaseURL, APIKey string}`），回傳所有符合 provider 的 entry slice。
`resolveNativeUpstream` 保留作為 backward-compat wrapper（取 slice[0]），不破壞既有呼叫點。

### FR-018：selectUpstream — round-robin 策略
新增 `selectUpstream(upstreams []nativeUpstream) nativeUpstream`，以全域 `atomic.Uint64` counter 實作 round-robin，goroutine-safe，不需額外 mutex。
`nativeProxy` 改呼叫 `resolveAllNativeUpstreams` + `selectUpstream` 取得本次請求的 upstream，proxy closure 捕獲**被選中的那個** apiKey（而非 config 第一個）。
Rate limit store key = `sha256(selectedAPIKey)[:12]`，如此 UI 才能真正顯示多個 token 各自的使用量。

### FR-019：429 response 也解析 rate limit headers
目前 `ModifyResponse` 在 `resp.StatusCode != http.StatusOK` 時提前 return，漏掉 429 攜帶的 rate limit headers（這是最重要的訊號：token 已耗盡）。
修正：在 non-200 路徑中，若 `providerName == "anthropic"`，不論 status code，仍須呼叫 `ParseAnthropicOAuthRateLimitHeaders(resp.Header, tokenKey)` 並 `RateLimitStore.Set`。
Discord alerter 的呼叫邏輯維持在 status 200 path，不動。

### SC（Success Criteria）新增

- **SC-007**：設定兩把 Anthropic OAuth token → 連送 10 個請求 → `GET /ui/api/rate-limit-state` 回傳兩筆獨立記錄，各自有不同 TokenKey
- **SC-008**：Anthropic 回傳 429 → `GET /ui/api/rate-limit-state` 仍能看到該 token 的 rate limit state 被更新（remaining = 0 或 header 值）
- **SC-009**：單一 OAuth token 設定時，行為與既有一致（backward compat）
