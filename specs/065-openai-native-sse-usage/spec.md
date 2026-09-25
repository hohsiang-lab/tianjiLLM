# HO-65: Add OpenAI Native SSE Usage Parsing

## 問題描述

`parseSSEUsage`（`internal/proxy/handler/native_format.go`）的 `switch providerName` 只處理 `"anthropic"` 和 `"gemini"`，缺少 `"openai"` case。當 provider 為 OpenAI 時，streaming response 的 token usage 資料被完全忽略，導致 log 中 prompt/completion token 計數為 0。

### OpenAI Streaming Usage 格式

OpenAI streaming 在請求帶 `stream_options: {"include_usage": true}` 時，**最後一個 chunk** 會包含：

```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion.chunk",
  "model": "gpt-4o",
  "choices": [],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 25,
    "total_tokens": 35
  }
}
```

特徵：`choices` 為空陣列，`usage` 欄位出現在 root level。

## 功能需求（FR）

1. **FR-1**：在 `parseSSEUsage` 的 switch 新增 `case "openai"` 分支
2. **FR-2**：解析每個 SSE data chunk，擷取：
   - `model` 欄位 → `modelName`（取最後出現的非空值）
   - `usage.prompt_tokens` → `prompt`
   - `usage.completion_tokens` → `completion`
3. **FR-3**：結構定義應與 OpenAI API 回傳格式一致：
   ```go
   var event struct {
       Model string `json:"model"`
       Usage struct {
           PromptTokens     int `json:"prompt_tokens"`
           CompletionTokens int `json:"completion_tokens"`
       } `json:"usage"`
   }
   ```

## 驗收標準（SC）

| # | 標準 | 驗證方式 |
|---|------|---------|
| SC-1 | `parseSSEUsage("openai", rawSSE)` 正確回傳 prompt tokens、completion tokens、model name | Unit test |
| SC-2 | 當 SSE 中無 usage chunk 時，回傳 `(0, 0, "")` 不 panic | Unit test |
| SC-3 | 多個 chunk 時取最後出現的 usage 值（與 gemini case 行為一致） | Unit test |
| SC-4 | model name 從任意包含 `model` 欄位的 chunk 擷取 | Unit test |
| SC-5 | 既有 anthropic/gemini test 不受影響 | `go test ./internal/proxy/handler/...` |
