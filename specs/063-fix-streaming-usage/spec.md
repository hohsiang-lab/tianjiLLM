# HO-63: Fix Streaming Usage Extraction (Cost / Tokens = 0)

## 問題描述

Request Logs 的 Cost / Tokens 欄位全為 0。DB 中 `SpendLogs` 的 `spend = 0`、`total_tokens = 0`、`prompt_tokens = 0`、`completion_tokens = 0`。

## Root Cause 分析

經過程式碼追蹤，問題可能來自以下三層，且可能同時存在：

### RC-1: `spend.Calculator` 無 pricing 資料（高機率）

`cmd/tianji/main.go:207` 初始化 Spend Tracker 時：

```go
calc, _ := spend.NewCalculator("")  // 空路徑 → prices map 為空
```

`spend.Calculator.Calculate()` 找不到任何 model → 回傳 0。

雖然 `native_format.go` 的 `buildNativeLogData` 使用 `pricing.Default().TotalCost()` 計算 cost，但 `chat.go` 的 `logStreamSuccess` 也用 `pricing.Default().TotalCost(req.Model, ...)`。如果 `req.Model` 是 user-facing alias（如 `claude-sonnet`）而非完整 model ID（如 `claude-sonnet-4-20250514`），pricing lookup 會失敗。

**影響**：cost = 0（即使 tokens 正確提取）

### RC-2: OpenAI-compatible 路徑的 model name 不一致（中機率）

`/v1/chat/completions` 路徑：
1. `resolveProvider()` 回傳 `resolvedModel`（strip provider prefix 後的 model name）
2. `req.Model` 被設為此值
3. `logStreamSuccess` 用 `req.Model` 查 pricing

如果 config 的 `ModelName`（user-facing）跟 `TianjiParams.Model`（provider/actual）不同，而 `model_prices.json` 只有 actual model name，pricing lookup 可能失敗。

### RC-3: Anthropic 延伸思考模型的 usage 格式差異（低機率）

Anthropic 的 extended thinking 模型在 `message_start.message.usage` 可能包含額外欄位如 `cache_creation_input_tokens`、`cache_read_input_tokens`，但基本的 `input_tokens` / `output_tokens` 仍存在，不影響 parsing。

### RC-4: Native Proxy 路徑的 `sseSpendReader` 未確保 `Close()` 被調用（低機率）

`httputil.ReverseProxy` 正常情況下會 close response body，但如果 client 提前斷線或發生 panic，`Close()` 可能不被呼叫，導致 usage 永遠不被 log。

### 確認結果

- `parseSSEUsage()` 對 Anthropic streaming SSE 的 parsing 邏輯**正確**（unit test 通過）
- `handleMessageStart` / `handleMessageDelta` 在 OpenAI-compatible 路徑的 token 提取邏輯**正確**
- 問題最可能出在 **pricing lookup 失敗**（model name 不匹配）導致 cost = 0，以及 `spend.Calculator` 空初始化導致無 fallback

---

## 功能需求（FR）

### FR-1: 統一 Spend Tracker 的 pricing 來源

`spend.Tracker` 應使用 `pricing.Default()` 作為 cost 計算來源，而非獨立的 `spend.Calculator`。

**修改**：
- `cmd/tianji/main.go`：將 `spend.NewTracker(queries, calc, nil)` 改為傳入 `pricing.Default()` 或直接在 `Tracker.Record()` 中使用 `pricing.Default()`
- 移除 `spend.Calculator` 或將其改為 `pricing.Default()` 的 wrapper

### FR-2: Model name fallback 鏈

Pricing lookup 應支援多層 fallback：
1. 精確匹配（e.g., `claude-sonnet-4-20250514`）
2. Strip provider prefix（e.g., `anthropic/claude-sonnet-4-20250514` → `claude-sonnet-4-20250514`）
3. 正則匹配或版本模糊匹配（e.g., `claude-sonnet-4-*`）→ 可選，Phase 2

**注意**：`pricing.Calculator.lookup()` 已實作第 1、2 層。`spend.Calculator.Calculate()` 只有精確匹配，這是差距所在。

### FR-3: Streaming usage 提取的防禦性強化

- `sseSpendReader.Close()` 應加 `defer` 保護，確保即使 parse 失敗也能記錄零值 log（目前已有此行為，確認保留）
- 加入 log 警告：當 streaming 結束但 `prompt + completion == 0` 時，記錄 warning（方便 debug）

### FR-4: 加入 Anthropic extended thinking tokens 支援（可選）

解析 `cache_creation_input_tokens`、`cache_read_input_tokens` 並納入 cost 計算。目前不影響基本 usage，但對精確 cost 計算有幫助。

---

## 驗收標準（SC）

### SC-1: Streaming Anthropic 請求的 SpendLogs 記錄正確 tokens

**Given**: 一個 Anthropic streaming 請求（`/v1/messages` 或 `/v1/chat/completions`），回應包含 `message_start.message.usage.input_tokens = 100` 和 `message_delta.usage.output_tokens = 50`
**When**: 請求完成
**Then**: `SpendLogs` 記錄 `prompt_tokens = 100`、`completion_tokens = 50`、`total_tokens = 150`

### SC-2: Streaming Anthropic 請求的 SpendLogs 記錄正確 cost

**Given**: 同上場景，model 為 `claude-sonnet-4-20250514`
**When**: 請求完成
**Then**: `SpendLogs.spend > 0`，且等於 `pricing.Default().TotalCost("claude-sonnet-4-20250514", 100, 50)`

### SC-3: 非 streaming Anthropic 請求的 SpendLogs 同樣正確

**Given**: 一個 Anthropic 非 streaming 請求
**When**: 請求完成
**Then**: `SpendLogs` 的 tokens 和 cost 均 > 0

### SC-4: User-facing alias 不影響 cost 計算

**Given**: Config 中 `ModelName = "claude-sonnet"`, `TianjiParams.Model = "anthropic/claude-sonnet-4-20250514"`
**When**: 透過 `/v1/chat/completions` 發送請求
**Then**: Cost 應基於 `claude-sonnet-4-20250514` 的 pricing 計算，不為 0

### SC-5: 現有 unit test 全部通過

**Given**: 修改完成
**Then**: `go test ./...` 全部通過，包括 `TestParseSSEUsage_*`、`TestNativeProxy_*SpendLog`

### SC-6: 新增整合測試

新增測試覆蓋：
- `spend.Tracker.LogSuccess` 使用 `pricing.Default()` 計算 cost
- Model alias → actual model name 的 pricing lookup
- `spend.Calculator` 空初始化時仍能正確計算 cost（透過 `pricing.Default()` fallback）

---

## 影響範圍

- `cmd/tianji/main.go` — Tracker 初始化
- `internal/spend/tracker.go` — cost 計算 fallback
- `internal/spend/calculator.go` — 可能重構或移除
- `internal/proxy/handler/native_format.go` — 無需修改（邏輯已正確）
- `internal/proxy/handler/chat.go` — 無需修改（邏輯已正確）
- `internal/provider/anthropic/stream.go` — 無需修改（parsing 正確）

## 優先級

**High** — 直接影響 usage tracking 和 budget 計算功能
