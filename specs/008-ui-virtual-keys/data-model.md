# Data Model: UI Virtual Keys Management

**Feature**: 008-ui-virtual-keys
**Date**: 2026-02-20

## Entities

### VerificationToken (existing — no schema changes)

已存在於 `internal/db/schema/001_initial.sql`。本 feature 不修改 DB schema，只新增 sqlc 查詢。

| Field | Type | Nullable | UI 用途 |
|-------|------|----------|---------|
| `token` | TEXT PK | No | Key ID（hash），列表/詳情頁的唯一識別 |
| `key_name` | TEXT | Yes | 內部名稱（UI 顯示為 Secret Key 脫敏值） |
| `key_alias` | TEXT | Yes | 使用者可讀別名，列表主要顯示欄位 |
| `spend` | DOUBLE PRECISION | No (default 0) | 花費金額，列表 + 詳情頁 |
| `max_budget` | DOUBLE PRECISION | Yes | 預算上限，NULL = Unlimited |
| `expires` | TIMESTAMPTZ | Yes | 到期時間，NULL = Never |
| `models` | TEXT[] | No (default '{}') | 允許模型列表，空 = All Models |
| `user_id` | TEXT | Yes | 關聯使用者 |
| `team_id` | TEXT | Yes | 關聯 team |
| `organization_id` | TEXT | Yes | 關聯組織 |
| `metadata` | JSONB | No (default '{}') | 自定義 metadata |
| `blocked` | BOOLEAN | Yes | 封鎖狀態 |
| `tpm_limit` | BIGINT | Yes | Tokens per minute 限制 |
| `rpm_limit` | BIGINT | Yes | Requests per minute 限制 |
| `budget_duration` | TEXT | Yes | 預算重置週期（daily/weekly/monthly） |
| `budget_reset_at` | TIMESTAMPTZ | Yes | 預算下次重置時間 |
| `created_at` | TIMESTAMPTZ | Yes (default NOW()) | 建立時間 |
| `created_by` | TEXT | Yes | 建立者 |
| `updated_at` | TIMESTAMPTZ | Yes (default NOW()) | 更新時間 |
| `updated_by` | TEXT | Yes | 更新者 |

### TeamTable (existing — read-only reference)

UI 列表頁需要 JOIN 或額外查詢取得 team alias。

| Field | UI 用途 |
|-------|---------|
| `team_id` | 過濾器下拉選單 |
| `team_alias` | 列表頁 Team Alias 欄位 |

### LiteLLM_UserTable (existing — read-only reference)

UI 建立表單需要 User ID 下拉選單。

| Field | UI 用途 |
|-------|---------|
| `user_id` | 過濾器 + 建立表單下拉 |

## UI View Models (Go structs)

### KeyRow (existing — enhanced)

列表頁每行數據。擴充現有 `KeyRow` struct。
**注意**：現有 KeyRow 使用值類型（`string`, `bool`），handler 在 DB→UI 邊界做 nil 轉換。新增字段遵循同樣模式。

```
KeyRow:
  Token         string          // token hash (PK, 用於 API 操作)（現有）
  KeyName       string          // 脫敏的 key 名稱（現有，handler 轉 nil→""）
  KeyAlias      string          // 可讀別名（現有，handler 轉 nil→""）
  Spend         float64         // 花費（現有）
  MaxBudget     *float64        // 預算上限 (nil = Unlimited)（現有，保持指標因為需區分 0 和 nil）
  Models        []string        // 允許模型 (empty = All)（現有）
  Blocked       bool            // 封鎖狀態（現有，handler 轉 nil→false）
  CreatedAt     time.Time       // 建立時間（現有）
  // --- 以下為新增字段 ---
  Expires       *time.Time      // 到期時間 (nil = Never)
  TeamID        string          // team ID (handler 轉 nil→"")
  TeamAlias     string          // team 別名 (分離查詢取得)
  UserID        string          // user ID (handler 轉 nil→"")
  TPMLimit      *int64          // TPM 限制 (保持指標：nil = Unlimited)
  RPMLimit      *int64          // RPM 限制 (保持指標：nil = Unlimited)
  BudgetDuration string         // 預算週期 (handler 轉 nil→"")
  BudgetResetAt *time.Time      // 預算重置時間
```

### KeysPageData (existing — enhanced)

列表頁完整數據。
**注意**：保留現有 `Search` 字段以向後兼容。

```
KeysPageData:
  Keys          []KeyRow        // （現有）
  Page          int             // 當前頁碼 (1-based)（現有）
  TotalPages    int             // 總頁數（現有）
  Search        string          // 搜尋關鍵字（現有，保留向後兼容）
  // --- 以下為新增字段 ---
  TotalCount    int             // 總筆數（精確分頁用）
  // 過濾狀態
  FilterTeamID    string
  FilterKeyAlias  string
  FilterUserID    string
  FilterKeyHash   string
```

### KeyDetailData (NEW)

詳情頁數據。

```
KeyDetailData:
  // 完整的 VerificationToken 欄位
  Token             string
  KeyName           *string
  KeyAlias          *string
  Spend             float64
  MaxBudget         *float64
  Expires           *time.Time
  Models            []string
  UserID            *string
  TeamID            *string
  OrganizationID    *string
  Metadata          string        // JSON string for display
  Blocked           *bool
  TPMLimit          *int64
  RPMLimit          *int64
  BudgetDuration    *string
  BudgetResetAt     *time.Time
  Tags              []string      // extracted from metadata
  CreatedAt         time.Time
  CreatedBy         *string
  UpdatedAt         time.Time
  UpdatedBy         *string
  // 計算欄位
  IsExpired         bool          // expires < now
  IsBlocked         bool          // blocked == true
  DisplayAlias      string        // key_alias || "Virtual Key"
  BudgetProgress    float64       // spend / max_budget * 100 (0 if unlimited)
```

### ToastData

Toast 通知不需要獨立 struct。直接使用 templ 函數參數傳遞 message + variant，
然後渲染現有 `toast.Toast(toast.Props{...})` 組件。

Toast 組件自帶 `fixed` 定位（`z-50 fixed`），通過 `data-tui-toast` + `toast.min.js` 自動管理位置和消失動畫。不需要容器元素。

## New sqlc Queries Required

**重要**：所有可選過濾參數必須使用 `sqlc.narg()` 生成 nullable 類型（`*string`），否則空字符串 `""` ≠ SQL NULL，IS NULL 判斷永遠為 false。

### ListVerificationTokensFiltered

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

### CountVerificationTokensFiltered

```sql
-- name: CountVerificationTokensFiltered :one
SELECT COUNT(*) FROM "VerificationToken"
WHERE
  (sqlc.narg(filter_team_id)::text IS NULL OR team_id = sqlc.narg(filter_team_id)) AND
  (sqlc.narg(filter_key_alias)::text IS NULL OR key_alias = sqlc.narg(filter_key_alias)) AND
  (sqlc.narg(filter_user_id)::text IS NULL OR user_id = sqlc.narg(filter_user_id)) AND
  (sqlc.narg(filter_token)::text IS NULL OR token = sqlc.narg(filter_token));
```

### GetVerificationTokenByAlias (for alias uniqueness check)

```sql
-- name: GetVerificationTokenByAlias :one
SELECT token FROM "VerificationToken"
WHERE key_alias = sqlc.arg(alias) AND (sqlc.narg(filter_team_id)::text IS NULL OR team_id = sqlc.narg(filter_team_id))
LIMIT 1;
```

### RegenerateVerificationTokenWithParams (NEW — 支持同時修改屬性)

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

### ListTeamAliases (NEW — team_id → alias 映射)

```sql
-- name: ListTeamAliases :many
SELECT team_id, team_alias FROM "TeamTable"
WHERE team_id = ANY(sqlc.arg(team_ids)::text[]);
```

## State Transitions

### Key Lifecycle

```
[Created] → active
  ├── block → [Blocked]
  │     └── unblock → [Active]
  ├── edit → [Active] (attributes changed)
  ├── regenerate → [Active] (new token, spend reset)
  ├── expire → [Expired] (automatic, time-based)
  └── delete → [Deleted] (permanent)
```

## Validation Rules

| Field | Rule | Source |
|-------|------|--------|
| key_alias | Required on create | FR-016 |
| key_alias | Unique within team | FR-017 |
| max_budget | >= 0 or NULL | FR-032 |
| tpm_limit | Positive integer or NULL | FR-032 |
| rpm_limit | Positive integer or NULL | FR-032 |
| duration | Format: `\d+[smhd]` or empty | FR-033 |
| metadata | Valid JSON or empty | FR-016 |
