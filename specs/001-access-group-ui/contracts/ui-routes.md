# UI Routes Contract: Access Group 管理

**Feature**: `001-access-group-ui`
**Base Path**: `/ui/access-groups`

這些是 UI handler 路由，不是 REST API 路由。所有路由需要 session 認證（`sessionAuth` middleware）。

---

## Routes

### List Page

```
GET /ui/access-groups
```

**Handler**: `handleAccessGroups`
**Response**: Full HTML page (templ: `AccessGroupsPage`)
**Query params**: `page` (int, default 1), `search` (string)

---

```
GET /ui/access-groups/table
```

**Handler**: `handleAccessGroupsTable`
**Response**: HTMX partial (templ: `AccessGroupsTablePartial`)
**Query params**: `page` (int), `search` (string)
**HTMX trigger**: `hx-trigger="input delay:500ms"` on search input

---

### Create

```
POST /ui/access-groups/create
```

**Handler**: `handleAccessGroupCreate`
**Form fields**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `group_alias` | string | ✅ | 群組名稱 |
| `organization_id` | string | ❌ | 選填，組織 ID |

**Response (success)**: HTMX partial `AccessGroupsTableWithToast(data, "Access group created successfully", toast.VariantSuccess)`
**Response (error)**: HTMX partial `AccessGroupsTableWithToast(data, "<error>", toast.VariantError)`

**Error cases**:
- `group_alias` 為空 → "Alias is required"
- `group_alias` 已存在 → "Alias already exists"
- DB 錯誤 → "Failed to create access group: <err>"

---

### Detail Page

```
GET /ui/access-groups/{group_id}
```

**Handler**: `handleAccessGroupDetail`
**Response**: Full HTML page (templ: `AccessGroupDetailPage`)
**404 behavior**: Redirect to `/ui/access-groups`

---

### Update (alias + org)

```
POST /ui/access-groups/{group_id}/update
```

**Handler**: `handleAccessGroupUpdate`
**Form fields**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `group_alias` | string | ✅ | 新的群組名稱 |
| `organization_id` | string | ❌ | 新的組織 ID（空值表示清除） |

**Response (success)**: HTMX partial `AccessGroupDetailHeaderWithToast(data, "Updated successfully", toast.VariantSuccess)`
**Response (error)**: HTMX partial `AccessGroupDetailHeaderWithToast(data, "<error>", toast.VariantError)`

---

### Models Management (Detail Page)

```
POST /ui/access-groups/{group_id}/models/add
```

**Handler**: `handleAccessGroupModelAdd`
**Form fields**: `model_name` (string, required)
**Response (success)**: HTMX partial `AccessGroupModelsWithToast(data, "Model added", toast.VariantSuccess)`
**Response (error)**: HTMX partial `AccessGroupModelsWithToast(data, "<error>", toast.VariantError)`

---

```
POST /ui/access-groups/{group_id}/models/remove
```

**Handler**: `handleAccessGroupModelRemove`
**Form fields**: `model_name` (string, required)
**Response**: HTMX partial `AccessGroupModelsWithToast(data, "Model removed", toast.VariantSuccess)`

---

### Delete

```
POST /ui/access-groups/{group_id}/delete
```

**Handler**: `handleAccessGroupDelete`
**Protection**: 查詢 `ListKeysByAccessGroup` — 若有 key 引用，返回錯誤 toast，不執行刪除
**Response (success)**: `HX-Redirect: /ui/access-groups` (HTTP 200 with header)
**Response (error)**: HTMX partial `AccessGroupsTableWithToast(data, "Cannot delete: <N> keys still use this group", toast.VariantError)`

---

## HTMX Swap Targets

| Action | `hx-target` | `hx-swap` |
|--------|-------------|-----------|
| Search | `#access-groups-table-container` | `outerHTML` |
| Pagination | `#access-groups-table-container` | `outerHTML` |
| Create | `#access-groups-table-container` | `outerHTML` |
| Delete (list) | `#access-groups-table-container` | `outerHTML` |
| Update (detail header) | `#access-group-header` | `outerHTML` |
| Model add/remove | `#access-group-models-section` | `outerHTML` |

---

## REST API Routes (existing, unchanged)

以下為現有 REST API routes（proxy handler），UI **不使用**這些端點，僅供參考：

```
POST   /model_access_group/new              → AccessGroupNew
GET    /model_access_group/info/{group_id}  → AccessGroupInfo
POST   /model_access_group/update           → AccessGroupUpdate (models only)
DELETE /model_access_group/delete/{group_id} → AccessGroupDelete
```
