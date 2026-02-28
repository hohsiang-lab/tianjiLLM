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

**參考**：LiteLLM `calculate_usage()` + `generic_cost_per_token()` + `_calculate_input_cost()` 實作。

---

## User Stories

### US-1：Cache Read Tokens 正確記錄（Parse Layer）

**As a** proxy admin  
**I want** `prompt_tokens` 包含 `input_tokens + cache_read_input_tokens + cache_creation_input_tokens`  
**So that** Request Logs 顯示真實輸入量

**Acceptance Criteria**:
- Streaming path (`parseSSEUsage`): `message_start` 解析全部三個欄位
- Non-streaming path (`parseUsage`): Anthropic response 解析全部三個欄位
- `prompt_tokens = input_tokens + cache_read + cache_creation`（與 LiteLLM 一致）
- 欄位 absent 時 default 為 0，不影響非 cache 請求

---

### US-2：Cache 費率分層計價（Cost Calculation Layer）

**As a** proxy admin  
**I want** cache tokens 用各自正確費率計算 spend  
**So that** 計費準確，不高估也不低估

**Anthropic 費率結構**（claude-sonnet-4-20250514）：

| Token 類型 | 費率 | JSON 欄位 |
|-----------|------|---------|
| 一般 input | $3.00/M = 3e-06 | input_cost_per_token |
| Cache read | $0.30/M = 3e-07 | cache_read_input_token_cost |
| Cache write | $3.75/M = 3.75e-06 | cache_creation_input_token_cost |

**LiteLLM 做法** (`_calculate_input_cost()`):
```python
cost  = text_tokens    * input_rate
cost += cache_hit      * cache_read_rate   # 各自費率
cost += cache_creation * cache_write_rate
```

**Acceptance Criteria**:
- `ModelInfo` struct 新增 `CacheReadCostPerToken` + `CacheCreationCostPerToken`
- `pricing.Calculator.Cost()` 接受 cache token 數量，分費率計算
- `model_prices.json` 中已有的 cache 費率被正確讀入 struct
- 無 cache 請求 backward-compatible
- Unit test：50K cache_read → spend = 50000 × 3e-07（不是 3e-06，差 10 倍）

---

### US-3：200K Token Threshold Tiered Pricing

**As a** proxy admin  
**I want** 超過 200K context 的請求自動套用 tiered 費率  
**So that** 長 context 請求計費正確

**Anthropic 200K+ 費率**（claude-sonnet-4-20250514）：

| Token 類型 | 標準費率 | 200K+ 費率 | JSON key |
|-----------|---------|-----------|---------|
| Input | 3e-06 | 6e-06 (2x) | input_cost_per_token_above_200k_tokens |
| Cache read | 3e-07 | 6e-07 (2x) | cache_read_input_token_cost_above_200k_tokens |
| Cache write | 3.75e-06 | 7.5e-06 (2x) | cache_creation_input_token_cost_above_200k_tokens |
| Output | 1.5e-05 | 2.25e-05 (1.5x) | output_cost_per_token_above_200k_tokens |

**LiteLLM 做法** (`_get_token_base_cost()`):
```python
# 掃描 model_info 找 threshold keys
threshold_keys = [k for k in model_info if k.startswith("input_cost_per_token_above_")]
# 若 prompt_tokens > threshold → 所有費率換成 tiered 版本（含 cache）
```

**Acceptance Criteria**:
- `ModelInfo` 加 threshold 費率欄位
- `Calculator.Cost()` 判斷 `promptTokens > 200000` → 切換 tiered 費率
- Input / output / cache_read / cache_creation 全部 threshold 生效
- Unit test：210K prompt → input_cost = 6e-06
- Unit test：190K prompt → input_cost = 3e-06
- Unit test：210K + 50K cache_read → cache_read_cost = 6e-07（tiered）
- Unit test：model 無 threshold 欄位 → no error，用 standard rate

---

### US-4：Cache Token 資訊傳遞到 Logger（Data Pipeline）

**As a** observability engineer  
**I want** `callback.LogData` 包含 `CacheReadTokens` + `CacheCreationTokens`  
**So that** callback / SpendLog 可以做 cache 效率分析

**Acceptance Criteria**:
- `callback.LogData` 加 `CacheReadTokens int` + `CacheCreationTokens int`
- `buildNativeLogData()` 填入這兩個欄位
- `spend.SpendRecord` 加對應欄位
- `spend.Tracker.Record()` → `calculateCost()` 傳入 cache tokens
- `spend.Calculator.Calculate()` 與 `pricing.Calculator.Cost()` 對齊

---

### US-5：End-to-End Spend 正確

**As a** proxy user  
**I want** Request Logs Cost 欄位反映真實 cache 費用  
**So that** 和 Anthropic billing console 一致

**預期**（claude-sonnet-4，50K cache_read）：
```
input_tokens:       1 × 3e-06    = $0.000003
cache_read_tokens: 50000 × 3e-07  = $0.015000
output_tokens:    500 × 1.5e-05  = $0.007500
Total:                            ~$0.022503  ← 目前算 ~$0.0000045
```

**Acceptance Criteria**:
- `SpendLogs.spend` 正確包含 cache 費用
- `SpendLogs.prompt_tokens` = total input（含 cache）
- Integration test：mock SSE 含 cache tokens → SpendLog.spend 正確

---

## Out of Scope（本次不做）

- `SpendLogs` schema 新增 `cache_read_tokens` / `cache_creation_tokens` column（P2）
- `cache_creation_input_token_cost_above_1hr`（ephemeral 1hr tier）
- Gemini / OpenAI cache token 計價（另開 issue）
- OpenAI compat response `prompt_tokens_details.cached_tokens` 透傳

---

## Files to Modify

| 檔案 | 改動 |
|------|------|
| `internal/proxy/handler/native_format.go` | `parseSSEUsage` + `parseUsage` 加 cache 欄位；`buildNativeLogData()` 填 cache tokens |
| `internal/callback/callback.go` | `LogData` 加 `CacheReadTokens` + `CacheCreationTokens` |
| `internal/pricing/pricing.go` | `ModelInfo` 加 cache + threshold 費率；`Cost()` 分費率 + threshold 判斷 |
| `internal/spend/calculator.go` | `ModelPricing` + `Calculate()` 同步 |
| `internal/spend/tracker.go` | `SpendRecord` + `calculateCost()` 傳 cache tokens |

---

## Test Cases（魏徵負責）

### Streaming（native_format_test.go）
- T01：Anthropic SSE 50K cache_read → prompt=50001, cacheRead=50000
- T02：Anthropic SSE 無 cache → backward-compat
- T03：Anthropic SSE 有 cache_creation → prompt=input+creation

### Non-Streaming
- T04：Anthropic response with cache_read
- T05：Anthropic response 無 cache

### Cost Calculation（pricing_test.go）
- T06：50K cache_read → cost = 50000 × 3e-07（≠ 3e-06）
- T07：1000 cache_creation → cost = 1000 × 3.75e-06
- T08：1 input + 50K cache_read → total = 1×3e-06 + 50000×3e-07
- T09：無 cache → backward-compat

### Threshold Pricing（pricing_test.go）
- T10：210K prompt → input_cost = 6e-06（tiered）
- T11：190K prompt → input_cost = 3e-06（standard）
- T12：210K + 50K cache_read → cache_read_cost = 6e-07（tiered）
- T13：model 無 threshold 欄位 → no error

### Integration（spend_tracking_test.go）
- T14：mock SSE with cache → SpendLog.spend correct
- T15：mock SSE 無 cache → backward-compat
