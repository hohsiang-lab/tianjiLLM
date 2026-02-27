# QA Review: User Management UI (HO-19) — PR #24

**Reviewer**: 魏徵（QA）
**Date**: 2026-02-27
**Status**: 🟡 Conditional PASS（有 2 個 P1 需修，無 P0 擋上線問題）

---

## 三個問題先過

1. **這是真問題還是臆想的？** — User Management 是 spec 明確要求的功能，是真需求。
2. **有更簡單的方法嗎？** — 實作沿用 Teams/Orgs pattern，合理。DB-level pagination 是正確的改進方向。手寫 query 而非 sqlc 生成略有風險但可接受。
3. **會破壞什麼？** — 新增的 `requireAdmin` middleware 只套用在 `/users` routes，不影響現有頁面。新 query 是 additive，不修改現有 query。低風險。

---

## PM Review 11 個 Issue 逐條確認

| # | PM Issue | 狀態 | 說明 |
|---|---------|------|------|
| 1 | RBAC 沒有實作方案 | ✅ 已解決 | `requireAdmin` middleware 在 `handler.go:91-100` 實作，從 session cookie 讀 `Role` 欄位，檢查 `== "admin"`。`routes.go` 用 `r.Group` + `r.Use(h.requireAdmin)` 包住所有 `/users` routes。 |
| 2 | Soft delete 不過濾現有 query | ✅ 已解決 | `ListUsersPaginated` 和 `CountUsers` 都有 `AND COALESCE(metadata->>'status', 'active') != 'deleted'`。`GetUser`/`GetUserByEmail`（sqlc 生成）未修改，但 `loadUserDetailData` 在 Go 層檢查 `status == "deleted"` 並返回 false。 |
| 3 | metadata.status 無初始值 | ✅ 已解決 | 所有 query 用 `COALESCE(metadata->>'status', 'active')`，Go 層 `userStatusFromMetadata()` 對 nil/empty/無 status key 都返回 `"active"`。 |
| 4 | blocked vs metadata.status 不一致 | ⚠️ 未說明 | Plan 沒有解釋為什麼選 `metadata.status` 而非 `blocked` 欄位。**不阻擋上線**，但應補文件說明。 |
| 5 | DB vs in-memory pagination 不一致 | ✅ 刻意改進 | Users 用 SQL `LIMIT/OFFSET`，是更好的做法。 |
| 6 | 缺 filter scenario | ⚠️ Spec 未更新 | 但實作已有 role + status filter（UI select + query 參數都有）。功能已做，spec 文件未補。 |
| 7 | CountUsers 缺 status 過濾 | ✅ 已解決 | `CountUsers` 有 `Search`, `RoleFilter`, `StatusFilter` 三個參數，WHERE 條件與 `ListUsersPaginated` 完全一致。 |
| 8 | Last admin 保護 | ✅ 已解決 | `handleUserBlock`、`handleUserDelete`、`handleUserUpdate` 三處都有 `CountUsersByRole("proxy_admin")` 檢查。 |
| 9 | Per-model spend breakdown | ❌ 未實作 | Detail 頁只顯示 total spend，沒有 per-model breakdown。Spec FR-006 要求 "spend summary"，但 acceptance scenario 3 明確要求 "per-model breakdown"。 |
| 10 | Auth 方式未說明 | ⚠️ 未處理 | Create User 後，user 如何登入仍不清楚。不影響功能，但應在 spec 或 UI 上說明。 |
| 11 | RBAC 測試 | ❌ 未補 | 目前只有 `TestUserStatusFromMetadata`，沒有 handler 層測試。詳見下方測試覆蓋率分析。 |

---

## QA 重點逐項檢查

### 1. RBAC — ✅ PASS

- `requireAdmin` middleware（`handler.go:91-100`）：從 HMAC-signed session cookie 解析 `sessionPayload.Role`，檢查 `== "admin"`。
- Session 設置（`handler.go:80`）：`authenticateKey` 只對 master key 返回 `"admin"`，其他 key 返回 `"", false` → 只有 master key holder 能登入且角色為 admin。
- 所有 `/users` routes 都在 `requireAdmin` group 內。
- HTMX 請求也有處理（返回 403 而非 redirect）。
- **注意**：目前只有 master key = admin 這一種認證方式，沒有 non-admin 用戶能登入 UI。這意味著 `requireAdmin` 在當前架構下其實是多餘的保護（但作為防禦性設計是正確的）。

### 2. Soft Delete — ✅ PASS

- `SoftDeleteUser` query 用 `jsonb_set(COALESCE(metadata, '{}'::jsonb), '{status}', '"deleted"')`，正確處理 metadata 為空的情況。
- `ListUsersPaginated` 和 `CountUsers` 都排除 deleted。
- `loadUserDetailData` 在 Go 層額外檢查 deleted status，防止直接 URL 存取。
- `handleUserCreate` 檢查 email 唯一性時，用 `userStatusFromMetadata` 判斷現有記錄是否為 deleted，允許重用被刪用戶的 email。✅ 正確。

### 3. Last Admin 保護 — ✅ PASS（有一個 P2 建議）

- Block/Delete/Update 三處都有保護。
- `CountUsersByRole` query 也正確排除 deleted users。
- **P2 建議**：`handleUserBlock` 在 last admin 保護觸發且 `return_to == "detail"` 時用 `http.Redirect` 而非 toast，用戶不會看到錯誤訊息。其他路徑（非 detail）有正確的 toast 提示。

### 4. SQL Injection — ✅ PASS

- 所有 query 都用 parameterized query（`$1`, `$2`, `$3` etc.）。
- `ILIKE '%' || $1 || '%'` 是在 SQL 層做字串拼接，`$1` 仍是 parameterized，**不構成 SQL injection**。
- 但 `ILIKE` 的 `%` 和 `_` 是 pattern 字元——用戶輸入 `%` 或 `_` 不會被 escape。這是 **P2 功能問題**（搜尋結果不精確），非安全問題。

### 5. HTMX Pattern — ✅ PASS

- 跟 Teams/Orgs 一致：search 用 `hx-get` + `delay:300ms`，table partial swap，toast 用 OOB swap。
- Pagination 用 HTMX partial update（`hx-get` + `hx-target="#users-table"`）。
- Delete 用 `HX-Redirect` header。
- Detail 頁的 block/unblock 用 `<form method="POST">` + `return_to=detail` 做 full page redirect（非 HTMX），與 list 頁的 HTMX inline 操作不一致但功能正確。

### 6. Edge Cases

| Case | 狀態 |
|------|------|
| 空列表 | ✅ 有 "No users found" empty state |
| 重複 email | ✅ 有檢查，且正確處理 soft-deleted 用戶的 email 重用 |
| 無效 input（空 email）| ✅ 有 server-side 驗證 + client-side `required` |
| Last admin | ✅ 三處保護 |
| 直接 URL 存取 deleted user | ✅ `loadUserDetailData` 檢查 → redirect |

### 7. Test 覆蓋率 — 🟡 不足

目前只有 `TestUserStatusFromMetadata`（10 個 case），覆蓋一個 utility function。

**缺少的關鍵測試**：

| 優先級 | 測試 | 原因 |
|--------|------|------|
| P1 | `TestRequireAdmin_Forbidden` | RBAC 是安全功能，必須有測試 |
| P1 | `TestHandleUserDelete_LastAdmin` | Last admin 保護是關鍵業務規則 |
| P2 | `TestHandleUserCreate_DuplicateEmail` | 資料完整性 |
| P2 | `TestHandleUserBlock_LastAdmin` | 同上 |
| P2 | `TestHandleUserUpdate_RoleChange_LastAdmin` | 同上 |
| P3 | `TestLoadUsersPageData_Pagination` | 分頁邏輯 |
| P3 | `TestUserTableRowFromDB_NilFields` | Nil pointer 防護 |

---

## Bug Report

### Bug #1 — `handleUserBlock` last admin 保護在 detail 頁無錯誤提示

**嚴重度**：P2
**重現步驟**：
1. 只有一個 admin user
2. 從 detail 頁點 "Disable"（`return_to=detail`）
3. Last admin 保護觸發

**預期結果**：顯示錯誤 toast「Cannot disable the last admin user」
**實際結果**：用 `http.Redirect` 回 detail 頁，沒有任何錯誤提示，用戶不知道為什麼操作沒生效。
**位置**：`handler_users.go:131-134`

### Bug #2 — `CreatedBy` / `UpdatedBy` 寫死 "admin" 而非實際 session user

**嚴重度**：P1
**重現步驟**：查看所有 handler 中的 `CreatedBy: "admin"` 和 `UpdatedBy: "admin"`
**預期結果**：應記錄實際執行操作的用戶 ID（從 session 取得）
**實際結果**：全部寫死 `"admin"` 字串，無法追蹤是哪個 admin 做了操作。
**位置**：`handler_users.go:123,148,174,207` 和 `handler_users_detail.go:99`
**備註**：目前 `sessionPayload.UserID` 在 `setSessionCookie` 時傳空字串（`handler.go:80`），需要先修 login flow 才能正確記錄。這是架構層面的問題，影響所有 audit trail。列為 P1 因為是 audit/compliance 需求。

### Bug #3 — Per-model spend breakdown 未實作

**嚴重度**：P1
**重現步驟**：打開任一 user 的 detail 頁
**預期結果**：根據 spec acceptance scenario 3，應顯示 per-model spend breakdown
**實際結果**：只顯示 total spend 數字，無 per-model 資訊
**位置**：`pages/users.templ` UserDetailPage 的 Spend/Budget card
**備註**：需要新增 query join SpendLogs table，可能超出本 PR scope。建議開 follow-up issue。

### Bug #4 — ILIKE search 不 escape `%` 和 `_` 特殊字元

**嚴重度**：P2
**重現步驟**：在搜尋框輸入 `%` 或 `_`
**預期結果**：搜尋包含這些字元的用戶名/email
**實際結果**：`%` 變成 wildcard 匹配所有，`_` 匹配任意單字元
**位置**：`user_queries.go:8` — `ILIKE '%' || $1 || '%'`

---

## 結論

### 做得好的地方
- RBAC middleware 正確實作，session 驗證安全（HMAC-signed cookie）
- Soft delete 的 `COALESCE` 處理完善，NULL case 全有覆蓋
- Last admin 保護三處都有（block/delete/role change）
- SQL injection 防護完善，全部 parameterized
- Email 唯一性檢查正確處理了 soft-deleted 用戶
- Empty state 和錯誤提示 UX 良好
- DB-level pagination 是正確的改進

### 需要處理

| # | Issue | 嚴重度 | 建議 |
|---|-------|--------|------|
| Bug #2 | CreatedBy/UpdatedBy 寫死 | P1 | 開 follow-up issue，先記錄技術債 |
| Bug #3 | Per-model spend 未實作 | P1 | 開 follow-up issue |
| Bug #1 | Detail 頁 last admin 無提示 | P2 | 本 PR 修復 |
| Bug #4 | ILIKE 特殊字元 | P2 | 可 follow-up |
| — | 測試覆蓋率不足 | P1 | 至少補 RBAC + last admin 測試 |

### 判決

**🟡 Conditional PASS** — 無 P0 擋上線問題。P1 items（Bug #2、#3）可以開 follow-up issue 追蹤。但**測試覆蓋率是真正的短板**——建議在 merge 前至少補 `TestRequireAdmin` 和 `TestHandleUserDelete_LastAdmin` 兩個測試。Bug #1 建議本 PR 修（改動很小）。

如果補了測試 + 修了 Bug #1 → **PASS**。
