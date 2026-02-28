# Spec HO-70: Anthropic Cache Tokens 計入 Spend 及 prompt_tokens

**Priority**: High  
**Status**: Spec  
**Issue**: HO-70  

---

## 1. 背景與問題

Anthropic API 回傳的 usage 欄位包含三種 token 計數：

| 欄位 | 說明 | 計費價格（claude-3-5-sonnet） |
|------|------|------|
| `input_tokens` | 一般輸入 token | $3.00/M |
| `cache_read_input_tokens` | 從 prompt cache 讀取的 token | $0.30/M |
| `cache_creation_input_tokens` | 寫入 prompt cache 的 token | $3.75/M |

目前 `parseSSEUsage`（streaming）與 `parseUsage`（non-streaming）的 Anthropic struct 均**未解析** `cache_read_input_tokens` 和 `cache_creation_input_tokens`，導致：

1. **`prompt_tokens` 低報**：只計 `input_tokens`，cache token 完全丟失
2. **Spend 低報**：cache read/creation 未計費，每次 Claude Code 請求約漏算 $0.015（50K cache read × $0.30/M）
3. **分析失準**：帳單對不上，用量追蹤失真

---

## 2. 目標

- `prompt_tokens` = `input_tokens` + `cache_read_input_tokens` + `cache_creation_input_tokens`
- Spend 計算加入 cache read / creation 的差異定價
- Streaming（SSE）與 non-streaming 路徑均修復
- 不影響 OpenAI / Gemini / 其他 provider

---

## 3. 範圍

### 不在範圍內

- DB schema 修改（`prompt_tokens` 欄位定義不變，只是值更準確）
- 前端 UI 改動
- 其他 provider 的 cache token 支援

---

## 4. 技術設計

### 4.1 新增 UsageResult struct

為避免 parse function signature 爆炸，定義統一回傳結構：

```go
// internal/proxy/handler/native_format.go

type UsageResult struct {
    PromptTokens             int
    CompletionTokens         int
    ModelName                string
    CacheReadInputTokens     int // Anthropic only
    CacheCreationInputTokens int // Anthropic only
}
```

### 4.2 parseSSEUsage — Anthropic struct 擴充

```go
// 修改前
Usage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
}

// 修改後
Usage struct {
    InputTokens              int `json:"input_tokens"`
    OutputTokens             int `json:"output_tokens"`
    CacheReadInputTokens     int `json:"cache_read_input_tokens"`
    CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}
```

計算邏輯：
```go
if event.Type == "message_start" {
    u := event.Message.Usage
    prompt = u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
    cacheRead = u.CacheReadInputTokens
    cacheCreation = u.CacheCreationInputTokens
}
```

注意：cache token 只出現在 `message_start` 的 `message.usage`，`message_delta` 只有 `output_tokens`。

### 4.3 parseUsage（non-streaming）同步修復

```go
case "anthropic":
    var parsed struct {
        Model string `json:"model"`
        Usage struct {
            InputTokens              int `json:"input_tokens"`
            OutputTokens             int `json:"output_tokens"`
            CacheReadInputTokens     int `json:"cache_read_input_tokens"`
            CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
        } `json:"usage"`
    }
    if json.Unmarshal(body, &parsed) == nil {
        u := parsed.Usage
        totalPrompt := u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
        // return UsageResult{totalPrompt, u.OutputTokens, parsed.Model, u.CacheReadInputTokens, u.CacheCreationInputTokens}
    }
```

### 4.4 ModelPricing 擴充

```go
// internal/spend/calculator.go

type ModelPricing struct {
    InputCostPerToken              float64 `json:"input_cost_per_token"`
    OutputCostPerToken             float64 `json:"output_cost_per_token"`
    CacheReadInputCostPerToken     float64 `json:"cache_read_input_cost_per_token"`
    CacheCreationInputCostPerToken float64 `json:"cache_creation_input_cost_per_token"`
    MaxTokens                      int     `json:"max_tokens"`
    MaxInputTokens                 int     `json:"max_input_tokens"`
}
```

### 4.5 新增 CalculateWithCache

```go
type CacheTokens struct {
    ReadInputTokens     int
    CreationInputTokens int
}

// CalculateWithCache 計算含 cache 差異定價的 spend。
// 非 Anthropic provider 傳 CacheTokens{} 即可，行為與 Calculate 一致。
func (c *Calculator) CalculateWithCache(model string, promptTokens, completionTokens int, cache CacheTokens) float64 {
    c.mu.RLock()
    pricing, ok := c.prices[model]
    c.mu.RUnlock()
    if !ok {
        return 0
    }

    // promptTokens 已含 cache tokens，需反推純 input
    baseInputTokens := promptTokens - cache.ReadInputTokens - cache.CreationInputTokens

    cost := float64(baseInputTokens)*pricing.InputCostPerToken +
        float64(completionTokens)*pricing.OutputCostPerToken

    readRate := pricing.CacheReadInputCostPerToken
    if readRate == 0 {
        readRate = pricing.InputCostPerToken // fallback
    }
    creationRate := pricing.CacheCreationInputCostPerToken
    if creationRate == 0 {
        creationRate = pricing.InputCostPerToken // fallback
    }

    cost += float64(cache.ReadInputTokens) * readRate
    cost += float64(cache.CreationInputTokens) * creationRate
    return cost
}
```

原 `Calculate` 保留不動，避免 breaking change。

### 4.6 LogData / Record 擴充

`internal/spend/tracker.go` 的 LogData 或 Record struct 加入：

```go
CacheReadInputTokens     int
CacheCreationInputTokens int
```

tracker 呼叫 cost 計算時改用 `CalculateWithCache`。

### 4.7 Pricing JSON 更新

```json
"claude-3-5-sonnet-20241022": {
    "input_cost_per_token": 0.000003,
    "output_cost_per_token": 0.000015,
    "cache_read_input_cost_per_token": 0.0000003,
    "cache_creation_input_cost_per_token": 0.00000375
},
"claude-3-7-sonnet-20250219": {
    "input_cost_per_token": 0.000003,
    "output_cost_per_token": 0.000015,
    "cache_read_input_cost_per_token": 0.0000003,
    "cache_creation_input_cost_per_token": 0.00000375
}
```

---

## 5. 驗收條件

| # | 條件 | 測試方式 |
|---|------|----------|
| AC-1 | Streaming：`message_start` 含 `cache_read_input_tokens=50000` 時，`PromptTokens = input_tokens + 50000` | Unit test: `TestParseSSEUsage_AnthropicCacheTokens` |
| AC-2 | Non-streaming：response body 含 cache tokens 時，`PromptTokens` 正確合併 | Unit test: `TestParseUsage_AnthropicCacheTokens` |
| AC-3 | 50K cache read tokens 計費 $0.015（不是 $0.15） | Unit test: `TestCalculateWithCache_CacheRead` |
| AC-4 | 10K cache creation tokens 計費 $0.0375 | Unit test: `TestCalculateWithCache_CacheCreation` |
| AC-5 | `CacheTokens{}` 零值時結果與原 `Calculate` 相同 | Unit test: `TestCalculateWithCache_NoCache` |
| AC-6 | Pricing JSON 未設 cache rate 時 fallback 用 `input_cost_per_token` | Unit test |
| AC-7 | OpenAI / Gemini parse 行為不變 | 現有 test 全過 |
| AC-8 | `go test ./...` 全過，無 regression | CI |

---

## 6. 影響分析

| 元件 | 影響 |
|------|------|
| `internal/proxy/handler/native_format.go` | 修改：parse struct 加欄位、新增 `UsageResult`、調整計算邏輯 |
| `internal/spend/calculator.go` | 修改：`ModelPricing` 加欄位、新增 `CalculateWithCache` |
| `internal/spend/tracker.go` | 修改：LogData/Record 加欄位、呼叫 `CalculateWithCache` |
| Pricing JSON | 更新：加入 claude-* 的 cache pricing |
| DB schema | 無影響 |
| OpenAI / Gemini | 無影響（zero-value cache tokens） |

---

## 7. 實作順序

1. `native_format.go`：定義 `UsageResult`，擴充兩個 parse func
2. `spend/calculator.go`：擴充 `ModelPricing`，新增 `CalculateWithCache`
3. `spend/tracker.go`：使用新 struct 與新方法
4. Pricing JSON 更新 claude-* 條目
5. 補 unit tests（AC-1 ~ AC-7）
6. `go test ./...` 全過
