# UI/UX Review: Gemini Response Modalities (Image Output)

**Reviewer**: 張大千 (UI/UX Designer)
**Date**: 2026-02-26
**Spec**: `specs/004-gemini-response-modalities/spec.md`
**Plan**: `specs/004-gemini-response-modalities/plan.md`

---

## 🎨 UI/UX 影響評估

### 結論：此 feature **不需要立即的 UI 改動**

這個 feature 的改動範圍限定在 API proxy 層（`internal/model/request.go`、`internal/provider/gemini/`），屬於純粹的 request/response 轉換邏輯。`modalities` 是 **per-request 的客戶端參數**，由呼叫者在 API request body 中指定，不是 model-level 的設定。

我 review 了以下管理 UI 頁面：

| 頁面 | 檔案 | 影響 |
|------|------|------|
| Models | `pages/models.templ` | ✅ 無影響 |
| Logs | `pages/logs.templ` | ⚠️ 有 nice-to-have（見下方） |
| Keys | `pages/keys.templ` | ✅ 無影響 |
| Usage | `pages/usage.templ` | ⚠️ 有 nice-to-have（見下方） |

---

## ✅ 不需要 UI 改動的確認

### Models 頁面
- Model 設定（name, provider, model ID, API base, API key, TPM, RPM）與 modalities 無關
- `modalities` 是 client request 參數，不是 model config — **不需要在 model 設定頁新增 modalities 選項**
- Create/Edit Model form 不受影響

### Keys 頁面
- API Key 的 model 限制（`Models []string`）控制的是哪些 model 可以用，不涉及 modalities
- **不需要在 key 設定新增 modalities 相關選項**

---

## ⚠️ 建議的 UI 改動（Future Enhancement，非 blocking）

以下建議 **不在此 feature scope 內**，但值得記錄為 follow-up：

### 1. Logs 頁面 — 顯示 response 是否包含 image

**現狀**：`RequestLogRow` 顯示 tokens、cost、duration，但沒有 content type 資訊。

**建議**：未來若 image generation 使用量增加，可考慮：
- 在 log row 加一個小 icon/badge 標示 response 包含 image（例如 🖼️ badge）
- 或在 filter panel 增加 content type filter（text / image / mixed）

**前提**：需要 DB schema 先記錄 response 是否包含 image data，目前 DB 沒有這個欄位。

**優先級**：Low — 等 image generation 有實際使用量後再評估

### 2. Usage 頁面 — Image generation 用量統計

**現狀**：Usage 頁面追蹤 token 用量和花費。

**建議**：未來可考慮：
- 區分 text vs image generation 的用量
- Image generation 可能有不同的計費方式（per-image vs per-token）

**優先級**：Low — 取決於 Gemini image generation 的計費模型是否與 text 不同

### 3. Models 頁面 — Capabilities 標示（長期）

**建議**：當支援的 modalities 越來越多（text, image, audio），可考慮在 model 列表加 capability badges（如 `📝 Text` `🖼️ Image`），讓管理者快速辨識 model 能力。

**優先級**：Low — 目前 proxy 不管理 model capabilities metadata，這需要更大的架構改動

---

## 使用者體驗注意事項

1. **API 使用者（開發者）面**：此 feature 是 transparent proxy 行為，開發者只要在 request 加 `modalities: ["text", "image"]` 就能用，符合 OpenAI SDK 慣例，**體驗良好**
2. **管理者面**：管理者不需要做任何設定就能讓此 feature 生效，**零設定負擔**，這是好的設計
3. **Backward compatibility**：不送 `modalities` 的 request 行為完全不變，**無破壞性**

---

## 總結

| 項目 | 結論 |
|------|------|
| 此 PR 需要 UI 改動？ | **否** |
| 有 blocking UI issue？ | **否** |
| 有 follow-up 建議？ | **是**（3 個 nice-to-have，均為 Low priority） |
| 整體 UI/UX 影響 | **Minimal** — 純 API 層改動，管理 UI 不受影響 |

**Review 結果：✅ LGTM（無 UI blocking issue）**
