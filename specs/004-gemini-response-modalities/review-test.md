# Test Plan Review — 004-gemini-response-modalities

**Reviewer**: 魏徵（QA）
**Date**: 2026-02-26
**Verdict**: ⚠️ 測試計畫有明顯缺口，需補強後才能進入實作

---

## 三個問題

1. **這是真問題還是臆想的？** 真問題。persona-factory 需要 Gemini image output，proxy 目前不支援。
2. **有更簡單的方法嗎？** Plan 只改 4 個檔案、~100 行，已經夠簡潔。
3. **會破壞什麼？** `message.Content` 從 string 變 `[]ContentPart` 可能破壞下游 JSON 反序列化。Plan 有用 `hasImage` 分流，合理。

三個問題通過，進入測試計畫 review。

---

## Plan 提出的 4 個 Test Function

| # | Test Function | 對應 |
|---|--------------|------|
| 1 | `TestTransformResponse_ImageOutput` | US1-AS1, SC-004(b) |
| 2 | `TestTransformResponse_MixedContent` | US1-AS3, SC-004(c) |
| 3 | `TestTransformRequest_WithModalities` | US1-AS2 |
| 4 | `TestTransformResponse_TextOnlyBackwardCompat` | US3-AS1, SC-004(a) |

---

## ✅ 覆蓋完整的部分

- **SC-004 三條路徑**（text-only / image-only / mixed）都有對應 test function
- **US1 三個 acceptance scenario** 全部有對應
- **US3-AS1** backward compatibility 有明確測試
- **回歸測試**：`TextOnlyBackwardCompat` 確認 `message.Content` 維持 plain string
- 現有測試（`TestTransformResponse`、`TestTransformRequest_BasicMessage`）提供額外回歸保護

---

## ⚠️ 建議增加的測試

### 1. `TestTransformRequest_ModalitiesTextOnly`
**對應**: US3-AS2
**原因**: Spec 明確要求 `modalities: ["text"]` 時設 `responseModalities: ["TEXT"]`，但 Plan 只測了 `["text", "image"]`，沒測 text-only modalities。

### 2. `TestTransformResponse_UnknownMimeType`
**對應**: Edge Case — 未知 MIME type
**原因**: Spec 要求 `video/mp4` 等非預期 MIME type 要 passthrough 而非 crash。Plan 完全沒提。

### 3. `TestTransformResponse_ImageOnlyNoText`
**對應**: Edge Case — image-only response
**原因**: Plan 的 `TestTransformResponse_ImageOutput` 描述是 "image-only response"，但沒有明確驗證 `finish_reason` 為 `"stop"`（Spec 要求）。建議獨立測試或在 ImageOutput 中加 assert。

### 4. `TestTransformContentPart_DataURLParsing`
**對應**: Change 2d — image input data URL fix
**原因**: Plan 修了 `transformContentPart` 的 data URL 解析 bug，但**沒有對應的測試**。這是一個 bug fix，必須有測試防止回歸。

### 5. `TestTransformRequest_EmptyModalities`
**對應**: Edge Case — 空 modalities `[]`
**原因**: Spec 要求空 array 視同 text-only（不設 `responseModalities`）。Plan 程式碼用 `len(req.Modalities) > 0` 分流，邏輯正確但沒測試。

---

## ❌ 缺失的關鍵測試

### 1. Streaming image 測試 — `TestParseStreamChunk_InlineData`
**嚴重度**: P0
**對應**: Change 3（stream.go）、Edge Case — streaming image、US1
**原因**: Plan 改了 `ParseStreamChunk` 加入 `inlineData` 處理，但**完全沒有對應的 unit test**。Streaming 是獨立的程式碼路徑，不被非 streaming 測試覆蓋。如果 streaming image 壞了，沒有任何測試會 catch。

### 2. Image input round-trip 測試 — `TestTransformRequest_ImageInput`
**嚴重度**: P1
**對應**: US2-AS1、FR-008
**原因**: US2 是 P2 priority 但 Plan 沒有任何測試覆蓋 image input + image output 的完整 request 轉換。至少需要驗證 image_url 在 request 中正確轉為 Gemini `inlineData`，同時 `responseModalities` 有設定。

---

## 建議的完整測試清單

### Unit Tests（`gemini_test.go`）

| # | Test Function | 覆蓋 | 優先 |
|---|--------------|------|------|
| 1 | `TestTransformResponse_ImageOutput` | US1-AS1, SC-004(b), image-only + finish_reason=stop | P0 |
| 2 | `TestTransformResponse_MixedContent` | US1-AS3, SC-004(c), order preservation | P0 |
| 3 | `TestTransformRequest_WithModalities` | US1-AS2, FR-002 | P0 |
| 4 | `TestTransformResponse_TextOnlyBackwardCompat` | US3-AS1, SC-004(a), FR-007 | P0 |
| 5 | `TestParseStreamChunk_InlineData` | streaming image, Change 3 | **P0** |
| 6 | `TestTransformContentPart_DataURLParsing` | Change 2d bug fix, US2-AS2 MIME preservation | **P0** |
| 7 | `TestTransformRequest_ImageInputWithModalities` | US2-AS1, FR-008 | P1 |
| 8 | `TestTransformRequest_ModalitiesTextOnly` | US3-AS2 | P1 |
| 9 | `TestTransformRequest_EmptyModalities` | Edge: empty `[]` | P1 |
| 10 | `TestTransformResponse_UnknownMimeType` | Edge: `video/mp4` passthrough | P2 |

### E2E Tests

**建議**：暫不需要新增 E2E test。理由：
- 現有 E2E 都是管理面（keys, teams, models），沒有 chat completion E2E 先例
- Image output 需要真實 Gemini API key + 可用的 image model，CI 環境不一定有
- Unit test 如果覆蓋完整（10 個），已足夠驗證轉換邏輯
- 建議在 `quickstart.md` 補充手動 E2E 驗證步驟（Plan 已有此檔案）

---

## 總結

Plan 提出的 4 個 test function 覆蓋了核心 happy path，但有兩個 **P0 缺口**：

1. **Streaming image 完全沒測試** — 改了 `stream.go` 卻沒有對應 test，這是不可接受的
2. **Data URL 解析 bug fix 沒測試** — 修了 bug 不寫 regression test，下次改壞了沒人知道

建議從 4 個 test 擴充到 **至少 8 個**（P0 + P1），理想 10 個（含 P2 edge case）。
