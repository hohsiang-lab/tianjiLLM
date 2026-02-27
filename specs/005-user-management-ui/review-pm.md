# PM Review: User Management UI (HO-19)

**Reviewer**: 諸葛亮（PM）
**Date**: 2026-02-27
**Status**: 🟡 Needs Revision

---

## 總結

Spec 和 Plan 的大方向正確，跟現有 Teams/Orgs pattern 一致。但有幾個**關鍵遺漏**必須在實作前處理，否則會留下安全漏洞或產生 bug。

---

## 🔴 Critical Issues

### 1. RBAC 完全沒有實作方案

**問題**：Spec 明確列出三種角色（Admin / Member / Viewer），但 Plan **完全沒提** RBAC 怎麼做。現有 `sessionAuth` middleware 只檢查「是否登入」，不檢查角色。

**現狀**：`routes.go` 的 protected group 只有 `r.Use(h.sessionAuth)`，沒有任何 role-based middleware。Teams / Orgs 頁面也沒做 RBAC——目前所有登入用戶都能操作所有頁面。

**要求**：
- Plan 必須明確說明 RBAC 策略。至少兩個選項：
  - (A) 新增 `requireAdmin` middleware，Users 頁面全部限 admin（最小可行）
  - (B) 在 handler 層判斷 session user role，Member 只能看自己，Viewer 唯讀
- 如果選 (B)，需定義 session 中如何攜帶 user role（目前 session 結構需確認）
- **建議 v1 先做 (A)**，因為 Teams/Orgs 目前也沒做 per-user 權限，保持一致

### 2. Soft Delete 與現有 Query 衝突

**問題**：Spec 說 soft delete 用 `metadata->>'status' = 'deleted'`，但現有的 `ListUsers`、`GetUser`、`GetUserByEmail` 查詢**都不會過濾 deleted 的 user**。

**影響**：
- Soft delete 後，被刪的 user 仍會出現在 list 頁面
- `GetUserByEmail` 仍會找到被刪的 user，導致「email 已存在」誤判
- API 層（非 UI）如果也用這些 query，會繼續對 deleted user 發 key

**要求**：
- `ListUsersPaginated` 必須加 `AND (metadata->>'status' IS NULL OR metadata->>'status' != 'deleted')` 條件
- 考慮是否也修改 `GetUserByEmail` 以排除 deleted（或者在 handler 層處理）
- Plan 需明確列出哪些現有 query 需要加 status 過濾

### 3. `metadata` JSONB 的 `status` 欄位沒有初始值

**問題**：Schema 預設 `metadata JSONB NOT NULL DEFAULT '{}'`。現有 user 的 metadata 裡沒有 `status` key。

**影響**：`jsonb_set(metadata, '{status}', ...)` 在 metadata 為空 `{}` 時可以正常運作（會新增 key），但查詢 `metadata->>'status'` 對現有 user 會返回 NULL。

**要求**：
- 查詢條件必須處理 NULL case（`metadata->>'status' IS NULL OR metadata->>'status' = 'active'`）
- Spec 需明確定義：現有 user（沒有 status key）視為 active

---

## 🟡 Important Issues

### 4. Teams `blocked` vs Users `metadata.status` 不一致

**問題**：TeamTable 用 `blocked BOOLEAN` 欄位，但 Users plan 用 `metadata->>'status'`。同一個 codebase 兩種 pattern。

**建議**：統一做法。如果不想加 migration，`metadata.status` 可以接受，但 Plan 需明確說明為什麼選這個方案（避免 schema migration），以及長期是否要統一。

### 5. `ListUsersPaginated` 用 DB-level pagination，但 Teams 用 Go in-memory filter

**問題**：看 `handler_teams.go`，Teams 是 `ListTeams` 全撈再 Go 裡 filter/paginate。Plan 提出用 SQL `LIMIT/OFFSET` 做 DB-level pagination。

**評估**：DB-level pagination 是**更好的做法**，但跟現有 pattern 不一致。

**建議**：可以接受，但 Plan 需說明這是**刻意改進**而非疏忽。未來 Teams 也應該改成 DB-level pagination（可開 follow-up issue）。

### 6. Spec 缺少 filter by status 的 scenario

**問題**：Spec FR-003 說「Admin can filter users by role and status」，但 acceptance scenario 只提到 search，沒有 filter by role/status 的具體 scenario。

**要求**：補充 acceptance scenario：
- Given users with different roles, When admin selects role filter, Then only matching users shown
- Given disabled users exist, When admin selects "disabled" status filter, Then only disabled users shown

### 7. Plan 的 SQL `CountUsers` 缺少 status 過濾

**問題**：`CountUsers` query 沒有 status 參數，但如果 list 頁面要 filter by status，count 也要一致。

**要求**：`CountUsers` 和 `ListUsersPaginated` 的 WHERE 條件必須完全一致（加 status 參數）。

---

## 🟢 Minor / Nice-to-Have

### 8. Edge Case: 最後一個 Admin

Spec 有提到「prevent role change/delete for last admin」，但 Plan 沒有提到如何實作。需要一個 `CountUsersByRole('proxy_admin')` query 或在 handler 裡先查。

### 9. User Detail 頁的 Spend per-model breakdown

Spec 說 detail 頁要顯示 per-model spend breakdown，但 Plan 沒提到從哪裡查這個資料。應該要 join `SpendLogs` 表做 group by model。Plan 需補充這個 query。

### 10. Create User 缺少 password/auth 說明

Spec assumption 說「No password management in v1」，但沒說清楚：建立 user 後，user 怎麼登入？靠 API key？靠 SSO？Plan 應該明確寫 out of scope，避免實作時猜測。

### 11. Test Plan 缺少 RBAC 測試

如果加了 RBAC，test plan 需要補：
- TestHandleUsers_NonAdminDenied
- TestHandleUserDetail_MemberSeesOnlySelf

---

## Checklist

| # | Issue | Severity | Action Required |
|---|-------|----------|-----------------|
| 1 | RBAC 沒有實作方案 | 🔴 Critical | Plan 補充 RBAC middleware 方案 |
| 2 | Soft delete 不過濾現有 query | 🔴 Critical | 修改 query + plan 補充 |
| 3 | metadata.status 無初始值 | 🔴 Critical | Query 處理 NULL case |
| 4 | blocked vs metadata.status 不一致 | 🟡 Important | Plan 說明理由 |
| 5 | DB vs in-memory pagination 不一致 | 🟡 Important | Plan 說明是刻意改進 |
| 6 | 缺 filter scenario | 🟡 Important | Spec 補 acceptance scenario |
| 7 | CountUsers 缺 status 過濾 | 🟡 Important | 修改 SQL |
| 8 | Last admin 保護 | 🟢 Minor | Plan 補實作方式 |
| 9 | Per-model spend query | 🟢 Minor | Plan 補 query |
| 10 | Auth 方式未說明 | 🟢 Minor | Spec 補 assumption |
| 11 | RBAC 測試 | 🟢 Minor | Test plan 補充 |

---

## 結論

**3 個 Critical issue 解決前不應進入實作。** 最大的問題是 RBAC——整個 spec 的核心需求之一（Role-Based Access），但 plan 完全沒有技術方案。建議先解決 #1 #2 #3，其餘 Important 和 Minor 可以在 PR review 時處理。
