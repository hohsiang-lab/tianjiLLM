# Research: UI Virtual Keys Management

**Feature**: 008-ui-virtual-keys
**Date**: 2026-02-20

## 1. `/key/list` API 擴充：伺服器端過濾/分頁/排序

### Decision
擴充現有 Go `KeyList` handler 和 sqlc 查詢，對齊 Python LiteLLM 的 `/key/list` endpoint 參數。

### Rationale
- Python LiteLLM `/key/list` 接受：`page`, `size`, `team_id`, `key_alias`, `user_id`, `key_hash`, `sort_by`, `sort_order`
- 當前 Go 實現：hardcoded `Limit: 1000, Offset: 0`，無過濾、無排序
- UI 過濾器需要伺服器端支援（客戶端過濾在數據量大時失效）
- 需要新增帶 WHERE 條件的 sqlc 查詢 + COUNT 查詢（準確分頁）

### Python 參考實現
```
GET /key/list?page=1&size=50&team_id=xxx&key_alias=xxx&user_id=xxx&key_hash=xxx&sort_by=created_at&sort_order=desc
```
Filter 條件建構：`_build_key_filter_conditions()` — 精確匹配 team_id/user_id/key_hash/key_alias，OR 組合
排序驗證：`_validate_sort_params()` — 白名單驗證 sort column
回應格式：`{keys: [], total_count: int, current_page: int, total_pages: int}`

### sqlc 實現方案

**問題 1**：sqlc 不支援動態 WHERE 子句。

**已驗證的陷阱**：`($1::text IS NULL OR team_id = $1)` 模式中，sqlc 將 `$1` 生成為 `string`（非 `*string`），即使配置了 `emit_pointers_for_null_types: true`。空字符串 `""` ≠ SQL NULL，因此 `$1::text IS NULL` 永遠為 false，過濾條件變成必填。已在 `spend_views.sql` 的 `GetSpendLogsByFilterParams` 中確認此問題（`Column3 string`）。

**正確方案**：使用 `sqlc.narg()` 宏標記可選參數為 nullable，生成 `*string` 類型：

```sql
-- name: ListVerificationTokensFiltered :many
SELECT * FROM "VerificationToken"
WHERE
  (sqlc.narg(filter_team_id)::text IS NULL OR team_id = sqlc.narg(filter_team_id)) AND
  (sqlc.narg(filter_key_alias)::text IS NULL OR key_alias = sqlc.narg(filter_key_alias)) AND
  (sqlc.narg(filter_user_id)::text IS NULL OR user_id = sqlc.narg(filter_user_id)) AND
  (sqlc.narg(filter_token)::text IS NULL OR token = sqlc.narg(filter_token))
ORDER BY created_at DESC
LIMIT sqlc.arg(query_limit) OFFSET sqlc.arg(query_offset);
```

這會生成：
```go
type ListVerificationTokensFilteredParams struct {
    FilterTeamID   *string `json:"filter_team_id"`
    FilterKeyAlias *string `json:"filter_key_alias"`
    FilterUserID   *string `json:"filter_user_id"`
    FilterToken    *string `json:"filter_token"`
    QueryLimit     int32   `json:"query_limit"`
    QueryOffset    int32   `json:"query_offset"`
}
```

Go handler 傳 `nil` 表示「不過濾」，傳 `&"value"` 表示精確匹配。

**問題 2**：排序無法動態化（sqlc 不支援動態 ORDER BY）。

**實際方案（遵守 Constitution VII）**：
- 預設排序固定為 `created_at DESC`（這是最常用場景，Python 也是默認 `created_at desc`）
- UI 的排序功能在第一版中限制為客戶端排序（在當前頁內排序）
- 伺服器端排序作為後續優化（需要 Constitution 豁免或使用 raw query）
- 過濾 + 分頁 + COUNT 完全在 SQL 層

### Alternatives Considered
1. **pgx raw query**：動態建構 SQL 支援所有過濾+排序，但違反 Constitution VII
2. **多個 sqlc 查詢組合**：為每種 sort_by 寫查詢，6 個排序欄位 × 2 個方向 = 12 個查詢，太冗餘
3. **CASE WHEN 動態 ORDER BY**：`ORDER BY CASE WHEN $7 = 'spend' THEN spend END` — sqlc 可能不支援，且 PostgreSQL 效能不佳

### Source
- Python: `litellm/proxy/management_endpoints/key_management_endpoints.py:3695-4132`
- Go current: `internal/proxy/handler/key.go:127-147`
- Go sqlc: `internal/db/queries/verification_token.sql`

---

## 2. templ + HTMX 架構模式

### Decision
使用現有 templ + HTMX + templUI 技術棧，遵循以下模式：

1. **分離路由服務全頁和 HTMX 局部**：全頁用 `/keys`，HTMX 局部用 `/keys/table`（沿用現有模式）
2. **OOB swap 實現 Toast**：操作成功/失敗時通過 `hx-swap-oob` 追加 toast 到 `<body>`（toast 組件自帶 `fixed` 定位）
3. **HX-Push-Url 維護瀏覽器 URL**：過濾、分頁、排序狀態反映在 URL 中
4. **獨立 URL 路由的詳情頁**：`/ui/keys/{id}` 整頁導航

### Rationale
- GitHub 搜索確認 `hx-swap-oob` 是 HTMX toast 的標準模式（`foks-proj/go-foks`, `depado/pb-templ-htmx-tailwind`）
- `HX-Push-Url` 讓使用者可以書籤/分享過濾後的列表視圖
- `angelofallars/htmx-go` 提供類型安全的 HTMX header 操作（但為避免新依賴，直接用 `r.Header.Get("HX-Request")` 即可）
- 詳情頁用獨立 URL 而非 SPA 替換，與 Go server-rendered 模式一致

### Key Patterns

**Toast OOB Swap**:

Toast 組件（`internal/ui/components/toast/toast.templ`）自帶 `fixed` 定位和 `z-50`，
通過 `data-tui-toast` + `toast.min.js` 自動管理位置和消失。
不需要容器元素。使用 OOB swap 追加到 body：

```templ
templ KeysTableWithToast(data KeysPageData, toastMsg string, toastVariant string) {
    @KeysTablePartial(data)
    if toastMsg != "" {
        <div id="toast-oob" hx-swap-oob="afterbegin:body">
            @toast.Toast(toast.Props{
                Title:       toastMsg,
                Variant:     toast.Variant(toastVariant),
                Dismissible: true,
                Duration:    3000,
            })
        </div>
    }
}
```

**注意**：現有頁面有 `<div id="toast-container">` 但實際上 toast 組件不使用它。
保留以向後兼容但新 toast 直接追加到 body。

**Copy-to-Clipboard**:
```templ
script copyToClipboard(text string) {
    navigator.clipboard.writeText(text).then(() => {
        const btn = event.currentTarget;
        const orig = btn.innerHTML;
        btn.innerHTML = 'Copied!';
        setTimeout(() => btn.innerHTML = orig, 1500);
    });
}
```

**Create Key — 一次性明文顯示**:
Handler 生成 key 後，response 返回帶 key 明文的「Save your Key」dialog HTML（OOB swap 到 dialog container）。

### Alternatives Considered
1. **HX-Trigger + client-side toast listener**：更解耦但需要額外 JS event handler
2. **React/Next.js 前端**：功能更強但完全重寫現有 UI 架構
3. **htmx-go 庫**：類型安全但引入新依賴，`r.Header.Get()` 已足夠

### Source
- GitHub: `foks-proj/go-foks`, `depado/pb-templ-htmx-tailwind`, `gofs-cli/gofs`
- Context7: templ component patterns, HTMX response headers
- Existing: `internal/ui/handler_keys.go`, `internal/ui/pages/keys.templ`

---

## 3. 建立 Key 後明文顯示

### Decision
`handleKeyCreate` 生成 key 後，將明文 key 保存在 response 中，通過 OOB swap 注入「Save your Key」dialog。

### Rationale
- **現有問題**：`handleKeyCreate` 生成明文 → SHA256 hash 存 DB → 明文丟失 → 使用者永遠無法看到 key
- **Python 做法**：`/key/generate` response 返回 `{key: "sk-xxx", ...}`，前端用 Modal 顯示
- **Go 方案**：handler 在 hash 前保存明文，response 中通過 OOB swap 顯示帶明文的 dialog + 複製按鈕

### Implementation
```go
func (h *UIHandler) handleKeyCreate(w http.ResponseWriter, r *http.Request) {
    rawKey := generateAPIKey()  // "sk-" + hex(24 bytes)
    hashedKey := hashKey(rawKey)
    // ... DB insert with hashedKey ...

    // Response: table partial + OOB dialog with rawKey
    render(r.Context(), w, pages.KeysTableWithKeyReveal(data, rawKey))
}
```

### Alternatives Considered
1. **Store raw key in session/cookie temporarily**：安全風險
2. **Redirect to a one-time URL**：過度工程化
3. **Return rawKey as response header**：HTMX 無法直接讀取自定義 header

### Source
- Python: `internal/proxy/handler/key.go:17-94` (KeyGenerateHandler)
- Python UI: `create_key_button.tsx` → separate Modal after creation

---

## 4. 詳情頁 Tab 切換

### Decision
使用現有 templUI `tabs` 組件實現 Overview / Settings 雙 tab，純客戶端切換（data-tui-tabs JS 驅動，無需 HTMX round-trip）。

### Rationale
- templUI 已有 `tabs.Tabs`, `tabs.List`, `tabs.Trigger`, `tabs.Content` 組件
- Tab 切換是即時的 UI 操作，不涉及新數據載入，用客戶端 JS 切換即可
- HTMX tab 切換會增加不必要的 server round-trip

### Source
- Existing: `internal/ui/components/tabs/tabs.go`
- Existing usage: `internal/ui/pages/spend.templ` (已有 tabs 用例)

---

## 5. 編輯模式（Settings tab → Edit 切換）

### Decision
Settings tab 的「Edit Settings」按鈕觸發 HTMX 請求，將 Settings 內容替換為編輯表單。Save/Cancel 後替換回查看模式。

### Rationale
- 與 LiteLLM Python UI 的行為一致（KeyEditView 替換 KeyInfoView）
- HTMX `hx-get="/ui/keys/{id}/edit"` 獲取編輯表單片段
- `hx-post="/ui/keys/{id}/update"` 提交後返回查看模式片段
- 無需 dialog/modal，直接 in-place 替換更直觀

### Source
- Python UI: `key_edit_view.tsx` — vertical form layout, sticky bottom bar

---

## 6. 刪除確認（輸入 alias）

### Decision
使用 templUI `dialog` 組件實現刪除確認對話框，包含 key 資訊展示和 alias 輸入驗證。

### Rationale
- LiteLLM Python UI 使用 `DeleteResourceModal` + `requiredConfirmation` 模式
- 輸入驗證在客戶端完成（JS 比較輸入值與目標 alias），無需 server round-trip
- 已有的 `hx-confirm` 瀏覽器原生 confirm 不夠安全

### Implementation
使用 templ `script` 指令生成客戶端驗證 JS：
```templ
script validateDeleteConfirm(expectedAlias string) {
    const input = document.getElementById('delete-confirm-input');
    const btn = document.getElementById('delete-confirm-btn');
    btn.disabled = input.value !== expectedAlias;
}
```

### Source
- Python UI: `DeleteResourceModal` in key_info_view.tsx

---

## 7. Key Regenerate 對話框

### Decision
Regenerate 使用 templUI `dialog`，預填可修改欄位（budget/tpm/rpm/duration），成功後切換為明文 key 展示模式。

### Rationale
- LiteLLM Python UI: `RegenerateKeyModal` 分兩階段（輸入 → 展示新 key）
- 使用 HTMX `hx-post="/ui/keys/{token}/regenerate"` 提交
- Response 替換 dialog 內容為新 key 展示（OOB swap）

### 已驗證的限制

**P0 問題**：現有 `RegenerateVerificationToken` 查詢只接受 2 個參數（old token, new token），無法同時修改 budget/tpm/rpm/duration。

```sql
-- 現有查詢（不足）:
-- name: RegenerateVerificationToken :one
UPDATE "VerificationToken"
SET token = $2, spend = 0, updated_at = NOW()
WHERE token = $1
RETURNING *;
```

**解決方案**：新增 `RegenerateVerificationTokenWithParams` 查詢，同時更新 token + 可選屬性：

```sql
-- name: RegenerateVerificationTokenWithParams :one
UPDATE "VerificationToken"
SET
    token = sqlc.arg(new_token),
    spend = 0,
    max_budget = COALESCE(sqlc.narg(new_max_budget), max_budget),
    tpm_limit = COALESCE(sqlc.narg(new_tpm_limit), tpm_limit),
    rpm_limit = COALESCE(sqlc.narg(new_rpm_limit), rpm_limit),
    budget_duration = COALESCE(sqlc.narg(new_budget_duration), budget_duration),
    updated_at = NOW()
WHERE token = sqlc.arg(old_token)
RETURNING *;
```

### Source
- Python UI: `regenerate_key_modal.tsx`
- Go API: `internal/proxy/handler/key_ext.go:1-42` (KeyRegenerate)

---

## 8. 路由模式（分離路由 vs HX-Request 檢測）

### Decision
沿用現有分離路由模式：全頁渲染用 `/keys`，HTMX 局部更新用 `/keys/table`。不使用 `HX-Request` header 檢測。

### Rationale
- 現有程式碼使用分離路由（`handleKeys` vs `handleKeysTable`），而非在同一 handler 中檢測 `HX-Request`
- `routes.go` 已有 `r.Get("/keys", h.handleKeys)` + `r.Get("/keys/table", h.handleKeysTable)` 模式
- 保持一致性，新路由同樣分離：
  - `/keys/{token}` — 詳情頁全頁
  - `/keys/{token}/settings` — Settings tab 內容（HTMX partial）
  - `/keys/{token}/edit` — 編輯表單（HTMX partial）
  - `/keys/{token}/update` — 更新提交（POST）
  - `/keys/{token}/delete` — 刪除（POST）
  - `/keys/{token}/regenerate` — 重新產生（POST）

### Source
- Existing: `internal/ui/routes.go:31-36`

---

## 9. 現有 KeyRow/KeysPageData 結構差異

### Decision
擴充現有結構而非重新定義，保持向後兼容。

### 已驗證的差異

| 字段 | 現有 keys.templ | plan.md 提議 | 修正 |
|------|----------------|-------------|------|
| KeyRow.KeyName | `string` | `*string` | 保持 `string`（handler 已做 nil 轉換） |
| KeyRow.KeyAlias | `string` | `*string` | 保持 `string` |
| KeyRow.Blocked | `bool` | `*bool` | 保持 `bool` |
| KeysPageData.Search | `string` | 缺失 | 保留（向後兼容） |
| keysPerPage | 20 | 50 | 改為 50（對齊 Python） |

### Rationale
現有 handler（`loadKeysPageData`）已將 DB 的 nullable 指標類型轉換為 templ 的值類型。
這是有意為之的「邊界轉換」模式——DB 層用指標表示 nullable，UI 層用零值表示缺失。
新增字段遵循同樣模式。

### Source
- Existing: `internal/ui/pages/keys.templ:16-32`, `internal/ui/handler_keys.go:58-94`

---

## 10. Team Alias 顯示需要額外查詢

### Decision
新增 `ListTeams` sqlc 查詢（或擴充 JOIN），在 handler 層將 team_id → team_alias 映射注入 KeyRow。

### Rationale
- 現有 `ListVerificationTokens` 不 JOIN TeamTable
- 列表頁需顯示 Team Alias 欄位
- 兩種方案：
  1. **JOIN 查詢**：在 `ListVerificationTokensFiltered` 中 LEFT JOIN TeamTable — sqlc 可能生成不乾淨的類型
  2. **分離查詢**：先查 tokens，再查 `ListTeamsForIDs` 取 team_id→alias 映射 — 更簡單

**選擇方案 2**（分離查詢）：
```sql
-- name: ListTeamAliases :many
SELECT team_id, team_alias FROM "TeamTable"
WHERE team_id = ANY(sqlc.arg(team_ids)::text[]);
```

Handler 中：收集所有 unique team_ids，批量查詢，構建 map[string]string，注入 KeyRow.TeamAlias。

### Source
- Existing: `internal/db/schema/001_initial.sql` (TeamTable schema)
