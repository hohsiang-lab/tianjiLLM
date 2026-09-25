# HO-72: Request Logs UI 應顯示 Error Requests（429/500/400 等）

## 問題

Request Logs UI 完全看不到 error requests（429/500/400 等）。
DB 中 `ErrorLogs` 表有 72 筆錯誤記錄，卻一筆都不顯示。

## Root Cause

### 失敗請求的資料流差異

```
成功請求：LiteLLM callback → SpendLogs ✅  ErrorLogs ❌
失敗請求：LiteLLM callback → SpendLogs ❌  ErrorLogs ✅
```

失敗請求（rate limit、upstream 5xx 等）只寫 `ErrorLogs`，不寫 `SpendLogs`（沒有 spend/token 可記錄）。

### SQL 根本問題

`ListRequestLogs` 目前：

```sql
FROM "SpendLogs" sl
LEFT JOIN "ErrorLogs" el ON sl.request_id = el.request_id
```

以 `SpendLogs` 為主表。`ErrorLogs`-only 記錄（無對應 SpendLogs row）完全不出現。

### 兩表 Schema 差異

| 欄位 | SpendLogs | ErrorLogs |
|------|-----------|-----------|
| request_id | ✅ | ✅ |
| api_key | api_key（明文 hash） | api_key_hash |
| model | provider/model | model |
| timestamp | starttime / endtime | created_at |
| status_code | ❌ | ✅ |
| error_type | ❌ | ✅ |
| spend/tokens | ✅ | ❌ |
| team_id | ✅ | ❌ |

## 目標

讓 Request Logs UI 同時顯示成功與失敗的 requests。

## 驗收條件

- [ ] ErrorLogs-only 的記錄（72 筆）出現在 Request Logs 表格
- [ ] 這些記錄 Status 欄顯示 `Failed` badge
- [ ] Model 欄正確顯示（來自 ErrorLogs.model）
- [ ] Key Hash 欄正確顯示（來自 ErrorLogs.api_key_hash）
- [ ] Cost、Tokens、Duration 欄顯示 `–`
- [ ] Status cell 顯示 status_code 和 error_type（如 `429 / RateLimitError`）
- [ ] Status filter = `Failed` → 包含兩類：SpendLogs+ErrorLogs 配對、ErrorLogs-only
- [ ] Status filter = `Success` → 不顯示任何 error records
- [ ] 排序依時間倒序（成功用 starttime，失敗用 created_at）
- [ ] TotalCount（分頁計數）包含 error-only records

## 技術設計

### SQL：UNION ALL 取代純 LEFT JOIN

修改 `internal/db/queries/spend_views.sql` 中的 `ListRequestLogs` 和 `CountRequestLogs`：

**策略**：保留原 SpendLogs LEFT JOIN ErrorLogs 邏輯（涵蓋成功 + 有 SpendLogs 的失敗），加上第二段 UNION ALL 只撈 `ErrorLogs`-only（沒有對應 SpendLogs 的）。

```sql
-- name: ListRequestLogs :many
(
  SELECT
    sl.request_id,
    sl.starttime        AS ts,
    sl.endtime,
    sl.api_key          AS key_hash,
    sl.model,
    sl.spend,
    sl.total_tokens,
    sl.prompt_tokens,
    sl.completion_tokens,
    sl.cache_hit,
    sl.team_id,
    sl.end_user,
    el.status_code      AS error_status_code,
    el.error_type
  FROM "SpendLogs" sl
  LEFT JOIN "ErrorLogs" el ON sl.request_id = el.request_id
  WHERE sl.starttime >= sqlc.arg(start_date)
    AND sl.starttime < sqlc.arg(end_date)
    AND (sqlc.narg(filter_api_key)::text IS NULL OR sl.api_key = sqlc.narg(filter_api_key))
    AND (sqlc.narg(filter_team_id)::text IS NULL OR sl.team_id = sqlc.narg(filter_team_id))
    AND (sqlc.narg(filter_model)::text IS NULL OR sl.model = sqlc.narg(filter_model))
    AND (sqlc.narg(filter_request_id)::text IS NULL OR sl.request_id = sqlc.narg(filter_request_id))
    AND (sqlc.narg(filter_status)::text IS NULL
         OR (sqlc.narg(filter_status) = 'success' AND el.id IS NULL)
         OR (sqlc.narg(filter_status) = 'failed' AND el.id IS NOT NULL))
)
UNION ALL
(
  SELECT
    el.request_id,
    el.created_at       AS ts,
    NULL                AS endtime,
    el.api_key_hash     AS key_hash,
    el.model,
    0.0::float8         AS spend,
    0::int8             AS total_tokens,
    0::int8             AS prompt_tokens,
    0::int8             AS completion_tokens,
    ''                  AS cache_hit,
    NULL::text          AS team_id,
    NULL::text          AS end_user,
    el.status_code      AS error_status_code,
    el.error_type
  FROM "ErrorLogs" el
  WHERE NOT EXISTS (SELECT 1 FROM "SpendLogs" WHERE request_id = el.request_id)
    AND el.created_at >= sqlc.arg(start_date)
    AND el.created_at < sqlc.arg(end_date)
    AND (sqlc.narg(filter_model)::text IS NULL OR el.model = sqlc.narg(filter_model))
    AND (sqlc.narg(filter_request_id)::text IS NULL OR el.request_id = sqlc.narg(filter_request_id))
    AND (sqlc.narg(filter_api_key)::text IS NULL OR el.api_key_hash = sqlc.narg(filter_api_key))
    AND (sqlc.narg(filter_status)::text IS NULL OR sqlc.narg(filter_status) = 'failed')
)
ORDER BY ts DESC
LIMIT sqlc.arg(query_limit) OFFSET sqlc.arg(query_offset);
```

> **filter_team_id 限制**：`ErrorLogs` 無 `team_id` 欄位，error-only records 在 team filter 下不顯示。可接受，後續如需修正需 DB migration 加欄位。

同理修改 `CountRequestLogs` 改為 `SELECT COUNT(*) FROM (... UNION ALL ...) AS combined`。

### Go：handler_logs.go

`ListRequestLogsRow` 的欄位名稱由 sqlc 重新產生後可能有調整。`toLogRow()` 中：
- 原本的 `row.ApiKey` 現在統一為 `row.KeyHash`（兩條路徑都已 alias 為 key_hash）
- error-only records：`row.ErrorStatusCode != nil` → `Status = "Failed"`，spend/tokens 為 0 → templ 自動顯示 `–`

### Templ：pages/logs.templ

在 `logRow()` Status cell 擴充顯示 status_code 和 error_type：

```go
if row.Status == "Failed" {
    @badge.Badge(badge.Props{Variant: badge.VariantDestructive}) { Failed }
    if row.StatusCode != nil {
        <span class="text-xs text-muted-foreground ml-1">
            { fmt.Sprintf("%d", *row.StatusCode) }
        </span>
    }
    if row.ErrorType != nil && *row.ErrorType != "" {
        <span class="block text-xs text-muted-foreground truncate max-w-[120px]" title={ *row.ErrorType }>
            { *row.ErrorType }
        </span>
    }
}
```

### 產生指令

```bash
sqlc generate
templ generate
```

## 影響範圍

| 元件 | 變更 |
|------|------|
| `internal/db/queries/spend_views.sql` | 修改 ListRequestLogs、CountRequestLogs |
| `internal/db/spend_views.sql.go` | sqlc 重新產生（自動） |
| `internal/ui/handler_logs.go` | 調整 toLogRow() 欄位引用 |
| `internal/ui/pages/logs.templ` | Status cell 加 status_code + error_type |
| `internal/ui/pages/logs_templ.go` | templ 重新產生（自動） |

## 不在本次範圍

- ErrorLogs 加 team_id 欄位（需 DB migration）
- Error request detail drawer
- ErrorLogs 清理策略（目前無 TTL）

## 參考

- `internal/db/queries/spend_views.sql` — 現有 ListRequestLogs SQL
- `internal/db/queries/error_log.sql` — ErrorLogs schema
- `internal/ui/handler_logs.go` — loadLogsPageData()、toLogRow()
- `internal/ui/pages/logs.templ` — logRow()
- Spec HO-67 — 上次 Logs UI 修復背景
