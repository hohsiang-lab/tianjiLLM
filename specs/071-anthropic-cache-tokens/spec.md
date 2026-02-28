# Spec: HO-71 — Anthropic Cache Tokens 計價修正

**Linear Issue**: HO-71  
**Branch**: `fix/ho71-anthropic-cache-tokens`  
**Priority**: High  
**Scope**: Anthropic native proxy streaming + non-streaming 計價全修

---

## Background

Claude Code 透過 tianjiLLM proxy 發送 Anthropic native format 請求時，Anthropic Prompt Cache 功能會在 usage object 中回傳三種 token：

```json
"usage": {
  "input_tokens": 1,
  "cache_creation_input_tokens": 0,
  "cache_read_input_tokens": 50000
}
```

目前系統只讀 `input_tokens`（1），導致：
1. `prompt_tokens` 嚴重低估（應為 50001）
2. `spend` 完全沒計算 cache 費用（$0.015/request 被漏掉）
3. 200K+ token context 走錯費率（無 tiered pricing）

**關鍵發現**：`model_prices.json` 已經內建所有 Anthropic 模型的 cache 費率資料，包含 200K threshold 版本。**資料已在，只差 `ModelInfo` struct 沒有對應欄位去接**。

```json
// model_prices.json 已有（claude-sonnet-4-20250514 為例）
{
  "input_cost_per_token": 3e-06,
  "output_cost_per_token": 1.5e-05,
  "cache_read_input_token_cost": 3e-07,          ← 已有，struct 沒讀
  "cache_creation_input_token_cost": 3.75e-06,   ← 已有，struct 沒讀
  "input_cost_per_token_above_200k_tokens": 6e-06,          ← 已有，struct 沒讀
  "output_cost_per_token_above_200k_tokens": 2.25e-05,      ← 已有，struct 沒讀
  "cache_read_input_token_cost_above_200k_tokens": 6e-07,   ← 已有，struct 沒讀
  "cache_creation_input_token_cost_above_200k_tokens": 7.5e-06  ← 已有，struct 沒讀
}
```

**參考**：LiteLLM `calculate_usage()` + `generic_cost_per_token()` + `_calculate_input_cost()` 實作。

---

## User Stories

### US-1：Cache Tokens 正確 Parse（Parse Layer）

**As a** proxy admin  
**I want** `prompt_tokens` 包含 `input_tokens + cache_read_input_tokens + cache_creation_input_tokens`  
**So that** Request Logs 顯示真實輸入量

**Acceptance Criteria**:
- Streaming path (`parseSSEUsage`): `message_start` 事件解析全部三個欄位
- Non-streaming path (`parseUsage`): Anthropic response body 解析全部三個欄位
- `prompt_tokens = input_tokens + cache_read + cache_creation`（與 LiteLLM 行為一致）
- 欄位 absent 時 default 為 0，不影響非 cache 請求（backward-compatible）

---

### US-2：ModelInfo Struct 加 Cache 費率欄位

**As a** pricing engineer  
**I want** `ModelInfo` struct 讀取 `model_prices.json` 中已有的 cache 費率欄位  
**So that** JSON decode 時 cache 費率不被丟棄

**修法**（新增欄位，JSON key 對齊 model_prices.json）：
```go
type ModelInfo struct {
    // 現有
    InputCostPerToken  float64 `json:"input_cost_per_token"`
    OutputCostPerToken float64 `json:"output_cost_per_token"`
    // 新增 cache 費率
    CacheReadCostPerToken     float64 `json:"cache_read_input_token_cost"`
    CacheCreationCostPerToken float64 `json:"cache_creation_input_token_cost"`
    // 新增 200K threshold 費率
    InputCostAbove200K            float64 `json:"input_cost_per_token_above_200k_tokens"`
    OutputCostAbove200K           float64 `json:"output_cost_per_token_above_200k_tokens"`
    CacheReadCostAbove200K        float64 `json:"cache_read_input_token_cost_above_200k_tokens"`
    CacheCreationCostAbove200K    float64 `json:"cache_creation_input_token_cost_above_200k_tokens"`
}
```

**Acceptance Criteria**:
- `ModelInfo` 新增上述 8 個欄位（現有 2 個不動）
- JSON unmarshal 後 `claude-sonnet-4-20250514` 的 `CacheReadCostPerToken` = 3e-07
- JSON unmarshal 後 `CacheReadCostAbove200K` = 6e-07
- `spend/calculator.go` 的 `ModelPricing` struct 同步加入 cache 費率欄位

---

### US-3：Cache 費率分層計價（Cost Calculation Layer）

**As a** proxy admin  
**I want** cache tokens 用各自正確費率計算 spend  
**So that** 計費準確，不高估也不低估

**費率結構**（從 `model_prices.json` 讀取，不 hardcode）：

| Token 類型 | 費率（claude-sonnet-4） | ModelInfo 欄位 |
|-----------|----------------------|--------------|
| 一般 input | $3.00/M = 3e-06 | `InputCostPerToken` |
| Cache read | $0.30/M = 3e-07 | `CacheReadCostPerToken` |
| Cache write | $3.75/M = 3.75e-06 | `CacheCreationCostPerToken` |

**計算公式**（LiteLLM `_calculate_input_cost()` 邏輯）：
```
spend = input_tokens        × InputCostPerToken
      + cache_read_tokens   × CacheReadCostPerToken
      + cache_creation_tokens × CacheCreationCostPerToken
      + completion_tokens   × OutputCostPerToken
```

**Acceptance Criteria**:
- `pricing.Calculator.Cost()` 新增 `cacheRead`, `cacheCreation` 參數（或改接受 struct）
- Unit test：50K cache_read → cost = 50000 × 3e-07 = 0.015（不是 0.15，差 10 倍）
- Unit test：1 input + 50K cache_read → total 分兩費率分別計算
- 無 cache 的請求 backward-compatible（`CacheReadCostPerToken=0` → cost 不變）

---

### US-4：200K Token Threshold Tiered Pricing

**As a** proxy admin  
**I want** 超過 200K context 的請求自動套用 tiered 費率  
**So that** 長 context 請求計費正確

**費率資料已在 `model_prices.json`**，只需在 `Cost()` 加 threshold 判斷：

```go
func (c *Calculator) Cost(model string, promptTokens, completionTokens, cacheRead, cacheCreation int) (float64, float64) {
    info := c.lookup(model)
    // threshold 判斷
    inputRate := info.InputCostPerToken
    outputRate := info.OutputCostPerToken
    cacheReadRate := info.CacheReadCostPerToken
    cacheCreationRate := info.CacheCreationCostPerToken

    if promptTokens > 200000 && info.InputCostAbove200K > 0 {
        inputRate = info.InputCostAbove200K
        outputRate = info.OutputCostAbove200K
        cacheReadRate = info.CacheReadCostAbove200K
        cacheCreationRate = info.CacheCreationCostAbove200K
    }
    // ...
}
```

**Acceptance Criteria**:
- `promptTokens > 200000` → 所有費率切換 tiered 版本（input / output / cache_read / cache_creation）
- `CacheReadCostAbove200K = 0`（model 無此欄位）→ fallback 用 `CacheReadCostPerToken`
- Unit test：210K prompt → input_cost = 6e-06（tiered）
- Unit test：190K prompt → input_cost = 3e-06（standard）
- Unit test：210K + 50K cache_read → cache_read_cost = 6e-07（tiered）
- Unit test：model 無 threshold 欄位 → no error，用 standard rate

---

### US-5：Cache Token 資訊傳遞到 Logger（Data Pipeline）

**As a** observability engineer  
**I want** `callback.LogData` 包含 `CacheReadTokens` + `CacheCreationTokens`  
**So that** spend tracker 拿到正確 cache token 數量，傳給 `Calculator.Cost()`

**Acceptance Criteria**:
- `callback.LogData` 加 `CacheReadTokens int` + `CacheCreationTokens int`
- `buildNativeLogData()` 填入這兩個欄位（從 parseSSEUsage / parseUsage 的回傳值）
- `spend.SpendRecord` 加 `CacheReadTokens` + `CacheCreationTokens`
- `spend.Tracker.Record()` → `calculateCost()` 把 cache tokens 傳給 `Calculator.Cost()`

---

### US-6：End-to-End Spend 正確

**As a** proxy user  
**I want** Request Logs Cost 欄位反映真實 cache 費用  
**So that** 和 Anthropic billing console 一致

**預期**（claude-sonnet-4，1 input + 50K cache_read + 500 completion）：
```
input_tokens:        1 × 3e-06    = $0.000003
cache_read_tokens: 50K × 3e-07    = $0.015000
completion_tokens: 500 × 1.5e-05  = $0.007500
Total:                              $0.022503
目前算出：                           $0.0000045（差 ~5000 倍）
```

**Acceptance Criteria**:
- `SpendLogs.spend` 正確包含 cache 費用
- `SpendLogs.prompt_tokens` = total input（input + cache_read + cache_creation）
- Integration test：mock SSE 含 cache tokens → SpendLog.spend = 正確值

---

## Out of Scope（本次不做）

- `SpendLogs` DB schema 新增 `cache_read_tokens` / `cache_creation_tokens` column（P2，另開 issue）
- `cache_creation_input_token_cost_above_1hr`（ephemeral cache 1hr tier）
- Gemini / OpenAI cache token 計價（另開 issue）
- OpenAI compat response `prompt_tokens_details.cached_tokens` 透傳

---

## Files to Modify

| 檔案 | 改動 |
|------|------|
| `internal/pricing/pricing.go` | `ModelInfo` 加 8 個 cache + threshold 欄位；`Cost()` 加 cache 參數 + threshold 判斷 |
| `internal/spend/calculator.go` | `ModelPricing` + `Calculate()` 同步更新 |
| `internal/proxy/handler/native_format.go` | `parseSSEUsage` + `parseUsage` 加 cache 欄位；return cache tokens；`buildNativeLogData()` 填 cache tokens |
| `internal/callback/callback.go` | `LogData` 加 `CacheReadTokens` + `CacheCreationTokens` |
| `internal/spend/tracker.go` | `SpendRecord` + `calculateCost()` 傳 cache tokens |

---

## Test Cases（魏徵負責）

### Parse Layer（native_format_cache_test.go）
- T01：SSE Anthropic 50K cache_read → prompt=50001, cacheRead=50000
- T02：SSE Anthropic 無 cache → backward-compat（prompt=1500）
- T03：SSE Anthropic with cache_creation → prompt=input+creation
- T04：Non-stream Anthropic with cache_read → prompt=50001
- T05：Non-stream Anthropic 無 cache → backward-compat

### Cost Calculation（pricing_cache_test.go）
- T06：50K cache_read → cost = 50000 × 3e-07（≠ 3e-06）
- T07：1000 cache_creation → cost = 1000 × 3.75e-06
- T08：1 input + 50K cache_read → 各自費率計算
- T09：無 cache token → backward-compat

### Threshold Pricing（pricing_cache_test.go）
- T10：210K prompt → input_cost = 6e-06（tiered）
- T11：190K prompt → input_cost = 3e-06（standard）
- T12：210K + 50K cache_read → cache_read_cost = 6e-07（tiered）
- T13：model 無 threshold 欄位 → no error，standard rate fallback

### Integration（spend_tracking_cache_test.go）
- T14：mock SSE with cache → SpendLog.spend 正確
- T15：mock SSE 無 cache → backward-compat
