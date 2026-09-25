# Multiple Claude OAuth Token 使用策略

## 概述

TianjiLLM 透過三層機制管理多個 Anthropic OAuth token，在不主動探測的情況下，依靠真實響應頭被動感知每個 token 的健康狀態，並以 round-robin 調度健康的 token。

---

## 架構元件

| 元件 | 檔案 | 職責 |
|------|------|------|
| `IsOAuthToken` | `internal/provider/anthropic/oauth.go` | 識別 OAuth token（`sk-ant-oat` 前綴） |
| `InMemoryRateLimitStore` | `internal/callback/ratelimit_store.go` | 儲存每個 token 的 rate limit 狀態 |
| `selectUpstreamWithThrottle` | `internal/proxy/handler/native_upstream.go` | 過濾 throttled token，round-robin 選擇 |
| `nativeProxy` | `internal/proxy/handler/native_format.go` | 轉發請求，從響應頭更新狀態 |
| `DiscordRateLimitAlerter` | `internal/callback/discord_ratelimit.go` | 狀態惡化時發送 Discord 告警 |

---

## 啟動初始化 (`cmd/tianji/main.go`)

```
main()
  ├── rateLimitStore = callback.NewInMemoryRateLimitStore()
  │     └── goroutine: 每 1 分鐘 Prune(5 * time.Minute)
  │           // 超過 5 分鐘未更新的 entry 會被清除
  │           // 清除後該 token 回到「首次放行」狀態
  │
  ├── discordAlerter = callback.NewDiscordRateLimitAlerter(webhookURL, threshold)
  │     // webhookURL 為空 → nil，所有呼叫都有 != nil guard
  │
  ├── handlers.RateLimitStore = rateLimitStore
  ├── handlers.DiscordAlerter = discordAlerter
  ├── handlers.Config = cfg              // cfg.ModelList 是 token 來源
  │
  └── uiHandler.RateLimitStore = rateLimitStore
        // handlers 和 uiHandler 共享同一個 store 指標
```

---

## 路由註冊 (`internal/proxy/server.go`)

```
POST /v1/messages
  middleware 鏈：AuthMiddleware → parallelMW → dynamicRateMW → cacheControlMW
                                                    ↑
                                      這是 virtual key 的 RPM/TPM，
                                      與 OAuth token rate limit 是兩套獨立機制
  └── handlers.AnthropicMessages()
        └── h.nativeProxy(w, r, "anthropic")
```

---

## 請求處理完整流程

### Step 1：解析所有可用 upstream

```
resolveAllNativeUpstreams("anthropic")
  遍歷 cfg.ModelList（每次請求重新遍歷，O(N)，熱重載即生效）
  過濾 TianjiParams.Model 前綴為 "anthropic" 的條目
  → []nativeUpstream{ {BaseURL, APIKey}, ... }

  config 範例：
    model_list:
      - tianji_params:
          model: anthropic/claude-sonnet-4-5
          api_key: $SK_ANT_OAT_TOKEN_1    ← nativeUpstream.APIKey
      - tianji_params:
          model: anthropic/claude-opus-4
          api_key: $SK_ANT_OAT_TOKEN_2    ← nativeUpstream.APIKey
```

### Step 2：選擇健康的 token

```
selectUpstreamWithThrottle("anthropic", upstreams)
  threshold = cfg.RatelimitAlertThreshold  // 預設 0.8（80%）
  seen = map[string]bool{}                 // 基於 APIKey 去重

  for each upstream:
    if seen[apiKey] → skip（同一 token 多個 model 條目只計一次）

    if !IsOAuthToken(apiKey):
      → available  // 非 OAuth 直接放行，無 rate limit 檢查

    key = sha256(apiKey)[:12]
    state, ok = RateLimitStore.Get(key)

    if !ok:
      → available  // 無記錄（首次或已被 Prune）→ 放行

    if state.UnifiedStatus == "rate_limited" || "overage":
      → skip + trackNearestReset  // 硬封鎖：Anthropic 明確拒絕

    if state.Unified5hUtilization >= threshold
    || state.Unified7dUtilization >= threshold:
      → skip + trackNearestReset  // 軟封鎖：接近上限，主動迴避

    → available

  if len(available) == 0:
    → allTokensThrottledError{resetAt: nearestReset}

  → roundRobinSelect("anthropic", available)
```

### Step 3：Round-Robin 選擇

```
roundRobinSelect("anthropic", available)
  全域 map[string]*atomic.Uint64，key = providerName
  // 每個 provider 獨立計數，避免跨 provider 干擾

  idx = counter.Add(1) - 1
  return available[idx % len(available)]
```

### Step 4：轉發請求

```
httputil.ReverseProxy.Director:
  if IsOAuthToken(apiKey):
    req.Header.Set("Authorization", "Bearer " + apiKey)
    req.Header.Set("anthropic-beta", "oauth-2025-04-20," + existing)
    // anthropic-dangerous-direct-browser-access: true 已由 SetOAuthHeaders 設定
  else:
    req.Header.Set("x-api-key", apiKey)
  req.Header.Set("anthropic-version", "2023-06-01")  // 若 client 未帶
```

### Step 5：從響應頭更新狀態（成功與失敗都更新）

```
ModifyResponse(resp):

  if resp.StatusCode != 200:
    // FR-019：429 是最重要的信號，必須在此更新
    if providerName == "anthropic" && RateLimitStore != nil:
      tokenKey = sha256(apiKey)[:12]   // apiKey 從 closure 捕獲
      rlState = ParseAnthropicOAuthRateLimitHeaders(resp.Header, tokenKey)
      RateLimitStore.Set(tokenKey, rlState)       // 下次請求生效
      if DiscordAlerter != nil && IsOAuthToken(apiKey):
        DiscordAlerter.CheckAndAlertOAuth(rlState)  // goroutine，不阻塞
    return

  // 200 OK：同樣更新
  ParseAnthropicRateLimitHeaders(resp.Header)     // legacy 格式，觸發舊告警
  rlState = ParseAnthropicOAuthRateLimitHeaders(resp.Header, tokenKey)
  RateLimitStore.Set(tokenKey, rlState)
  DiscordAlerter.CheckAndAlertOAuth(rlState)
```

**解析的響應頭：**

```
anthropic-ratelimit-unified-status         → UnifiedStatus ("allowed"/"rate_limited"/"overage")
anthropic-ratelimit-unified-5h-utilization → Unified5hUtilization (float [0,1])
anthropic-ratelimit-unified-5h-reset       → Unified5hReset (unix timestamp)
anthropic-ratelimit-unified-7d-utilization → Unified7dUtilization (float [0,1])
anthropic-ratelimit-unified-7d-reset       → Unified7dReset (unix timestamp)
anthropic-ratelimit-unified-representative-claim → RepresentativeClaim ("five_hour"/"seven_day")
```

### Step 6：Discord 告警

```
CheckAndAlertOAuth(state):
  if UnifiedStatus == "rate_limited":
    → sendOAuthAlertIfNotCooling("ratelimit:oauth:rate_limited:<tokenKey>")
    → return  // 早返回：已硬封鎖，utilization 告警多餘

  if Unified5hUtilization >= threshold:
    → sendOAuthAlertIfNotCooling("ratelimit:oauth:5h_util:<tokenKey>")

  if Unified7dUtilization >= threshold:
    → sendOAuthAlertIfNotCooling("ratelimit:oauth:7d_util:<tokenKey>")

sendWithCooldown(key, msg):
  mu.Lock()
  if alerted[key] 存在 && time.Since(last) < 1h → return  // 每種告警每 token 1 小時冷卻
  alerted[key] = now
  mu.Unlock()
  POST webhookURL  // 同步發送（已在 goroutine 內）
```

### Step 7：全部 throttled 時的響應

```
allTokensThrottledError{resetAt: nearestReset}
  retryAfter = time.Until(resetAt).Seconds()
  if retryAfter < 1 → retryAfter = 60  // floor 60 秒

  HTTP 429
  Retry-After: <seconds>
  {"error": {"message": "all OAuth tokens throttled", "type": "rate_limit_error"}}
```

---

## 資料流總覽

```
proxy_config.yaml (ModelList)
    │
    │  每次請求重新遍歷
    ▼
resolveAllNativeUpstreams()
    │  []nativeUpstream{BaseURL, APIKey}
    ▼
selectUpstreamWithThrottle()
    │
    ├─ 非 OAuth ──────────────────────────────────────────► available
    │
    ├─ OAuth，RateLimitStore miss ────────────────────────► available（首次放行）
    │
    ├─ OAuth，status=rate_limited/overage ────────────────► skip + trackNearestReset
    │
    ├─ OAuth，5h/7d util ≥ 80% ───────────────────────────► skip + trackNearestReset
    │
    └─ OAuth，健康 ───────────────────────────────────────► available
                                │
                     roundRobinSelect（atomic counter per provider）
                                │
                                ▼
                        upstream.APIKey
                                │
                     ReverseProxy.Director
                     設置 Bearer / x-api-key header
                                │
                                ▼
                        Anthropic API
                                │
                     响應頭含 anthropic-ratelimit-unified-*
                                │
                     ModifyResponse（200 和非 200 都執行）
                                │
                    ┌───────────┴────────────┐
                    ▼                        ▼
           RateLimitStore.Set()    DiscordAlerter.CheckAndAlertOAuth()
           （下次請求生效）          （goroutine，1h 冷卻去重）
```

---

## 狀態機（單一 token）

```
              首次 / Prune 後
                    │
                    ▼
              ┌──────────┐
              │  無記錄   │ ──── 任何請求 ──► 放行，響應後寫入狀態
              └──────────┘
                    │
                    ▼ 寫入狀態
              ┌──────────┐
              │  allowed  │ ◄─── 每次請求後從響應頭更新
              └──────────┘
                    │
         ┌──────────┼──────────┐
         ▼          ▼          ▼
    5h/7d util   status=     status=
     ≥ 80%      overage    rate_limited
         │          │          │
         └──────────┴──────────┘
                    │
                    ▼
              ┌──────────┐
              │  throttled │ ──── 跳過，等待 resetAt
              └──────────┘
                    │
         reset 後下次響應頭更新，或 Prune 後清除
                    │
                    ▼
              ┌──────────┐
              │  allowed  │ 或 無記錄（Prune 後）
              └──────────┘
```

---

## 設計特性與陷阱

### 設計特性

| 特性 | 說明 |
|------|------|
| 被動感知 | 不主動探測，靠真實響應頭更新，零額外請求開銷 |
| 首次放行 | 無歷史記錄的 token 直接可用，無冷啟動問題 |
| 429 也更新 | `ModifyResponse` 對所有狀態碼都解析 rate limit headers |
| 兩套告警並存 | legacy `CheckAndAlert`（舊格式）+ OAuth `CheckAndAlertOAuth`（新格式）|
| 閉包捕獲 apiKey | `ModifyResponse` 捕獲的是已選定的 upstream apiKey，非 client key |

### 已知陷阱

| 陷阱 | 影響 |
|------|------|
| 狀態僅在記憶體 | 重啟後清空，需重新探測才能恢復 throttle 感知 |
| Prune 清除 throttle 狀態 | 低流量 token（5 分鐘無請求）的 throttle 狀態會消失，下次放行後才重新感知 |
| 延遲感知 | token 被 rate limit 後，至少有一個請求先打到 Anthropic 才能知道，下次才跳過 |
| 非 OAuth token 無 throttle 保護 | `!IsOAuthToken` 直接放行，不檢查任何 rate limit 狀態 |
| `resolveAllNativeUpstreams` 無快取 | 每次請求 O(N) 遍歷 ModelList；ModelList 極大時有效能影響 |
