# HO-67: Fix Request Logs UI — Cost / Tokens 欄位顯示 "–"

## 問題

Request Logs UI 的 Cost 和 Tokens 欄位全部顯示 `–`（en-dash），但使用者回報 DB 有正確的 spend（~$0.04）和 tokens（~70K）。

## 目標

讓 Request Logs UI 正確顯示每筆 request 的 Cost、Tokens（含 prompt + completion breakdown）。

## 程式碼分析

### 資料流（完整鏈路）

```
SpendLogs DB table
  → ListRequestLogs SQL query (internal/db/queries/spend_views.sql)
  → ListRequestLogsRow struct (internal/db/spend_views.sql.go)
  → toLogRow() mapping (internal/ui/handler_logs.go)
  → RequestLogRow struct (internal/ui/pages/logs.templ)
  → logRow() templ rendering (internal/ui/pages/logs.templ)
```

### 各層檢查結果

| 層級 | 狀態 | 說明 |
|------|------|------|
| **SQL query** | ✅ 正確 | `SELECT sl.spend, sl.total_tokens, sl.prompt_tokens, sl.completion_tokens` 都有選 |
| **Go scan** | ✅ 正確 | `ListRequestLogsRow.Spend` (float64), `.TotalTokens` (int32) 正確 scan |
| **toLogRow mapping** | ✅ 正確 | `row.Spend → lr.Spend`, `int(row.TotalTokens) → lr.TotalTokens` |
| **Templ rendering** | ✅ 正確 | `if row.Spend > 0` → 顯示 `$%.4f`，否則顯示 `–` |
| **Compiled templ** | ✅ 正確 | `logs_templ.go` 與 `logs.templ` 一致 |

### 結論：程式碼鏈路正確，問題在資料寫入端

所有 render 邏輯都正確。如果 UI 顯示 `–`，那 **DB 中該筆 row 的 `spend` 和 `total_tokens` 就是 0**。

## Root Cause

### 最可能原因：Native Proxy 的 `parseUsage()` 不支援 OpenAI 格式

**檔案**：`internal/proxy/handler/native_format.go`

```go
func parseUsage(providerName string, body []byte) (prompt, completion int, modelName string) {
    switch providerName {
    case "anthropic":
        // ✅ 有處理
    case "gemini":
        // ✅ 有處理
    }
    return 0, 0, "" // ← OpenAI、OpenRouter 等全部 fallthrough 回傳 0
}
```

**影響**：透過 Native Proxy（reverse proxy 模式）的非 streaming OpenAI 請求，`parseUsage()` 回傳 0 tokens → `calculateCost()` 拿到 0 tokens → spend = 0 → DB 寫入 `spend=0, total_tokens=0`。

**注意**：streaming 路徑的 `parseSSEUsage()` **有**處理 OpenAI 格式（有 `case "openai":`），所以 streaming 請求不受影響。

### 兩條 Proxy 路徑差異

| 路徑 | 檔案 | 非 streaming | Streaming |
|------|------|-------------|-----------|
| **Chat Handler**（Go parsed） | `chat.go` | ✅ 從 `result.Usage` 讀取 | ✅ 從 `accUsage` / `lastChunk.Usage` 讀取 |
| **Native Format**（reverse proxy） | `native_format.go` | ❌ `parseUsage()` 缺 OpenAI case | ✅ `parseSSEUsage()` 有 OpenAI case |

### 次要可能：使用者查看的是聚合數據

使用者可能透過 Dashboard 的 Usage Metrics（`GetUsageMetrics` / `GetDailySpendByModel`）看到非零 spend，但這些是 **SUM 聚合**。個別 SpendLogs row 可能大多是 0，只有少數透過 Chat Handler 路徑的有值。

## 修復方案

### Fix 1：補上 `parseUsage()` 的 OpenAI case（必要）

**檔案**：`internal/proxy/handler/native_format.go`

```go
func parseUsage(providerName string, body []byte) (prompt, completion int, modelName string) {
    switch providerName {
    case "anthropic":
        // 現有 code...
    case "gemini":
        // 現有 code...
    case "openai", "openrouter", "deepseek", "groq", "together":
        // OpenAI-compatible format
        var parsed struct {
            Model string `json:"model"`
            Usage struct {
                PromptTokens     int `json:"prompt_tokens"`
                CompletionTokens int `json:"completion_tokens"`
            } `json:"usage"`
        }
        if json.Unmarshal(body, &parsed) == nil {
            return parsed.Usage.PromptTokens, parsed.Usage.CompletionTokens, parsed.Model
        }
    }
    return 0, 0, ""
}
```

### Fix 2：加 OpenAI-compatible fallback（建議）

在 `parseUsage()` 的 default case 加上 OpenAI 格式 fallback，處理未知 provider 但使用 OpenAI 格式的情況：

```go
default:
    // Fallback: try OpenAI-compatible format
    var parsed struct {
        Model string `json:"model"`
        Usage struct {
            PromptTokens     int `json:"prompt_tokens"`
            CompletionTokens int `json:"completion_tokens"`
        } `json:"usage"`
    }
    if json.Unmarshal(body, &parsed) == nil && (parsed.Usage.PromptTokens > 0 || parsed.Usage.CompletionTokens > 0) {
        return parsed.Usage.PromptTokens, parsed.Usage.CompletionTokens, parsed.Model
    }
```

### Fix 3：驗證修復（e2e test 補強）

`test/e2e/logs_list_test.go` 的 `TestLogsList_ShowsLogs` 目前沒有驗證 cost/tokens 的 render。需要補上：

```go
func TestLogsList_ShowsLogs(t *testing.T) {
    f := setup(t)
    f.SeedSpendLog(SeedSpendLogOpts{Model: "openai/gpt-4o", Spend: 0.05, Tokens: 150, Prompt: 100, Completion: 50})
    f.NavigateToLogs()

    body := f.Text("#logs-table")
    assert.Contains(t, body, "$0.0500")   // ← 新增
    assert.Contains(t, body, "150")        // ← 新增 total tokens
    assert.Contains(t, body, "(100+50)")   // ← 新增 prompt+completion
}
```

## 驗收條件

1. [ ] `parseUsage()` 支援 OpenAI-compatible 格式（含 default fallback）
2. [ ] 非 streaming OpenAI 請求的 SpendLogs row 有正確 spend / tokens
3. [ ] Request Logs UI 的 Cost 欄位顯示 `$X.XXXX`（非零時）
4. [ ] Request Logs UI 的 Tokens 欄位顯示 `N (P+C)`（非零時）
5. [ ] E2E test `TestLogsList_ShowsLogs` 驗證 cost/tokens render
6. [ ] 既有 E2E tests 全部通過

## 影響範圍

- **修改檔案**：`internal/proxy/handler/native_format.go`
- **測試檔案**：`test/e2e/logs_list_test.go`
- **不影響**：Chat Handler 路徑、DB schema、templ 模板

## 優先級

**High** — 使用者無法在 UI 看到 cost/tokens 數據，影響基本的 observability 功能。

## 備注

- 如果部署環境只用 Chat Handler 路徑（不走 Native Proxy），則此 bug 可能是**已有資料的 spend 在寫入時就是 0**（例如 pricing table 缺少該 model）。建議先在 production DB 直接查：`SELECT request_id, spend, total_tokens FROM "SpendLogs" ORDER BY starttime DESC LIMIT 10;` 確認個別 row 的值。
- `parseSSEUsage()` 也建議加上 default fallback（同 Fix 2 的 pattern），但目前 streaming 路徑已有 OpenAI case，優先級較低。
