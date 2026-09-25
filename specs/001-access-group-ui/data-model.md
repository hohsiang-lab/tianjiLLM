# Data Model: Access Group 管理 UI

**Branch**: `001-access-group-ui`

---

## Entity: ModelAccessGroup

### Source Table

```sql
-- File: internal/db/schema/003_organization.up.sql
CREATE TABLE IF NOT EXISTS "ModelAccessGroup" (
    group_id        TEXT PRIMARY KEY,
    group_alias     TEXT,
    models          TEXT[] NOT NULL DEFAULT '{}',
    organization_id TEXT REFERENCES "OrganizationTable"(organization_id),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by      TEXT NOT NULL DEFAULT '',
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by      TEXT NOT NULL DEFAULT ''
);
```

### Go Struct (sqlc-generated, `internal/db/models.go`)

```go
type ModelAccessGroup struct {
    GroupID        string             `json:"group_id"`
    GroupAlias     *string            `json:"group_alias"`       // nullable
    Models         []string           `json:"models"`
    OrganizationID *string            `json:"organization_id"`   // nullable FK
    Metadata       []byte             `json:"metadata"`          // JSONB
    CreatedAt      pgtype.Timestamptz `json:"created_at"`
    CreatedBy      string             `json:"created_by"`
    UpdatedAt      pgtype.Timestamptz `json:"updated_at"`
    UpdatedBy      string             `json:"updated_by"`
}
```

### Business Rules

| Rule | Description |
|------|-------------|
| `group_id` | UUID，由 Go 層使用 `uuid.New().String()` 生成 |
| `group_alias` | 唯一性由應用層保證（建立前查 `GetAccessGroupByAlias`）|
| `models = []` | 空陣列語意 = 允許所有模型（與 Teams 慣例一致）|
| `organization_id` | 可選 FK，刪除組織時不級聯（Access Group 仍存在） |
| 刪除保護 | 若有任何 `VerificationToken.access_group_ids` 包含此 `group_id`，拒絕刪除 |

### State Transitions

```
[created] → [updated alias/org] → [models added/removed] → [deleted]
                                                          ↑
                                          如 key 仍引用則拒絕刪除
```

---

## Related Entity: VerificationToken (反向關聯，唯讀)

```sql
-- File: internal/db/schema/001_initial.up.sql (relevant column only)
"VerificationToken" (
    token             TEXT PRIMARY KEY,
    key_name          TEXT,
    key_alias         TEXT,
    access_group_ids  TEXT[] NOT NULL DEFAULT '{}'  -- 包含 group_id 的清單
)
```

詳細頁顯示「哪些 API Key 參照了此 Access Group」透過此欄位的反向查詢實現。

---

## UI Layer Data Types

以下為 templ 模板使用的 Go struct，定義於 `internal/ui/pages/access_groups.templ`。

### 列表頁

```go
type AccessGroupRow struct {
    GroupID     string
    GroupAlias  string      // 若 DB 中為 nil 則顯示 group_id 前綴
    OrgID       string
    OrgAlias    string      // 從 OrganizationTable join 取得
    ModelCount  int
    CreatedAt   time.Time
}

type AccessGroupsPageData struct {
    Groups      []AccessGroupRow
    Orgs        []OrgOption      // 建立表單中的組織下拉選單
    AvailableModels []string     // 未來用於建立時選擇初始模型（P2 可選）
    Page        int
    TotalPages  int
    TotalCount  int
    PerPage     int
    Search      string
}
```

### 詳細頁

```go
type AccessGroupKeyRow struct {
    Token    string  // 截短的 key token
    KeyName  string
    KeyAlias string
}

type AccessGroupDetailData struct {
    Group           AccessGroupRow
    AvailableModels []string        // 系統中可選模型（用於新增模型下拉）
    Keys            []AccessGroupKeyRow  // 參照此群組的 API Keys（唯讀）
    OrgAlias        string
}
```

---

## SQL Queries: 新增至 `internal/db/queries/access_group.sql`

以下為需要新增的 sqlc 查詢（現有 5 個查詢已可用）：

### GetAccessGroupByAlias

```sql
-- name: GetAccessGroupByAlias :one
SELECT * FROM "ModelAccessGroup" WHERE group_alias = $1 LIMIT 1;
```

**用途**: 建立時重複 alias 檢查。

### UpdateAccessGroupMeta

```sql
-- name: UpdateAccessGroupMeta :exec
UPDATE "ModelAccessGroup"
SET group_alias     = $2,
    organization_id = $3,
    updated_at      = NOW(),
    updated_by      = $4
WHERE group_id = $1;
```

**用途**: 從詳細頁或列表頁更新 alias + organization_id。

### AddAccessGroupModel

```sql
-- name: AddAccessGroupModel :exec
UPDATE "ModelAccessGroup"
SET models     = array_append(models, $2),
    updated_at = NOW()
WHERE group_id = $1
  AND NOT ($2 = ANY(models));
```

**用途**: 詳細頁新增單一模型（冪等 — 已存在則不重複新增）。

### RemoveAccessGroupModel

```sql
-- name: RemoveAccessGroupModel :exec
UPDATE "ModelAccessGroup"
SET models     = array_remove(models, $2),
    updated_at = NOW()
WHERE group_id = $1;
```

**用途**: 詳細頁移除單一模型。

### ListKeysByAccessGroup

```sql
-- name: ListKeysByAccessGroup :many
SELECT token, key_name, key_alias
FROM "VerificationToken"
WHERE $1 = ANY(access_group_ids)
ORDER BY created_at DESC;
```

**用途**: 詳細頁顯示參照此 Access Group 的 API Keys（唯讀）；刪除保護檢查。

---

## Validation Rules

| Field | Rule | Error Message |
|-------|------|---------------|
| `group_alias` | 必填，TrimSpace 後不得為空 | "Alias is required" |
| `group_alias` | 建立時在現有群組中唯一 | "Alias already exists" |
| `organization_id` | 選填，若填寫須為系統中存在的 org_id | (下拉選單限制，無需後端驗證) |
| `model_name` | 新增模型時不得為空 | "Model name is required" |

---

## Scope Note: TeamTable 不在本 Feature 範圍內

`TeamTable` 目前沒有 `access_group_ids` 欄位，因此詳細頁的「成員清單」僅顯示 API Keys。
顯示「Teams 反向查詢功能尚未支援」的友善空狀態提示，而非完全隱藏此區塊。
