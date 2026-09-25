# E2E Test Plan: Gemini Response Modalities (Image Output)

**Author**: 魏徵（QA）
**Date**: 2026-02-27
**Status**: Draft
**Spec**: `specs/004-gemini-response-modalities/spec.md`
**Implementation Commit**: `8ac4f37`

---

## 測試策略

### 為什麼需要 E2E 測試

Unit tests（14/14 PASS）覆蓋了 `transformToOpenAI`、`transformRequestBody`、`ParseStreamChunk` 等個別函數的邏輯。但 E2E 測試驗證的是**完整的 HTTP 請求-回應鏈路**：

```
Client HTTP Request → proxy.Server → handler → gemini provider → upstream → response transform → Client HTTP Response
```

Unit test 無法覆蓋的風險：
1. JSON marshaling/unmarshaling 在完整 HTTP 鏈路中的行為（`message.Content` 是 `any` 型別，string vs `[]ContentPart` 的 JSON 序列化）
2. `modalities` 欄位能否通過 `knownFields` 檢查不被過濾
3. Streaming SSE 事件格式（`delta.content_parts` 是否正確出現在 SSE payload 中）
4. Request routing（帶 `modalities` 的 request 是否正確路由到 Gemini provider）

### 測試方式：Mock Gemini API Server

**不用真實 Gemini API**，理由：
- CI 環境無 API key
- 真實 API 呼叫不穩定（rate limit、model availability、費用）
- 無法控制 response 內容（無法確定性地驗證 mixed content、image-only 等場景）

**方案：在 test setup 中啟動一個 mock HTTP server**，模擬 Gemini API 的 `/v1beta/models/{model}:generateContent` 和 `:streamGenerateContent` endpoint，根據 request 返回預設的 Gemini response fixture。

這與現有 E2E 架構一致——現有 `setup_test.go` 已經用 `httptest.NewServer` 啟動完整的 Tianji proxy，只需額外啟動一個 mock upstream server 並在 model config 中指向它。

### 測試檔案位置

```
test/e2e/
├── gemini_image_test.go       # E2E test cases
├── gemini_mock_test.go        # Mock Gemini API server
└── helpers_test.go            # (existing) 新增 API call helpers
```

### 不使用 Playwright

現有 E2E 是 Playwright UI 測試。本 feature 是純 API 層改動，無 UI 影響（張大千 review 已確認）。E2E 測試以 **HTTP API call**（`net/http` client）為主，不需要 browser。

---

## Test Infrastructure

### Mock Gemini Server (`gemini_mock_test.go`)

```go
// 需要實作的 mock server
type mockGeminiServer struct {
    // 根據 request 中的 prompt 關鍵字返回不同 fixture
    responses map[string]geminiResponse
}
```

**行為**：
- `POST /v1beta/models/{model}:generateContent` → 返回非 streaming response
- `POST /v1beta/models/{model}:streamGenerateContent` → 返回 SSE streaming response
- 根據 request body 中 `contents[].parts[].text` 的內容決定返回哪個 fixture
- 驗證收到的 request 是否包含 `generationConfig.responseModalities`

**需要的 response fixtures**（JSON）：

| Fixture | 描述 |
|---------|------|
| `gemini-text-only.json` | 純文字 response（一個 text part） |
| `gemini-image-only.json` | 純圖片 response（一個 inlineData part） |
| `gemini-mixed.json` | 混合 response（text + inlineData + text，驗證順序保持） |
| `gemini-unknown-mime.json` | inlineData 但 mimeType 為 `video/mp4`（edge case） |
| `gemini-stream-image.json` | Streaming response with inlineData chunk |

### Setup 擴充（`setup_test.go`）

在 `TestMain` 中：
1. 啟動 `mockGeminiServer` 作為 `httptest.Server`
2. 在 `cfg.ModelList` 中新增一個指向 mock server 的 Gemini model config：
   ```go
   config.ModelConfig{
       ModelName:    "gemini-2-flash-exp",
       TianjiParams: config.TianjiParams{
           Model:   "gemini/gemini-2-flash-exp",
           APIKey:  "fake-key",
           APIBase: mockGeminiServer.URL,
       },
   }
   ```
3. 建立一個用於 API call 的 test key（非 masterKey）

### API Call Helper（`helpers_test.go` 擴充）

```go
// 新增 API-level helpers（不經過 Playwright）
func (f *Fixture) ChatCompletion(req ChatCompletionRequest) (*http.Response, ChatCompletionResponse)
func (f *Fixture) ChatCompletionStream(req ChatCompletionRequest) (*http.Response, []StreamChunk)
```

或者獨立建一個 `APIFixture`（不需要 browser context）：

```go
type APIFixture struct {
    T      *testing.T
    APIKey string  // test API key
}

func apiSetup(t *testing.T) *APIFixture
```

---

## E2E Test Cases

### TC-01: Text-to-Image Generation（非 streaming）

**對應 Spec**: US1-AS1, FR-002, FR-004, FR-005, SC-001
**描述**: 發送帶 `modalities: ["text", "image"]` 的 chat completion request，驗證 response 包含 `image_url` content part。

**步驟**:
1. POST `/v1/chat/completions` with:
   - `model: "gemini-2-flash-exp"`
   - `messages: [{"role": "user", "content": "Draw a cat"}]`
   - `modalities: ["text", "image"]`
   - `stream: false`
2. Mock server 返回 `gemini-image-only.json`

**預期結果**:
- HTTP 200
- `choices[0].message.content` 是 JSON array
- Array 包含至少一個 `{"type": "image_url", "image_url": {"url": "data:image/png;base64,..."}}`
- `choices[0].finish_reason` 為 `"stop"`

**Mock 驗證**:
- Mock server 收到的 request 包含 `generationConfig.responseModalities: ["TEXT", "IMAGE"]`

---

### TC-02: Mixed Content Response（text + image，順序保持）

**對應 Spec**: US1-AS3, FR-006, SC-004(c)
**描述**: Gemini 返回 text + image + text 混合 response，驗證 OpenAI response 保持原始順序。

**步驟**:
1. POST `/v1/chat/completions` with:
   - `modalities: ["text", "image"]`
   - prompt 觸發 mock 返回 `gemini-mixed.json`
2. Mock 返回: `[text("Here is"), inlineData(png), text("a cat")]`

**預期結果**:
- `choices[0].message.content` 是 array，長度 3
- `content[0]`: `{"type": "text", "text": "Here is"}`
- `content[1]`: `{"type": "image_url", "image_url": {"url": "data:image/png;base64,..."}}`
- `content[2]`: `{"type": "text", "text": "a cat"}`
- 順序與 Gemini response 一致

---

### TC-03: Backward Compatibility — Text-Only（無 modalities）

**對應 Spec**: US3-AS1, FR-007, SC-002
**描述**: 不帶 `modalities` 的 text-only request，response 必須維持 plain string（非 array）。

**步驟**:
1. POST `/v1/chat/completions` with:
   - `model: "gemini-2-flash-exp"`
   - `messages: [{"role": "user", "content": "Hello"}]`
   - **無** `modalities` 欄位
2. Mock 返回 `gemini-text-only.json`

**預期結果**:
- HTTP 200
- `choices[0].message.content` 是 plain string（`"Hello, world!"`），**不是** JSON array
- JSON raw value 以 `"` 開頭（string），非 `[`（array）

**Mock 驗證**:
- Mock server 收到的 request **不包含** `generationConfig.responseModalities`

---

### TC-04: Backward Compatibility — `modalities: ["text"]`

**對應 Spec**: US3-AS2, FR-003
**描述**: `modalities: ["text"]`（text-only modalities）不應設定 `responseModalities`。

**步驟**:
1. POST `/v1/chat/completions` with `modalities: ["text"]`
2. Mock 返回 `gemini-text-only.json`

**預期結果**:
- Response `content` 是 plain string

**Mock 驗證**:
- Mock server 收到的 request **不包含** `generationConfig.responseModalities`

---

### TC-05: Image Input + Image Output（Round-Trip）

**對應 Spec**: US2-AS1, US2-AS2, FR-008, SC-003
**描述**: 發送包含 image_url input 的 request，同時要求 image output，驗證 round-trip。

**步驟**:
1. POST `/v1/chat/completions` with:
   - `modalities: ["text", "image"]`
   - `messages[0].content`: array with `{"type": "text", "text": "Edit this image"}` and `{"type": "image_url", "image_url": {"url": "data:image/jpeg;base64,/9j/..."}}`
2. Mock 返回 `gemini-image-only.json`（mimeType: `image/png`）

**預期結果**:
- Response 包含 `image_url` content part
- Data URL 的 MIME type 為 `image/png`（保持 Gemini 返回的 MIME type）

**Mock 驗證**:
- Mock 收到的 request 包含 `generationConfig.responseModalities: ["TEXT", "IMAGE"]`
- Mock 收到的 request 的 `contents[].parts` 包含 `inlineData`（來自 input image）
- Input image 的 `inlineData.data` 是純 base64（不含 `data:` prefix）
- Input image 的 `inlineData.mimeType` 為 `image/jpeg`

---

### TC-06: Streaming Image Response

**對應 Spec**: Edge Case（streaming + image）, Change 3
**描述**: Streaming mode 下 Gemini 返回 inlineData，驗證 SSE event 包含 `content_parts`。

**步驟**:
1. POST `/v1/chat/completions` with:
   - `modalities: ["text", "image"]`
   - `stream: true`
2. Mock server 以 SSE 格式返回 streaming response，其中一個 chunk 包含 `inlineData`

**預期結果**:
- SSE events 格式正確（`data: {...}\n\n`）
- 包含 inlineData 的 chunk：`delta` 物件包含 `content_parts` array
- `content_parts[0]`: `{"type": "image_url", "image_url": {"url": "data:image/png;base64,..."}}`
- 最後一個 event 為 `data: [DONE]`

---

### TC-07: Unknown MIME Type Passthrough

**對應 Spec**: Edge Case（unexpected MIME type）
**描述**: Gemini 返回 `video/mp4` 的 inlineData，proxy 應 passthrough 不 crash。

**步驟**:
1. POST `/v1/chat/completions` with `modalities: ["text", "image"]`
2. Mock 返回 `gemini-unknown-mime.json`（inlineData with `video/mp4`）

**預期結果**:
- HTTP 200（不 crash）
- `content` array 包含 `{"type": "image_url", "image_url": {"url": "data:video/mp4;base64,..."}}`

---

### TC-08: Empty Modalities `[]`

**對應 Spec**: Edge Case（empty modalities array）
**描述**: `modalities: []` 應視同未設定，走 text-only 路徑。

**步驟**:
1. POST `/v1/chat/completions` with `modalities: []`
2. Mock 返回 `gemini-text-only.json`

**預期結果**:
- Response `content` 是 plain string

**Mock 驗證**:
- Mock server 收到的 request **不包含** `generationConfig.responseModalities`

---

## Coverage Matrix

| Spec Requirement | Test Case(s) |
|-----------------|-------------|
| US1-AS1 (image output) | TC-01 |
| US1-AS2 (upstream responseModalities) | TC-01, TC-02 (mock驗證) |
| US1-AS3 (mixed content, order) | TC-02 |
| US2-AS1 (image input + output) | TC-05 |
| US2-AS2 (MIME type preserved) | TC-05 |
| US3-AS1 (no modalities → string) | TC-03 |
| US3-AS2 (text-only modalities) | TC-04 |
| FR-002 (responseModalities mapping) | TC-01, TC-05 |
| FR-003 (text-only no responseModalities) | TC-03, TC-04 |
| FR-006 (mixed content order) | TC-02 |
| FR-007 (text-only backward compat) | TC-03 |
| FR-008 (image input + output) | TC-05 |
| SC-001 (standard SDK works) | TC-01 |
| SC-002 (existing tests pass) | TC-03 |
| SC-003 (round-trip) | TC-05 |
| Edge: streaming image | TC-06 |
| Edge: unknown MIME | TC-07 |
| Edge: empty modalities | TC-08 |

---

## 優先順序

| Priority | Test Cases | 理由 |
|----------|-----------|------|
| **P0** | TC-01, TC-02, TC-03 | 核心功能 + 回歸保護，必須有 |
| **P1** | TC-05, TC-06 | Round-trip 和 streaming 是重要路徑 |
| **P2** | TC-04, TC-07, TC-08 | Edge cases，nice-to-have |

---

## 實作注意事項

1. **Mock server 要驗證 request**：不只返回 fixture，還要 assert 收到的 request 正確（`responseModalities` 有/無）。可以用 channel 或 atomic 記錄 request，test case 結束時驗證。

2. **API Fixture 與 UI Fixture 分離**：現有 `setup(t)` 啟動 browser context，API 測試不需要。建議新增 `apiSetup(t)` 只做 DB clean + API key setup，不啟 Playwright。

3. **Test isolation**：每個 test case 獨立，不依賴其他 test 的狀態。Mock server 的 response routing 用 request body 中的 prompt keyword 區分。

4. **Build tag**：所有新檔案必須包含 `//go:build e2e`。

5. **不需要新的 DB schema**：這是純 proxy 層測試，不涉及 DB 操作（除了 key 驗證）。

6. **Base64 fixture 大小**：mock response 中的 base64 image data 用小型 placeholder（e.g., 1x1 PNG），不需要真實圖片。最小合法 PNG 約 67 bytes base64。
