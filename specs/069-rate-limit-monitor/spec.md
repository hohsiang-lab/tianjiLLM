# HO-69 — Anthropic Rate Limit 監控 + Discord 告警

**Priority**: Medium
**Author**: 諸葛亮（PM）
**Date**: 2026-03-01
**Branch**: `069-rate-limit-monitor`

---

## 1. 背景與問題

TianjiLLM proxy 透過 `http.DefaultClient.Do(httpReq)` 轉發請求到 Anthropic。
Anthropic 在每個 response 的 HTTP header 帶回 rate limit 資訊：

```
anthropic-ratelimit-tokens-limit: 800000
anthropic-ratelimit-tokens-remaining: 120000
anthropic-ratelimit-tokens-reset: 2026-03-01T00:00:00Z
anthropic-ratelimit-requests-limit: 1000
anthropic-ratelimit-requests-remaining: 50
```

**現況（codebase 調查結果）**：
- `grep anthropic-ratelimit` 全 repo 無結果 → 這些 header **完全未被讀取**
- `handleNonStreamingCompletion` / `handleStreamingCompletion` 取得 `resp` 後，直接交給 `p.TransformResponse()`，header 被丟棄
- 現有告警機制（`SlackCallback`）只處理 budget / 慢請求 / provider 錯誤率，無 rate limit 感知

**問題**：tokens remaining 耗盡前，operator 沒有任何預警，只能被動收到 429 錯誤。

---

## 2. 目標

1. 讀取 Anthropic response 的 `anthropic-ratelimit-*` headers
2. 以 in-memory store 儲存最新 rate limit 狀態（per provider key）
3. tokens remaining / tokens limit < 閾值（預設 20%，可設定）時，發 Discord webhook 告警
4. 告警支援 throttle（同 key 預設 1 小時內只打一次）

---

## 3. 範圍

### In scope
- 解析 `anthropic-ratelimit-*` headers（非 Anthropic 請求直接忽略）
- `sync.RWMutex` + map 儲存最新狀態（per `providerKey`）
- Discord webhook 告警（embed 格式）
- 閾值與 cooldown 可透過 config 設定
- `GET /internal/ratelimit` 管理 API（回傳目前所有 provider key 的狀態）

### Out of scope（後續 ticket）
- 持久化到 DB
- Requests 維度的告警
- 其他 provider（OpenAI、Azure）的 rate limit header

---

## 4. 設計

### 4.1 新增 package：`internal/ratelimit`

**state.go**

```go
package ratelimit

import (
    "sync"
    "time"
)

type State struct {
    TokensLimit       int
    TokensRemaining   int
    TokensReset       time.Time
    RequestsLimit     int
    RequestsRemaining int
    UpdatedAt         time.Time
}

type Store struct {
    mu    sync.RWMutex
    state map[string]*State
}

func NewStore() *Store
func (s *Store) ParseAndUpdate(providerKey string, h http.Header)
func (s *Store) Get(providerKey string) (*State, bool)
func (s *Store) All() map[string]State
```

`ParseAndUpdate` 讀取 header：
- `anthropic-ratelimit-tokens-limit`
- `anthropic-ratelimit-tokens-remaining`
- `anthropic-ratelimit-tokens-reset`（RFC3339）
- `anthropic-ratelimit-requests-limit`
- `anthropic-ratelimit-requests-remaining`

缺失的欄位保留舊值，不 panic。

### 4.2 DiscordAlerter

**alerter.go**

```go
type DiscordAlerter struct {
    webhookURL string
    threshold  float64       // 0.0–1.0，預設 0.20
    cooldown   time.Duration // 預設 1h
    client     *http.Client
    mu         sync.Mutex
    alerted    map[string]time.Time
}

func NewDiscordAlerter(webhookURL string, threshold float64, cooldown time.Duration) *DiscordAlerter
func (a *DiscordAlerter) Check(providerKey string, store *Store)
```

`Check` 流程：
1. 取得 State；`TokensLimit == 0` 則跳過
2. 計算 `ratio = TokensRemaining / TokensLimit`；`ratio > threshold` 則返回
3. 檢查 cooldown（mutex 保護）；在冷卻期內則返回
4. 更新 `alerted[providerKey]` 時間戳
5. POST Discord webhook（embed 格式）

Discord Embed payload：

```json
{
  "embeds": [{
    "title": "⚠️ Anthropic Rate Limit 警告",
    "color": 16776960,
    "fields": [
      {"name": "Provider Key", "value": "anthropic/sk-***xxxx"},
      {"name": "Tokens Remaining", "value": "120000 / 800000 (15.0%)"},
      {"name": "Reset At", "value": "2026-03-01T00:00:00Z"}
    ]
  }]
}
```

### 4.3 Config 新增欄位

`internal/config/config.go`：

```go
RateLimitMonitor *RateLimitMonitorConfig `yaml:"ratelimit_monitor,omitempty"`

type RateLimitMonitorConfig struct {
    Enabled           bool    `yaml:"enabled"`
    AlertThreshold    float64 `yaml:"alert_threshold"`    // 預設 0.20
    DiscordWebhookURL string  `yaml:"discord_webhook_url"`
    CooldownMinutes   int     `yaml:"cooldown_minutes"`   // 預設 60
}
```

範例 config：

```yaml
ratelimit_monitor:
  enabled: true
  alert_threshold: 0.20
  discord_webhook_url: "https://discord.com/api/webhooks/..."
  cooldown_minutes: 60
```

### 4.4 Handlers 注入

`internal/proxy/handler/handler.go` — Handlers struct 新增：

```go
RateLimitStore   *ratelimit.Store
RateLimitAlerter *ratelimit.DiscordAlerter
```

`internal/proxy/handler/chat.go` — 在 `handleNonStreamingCompletion` 與 `handleStreamingCompletion` 取得 `resp` 後、呼叫 `p.TransformResponse()` 前插入：

```go
if p.Name() == "anthropic" && h.RateLimitStore != nil {
    providerKey := "anthropic/" + last4(apiKey)
    h.RateLimitStore.ParseAndUpdate(providerKey, resp.Header)
    if h.RateLimitAlerter != nil {
        go h.RateLimitAlerter.Check(providerKey, h.RateLimitStore)
    }
}
```

### 4.5 管理 API

路由：`GET /internal/ratelimit`（加入現有 admin route group）

Response：

```json
{
  "providers": {
    "anthropic/xxxx": {
      "tokens_limit": 800000,
      "tokens_remaining": 120000,
      "tokens_reset": "2026-03-01T00:00:00Z",
      "requests_limit": 1000,
      "requests_remaining": 50,
      "updated_at": "2026-03-01T00:05:00Z"
    }
  }
}
```

---

## 5. 流程圖

```
Client → Proxy → Anthropic API
                      ↓
                 HTTP Response
                      ↓
         p.Name() == "anthropic"?
                Yes ↓
         Store.ParseAndUpdate(header)
                      ↓
         DiscordAlerter.Check() [goroutine]
              ratio < threshold?
                   Yes ↓
             cooldown check
                Pass ↓
         POST Discord Webhook (embed)
```

---

## 6. 驗收條件

| # | 條件 | 測試 |
|---|------|------|
| 1 | Anthropic resp header 解析後存入 Store | unit: mock resp headers → Store.Get() |
| 2 | 非 Anthropic provider 不更新 Store | unit: p.Name() = "openai" → Store 空 |
| 3 | ratio < 0.20 → 發 Discord webhook | unit: mock HTTP server 捕獲 POST |
| 4 | ratio >= 0.20 → 不發送 | unit: assert zero HTTP calls |
| 5 | 同 key 1h 內只打一次 | unit: 連呼兩次 Check，assert 1 call |
| 6 | header 部分缺失不 panic，保留舊值 | unit: partial headers |
| 7 | streaming 與 non-streaming 都觸發解析 | unit for both code paths |
| 8 | GET /internal/ratelimit 回傳正確 JSON | contract test |
| 9 | enabled: false 時完全跳過 | unit: alerter nil → no panic |

---

## 7. 影響評估

| 面向 | 影響 |
|------|------|
| 效能 | 每 request 多讀 5 個 header + 1 mutex write，可忽略 |
| 現有功能 | 無破壞，插入點在 TransformResponse 之前 |
| 相依套件 | 無新增外部依賴 |
| 設定向下相容 | `ratelimit_monitor` 為可選欄位，不設定 → disabled |

---

## 8. 實作順序（給魯班）

1. `internal/ratelimit/state.go` — Store + State + ParseAndUpdate
2. `internal/ratelimit/alerter.go` — DiscordAlerter
3. `internal/ratelimit/state_test.go` + `alerter_test.go`
4. `internal/config/config.go` — RateLimitMonitorConfig
5. `internal/proxy/handler/handler.go` — 注入欄位
6. `internal/proxy/handler/chat.go` — 呼叫 ParseAndUpdate + Check
7. `internal/proxy/server.go` — 初始化注入 + admin route
8. `test/contract/ratelimit_monitor_test.go`
