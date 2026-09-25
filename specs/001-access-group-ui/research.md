# Research: Access Group 管理 UI

**Branch**: `001-access-group-ui` | **Phase**: 0

## Summary

所有「NEEDS CLARIFICATION」項目均已透過直接讀取現有代碼庫解決，無需外部文件查詢。

---

## Decision 1: 資料庫 Schema 與現有查詢

**Decision**: 使用現有 `ModelAccessGroup` table（`003_organization.up.sql` 中定義），並擴充 `access_group.sql` 查詢集。

**Current schema**:
```sql
CREATE TABLE IF NOT EXISTS "ModelAccessGroup" (
    group_id        TEXT PRIMARY KEY,
    group_alias     TEXT,                          -- 顯示名稱（可為空）
    models          TEXT[] NOT NULL DEFAULT '{}',  -- 允許模型清單
    organization_id TEXT REFERENCES "OrganizationTable"(organization_id),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by      TEXT NOT NULL DEFAULT '',
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by      TEXT NOT NULL DEFAULT ''
);
```

**Existing sqlc queries** (已生成，可直接使用):
| Query | Method | 用途 |
|-------|--------|------|
| `CreateAccessGroup` | `:one` | 建立群組 |
| `GetAccessGroup` | `:one` | 取得單一群組 |
| `ListAccessGroups` | `:many` | 列出所有群組 |
| `UpdateAccessGroup` | `:exec` | 更新 models 欄位 |
| `DeleteAccessGroup` | `:exec` | 刪除群組 |

**Missing queries — 需新增至 `access_group.sql`**:

| Query | 類型 | 用途 |
|-------|------|------|
| `GetAccessGroupByAlias` | `:one` | 建立時重複 alias 檢查 |
| `UpdateAccessGroupMeta` | `:exec` | 更新 alias + organization_id |
| `AddAccessGroupModel` | `:exec` | 詳細頁新增單一模型 |
| `RemoveAccessGroupModel` | `:exec` | 詳細頁移除單一模型 |
| `ListKeysByAccessGroup` | `:many` | 詳細頁顯示參照此群組的 API Keys |

**Rationale**: 現有的 `UpdateAccessGroup` 僅更新 `models` 欄位，但 FR-007 需要編輯 `group_alias` 與 `organization_id`，故需新增 `UpdateAccessGroupMeta`。模型的新增/移除採用 `array_append`/`array_remove` 模式，與 Teams 的 `AddTeamModel`/`RemoveTeamModel` 一致。

**Alternatives considered**: 將 `UpdateAccessGroup` 改為更新所有欄位 — 拒絕，會破壞現有 REST API handler (`accessgroup.go`) 的行為（該 handler 只傳遞 models）。

---

## Decision 2: UI 架構模式

**Decision**: 完全遵循現有 guardrails + teams 的 UI 模式。

**Pattern confirmed from codebase**:
```
handler_[feature].go           → 列表 CRUD handlers
handler_[feature]_detail.go    → 詳細頁 handlers
pages/[feature].templ          → 列表頁 templ 模板
pages/[feature]_detail.templ   → 詳細頁 templ 模板（如有必要）
```

**HTMX patterns** (from guardrails/teams):
- `loadXxxPageData()` → 載入資料的共用函式
- `pages.XxxTableWithToast()` → HTMX 局部更新 + Toast 通知
- `hx-post` → 表單提交
- `hx-get` → 搜尋框延遲篩選
- Dialog 元件 → 確認刪除、建立/編輯表單

**Rationale**: 遵循現有模式確保 UI 一致性，減少引入新概念的風險。

**Alternatives considered**: 使用 SPA 框架 — 拒絕，這與整個 UI 的 HTMX + server-rendered 架構不符。

---

## Decision 3: 成員反向查詢範圍

**Decision**: 詳細頁的「成員清單」僅顯示 API Keys，暫不顯示 Teams。

**Evidence from schema**:
- `VerificationToken` 有 `access_group_ids TEXT[]` ✅
- `TeamTable` **沒有** `access_group_ids` 欄位 ❌

**Rationale**: 現有 schema 中 `TeamTable` 沒有 `access_group_ids` 欄位，因此無法透過反向查詢找到哪些 Team 屬於此 Access Group。若強行新增此欄位，超出本 Feature 的範疇。詳細頁顯示 "API Keys using this group" 已滿足 FR-012 的核心需求；spec 中 "Teams 清單" 為 "唯讀，供參考"，故此設計在 spec 範圍內為可接受的次要妥協。

**Risk**: 與 spec FR-012 有局部差距（未顯示 Teams）。須在 plan.md 的 Complexity Tracking 中記錄。

**Alternatives considered**:
1. 新增 migration 為 `TeamTable` 加 `access_group_ids` 欄位 — 超出 UI feature 範疇，推遲至獨立任務
2. 顯示「Teams 功能尚未支援」的空狀態提示 — 採用，既誠實又不阻礙上線

---

## Decision 4: 刪除保護策略

**Decision**: 刪除前查詢 `VerificationToken` 檢查引用，有引用則顯示錯誤並拒絕刪除。

**Evidence**: spec Assumptions 明確說明 "採用警告並拒絕刪除策略（需先在 key/team 端解除關聯才能刪除）"。

**Implementation**: 使用 `ListKeysByAccessGroup` 查詢，若回傳非空清單則拒絕刪除並顯示錯誤 toast，列出有多少 keys 仍在使用。

**Pattern reference**: `handler_guardrails.go:handleGuardrailDelete` — 查詢 policies 中的引用，找到則拒絕刪除，顯示具體 policy 名稱。

---

## Decision 5: 搜尋篩選策略

**Decision**: 在 Go handler 中進行記憶體篩選，與 guardrails/teams 模式完全一致。

**Rationale**: Access Group 數量預期在數百以下，記憶體篩選效能足夠（SC-004 要求 1 秒內，Go 對 1000 筆記錄的記憶體篩選遠低於 10ms）。

**Alternatives considered**: 資料庫 `ILIKE` 查詢 — 效能過度設計，且需額外 sqlc 查詢。

---

## Decision 6: 路由設計

**Decision**: 遵循 teams 路由結構。

```
GET  /ui/access-groups              → 列表頁
GET  /ui/access-groups/table        → HTMX 表格局部更新
POST /ui/access-groups/create       → 建立
GET  /ui/access-groups/{group_id}   → 詳細頁
POST /ui/access-groups/{group_id}/update         → 更新 alias + org
POST /ui/access-groups/{group_id}/models/add     → 新增模型
POST /ui/access-groups/{group_id}/models/remove  → 移除模型
POST /ui/access-groups/{group_id}/delete         → 刪除（含保護檢查）
```

**Spec requirement FR-013**: 側邊導覽列需加入 Access Groups 入口。
在 `layout.templ` 的 Management 群組中，加在 Guardrails 之後：
```
@navItem("/ui/access-groups", "Access Groups", "layers", activePath)
```

---

## Unresolved Items

無。所有 NEEDS CLARIFICATION 均已解決。
