# API Contract: Enhanced `/key/list`

**Feature**: 008-ui-virtual-keys
**Date**: 2026-02-20
**Reference**: Python LiteLLM `GET /key/list` endpoint

## Overview

擴充現有 `GET /key/list` endpoint 以支援伺服器端過濾、分頁和排序。保持向後兼容——不傳新參數時行為不變。

## Endpoint

```
GET /key/list
```

### Query Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `page` | int | No | 1 | 頁碼 (1-based, >= 1) |
| `size` | int | No | 50 | 每頁筆數 (1-100) |
| `team_id` | string | No | — | 精確匹配 team_id |
| `key_alias` | string | No | — | 精確匹配 key_alias |
| `user_id` | string | No | — | 精確匹配 user_id |
| `key_hash` | string | No | — | 精確匹配 token (hash) |
| `sort_by` | string | No | `created_at` | 排序欄位。第一版固定 `created_at` |
| `sort_order` | string | No | `desc` | 排序方向。第一版固定 `desc` |

### Response

```json
{
  "keys": [
    {
      "token": "sha256-hash...",
      "key_name": "sk-xxxx...yyyy",
      "key_alias": "my-api-key",
      "spend": 12.50,
      "max_budget": 100.00,
      "expires": "2026-03-01T00:00:00Z",
      "models": ["gpt-4", "claude-sonnet-4-5-20250929"],
      "user_id": "user-123",
      "team_id": "team-456",
      "organization_id": null,
      "blocked": false,
      "tpm_limit": 10000,
      "rpm_limit": 100,
      "budget_duration": "monthly",
      "budget_reset_at": "2026-03-01T00:00:00Z",
      "metadata": {},
      "created_at": "2026-01-15T10:30:00Z",
      "created_by": "admin",
      "updated_at": "2026-02-10T14:20:00Z",
      "updated_by": "admin"
    }
  ],
  "total_count": 150,
  "current_page": 1,
  "total_pages": 3
}
```

### Error Responses

| Status | Condition | Body |
|--------|-----------|------|
| 400 | Invalid page/size (< 1 or size > 100) | `{"error": {"message": "invalid pagination parameters", "type": "invalid_request_error"}}` |
| 503 | Database not configured | `{"error": {"message": "database not configured", "type": "internal_error"}}` |

### Backward Compatibility

不傳 `page` 和 `size` 時：
- `page` 默認 1，`size` 默認 50
- 回應格式從 `{"keys": [...]}` 擴充為 `{"keys": [...], "total_count": N, "current_page": 1, "total_pages": M}`
- 舊客戶端只讀 `keys` 欄位，不受影響

## UI-Only Endpoints (Internal)

以下端點只由 UI handler 使用，不屬於公開 REST API。
沿用現有分離路由模式（全頁 vs HTMX partial 用不同路由），不使用 HX-Request header 檢測。

### GET /ui/keys （現有 — 增強）

列表頁完整頁面。

Query params: `page`, `search` (legacy), `team_id`, `key_alias`, `user_id`, `key_hash`

Response: 完整頁面（AppLayout + table + filters + pagination）

### GET /ui/keys/table （現有 — 增強）

列表頁 HTMX partial。

Query params: 同 `/ui/keys`

Response: Table HTML fragment（含分頁）

### GET /ui/keys/{token} （新增）

詳情頁。token 是 SHA256 hash。

Response: 完整頁面（AppLayout + detail content with Overview/Settings tabs）

### GET /ui/keys/{token}/edit （新增）

編輯表單片段（HTMX partial）。

Response: 編輯表單 HTML fragment（替換 Settings tab 內容）

### POST /ui/keys/create （現有 — 增強）

建立 key。Form data。增加更多字段（key_alias, user_id, team_id, models, max_budget, tpm_limit, rpm_limit, budget_duration, metadata）。

Response: Table partial + OOB「Save your Key」toast/dialog（包含明文 key）

### POST /ui/keys/{token}/update （新增）

更新 key 屬性。Form data。

Response: Settings tab 查看模式 HTML fragment + OOB toast

### POST /ui/keys/{token}/delete （新增 — 替代現有 /ui/keys/delete）

刪除 key。token 在 URL path 中。

Response: `HX-Redirect: /ui/keys` header

### POST /ui/keys/block, /ui/keys/unblock （現有 — 增強）

封鎖/解封。Form data: `token=<hash>`。保持現有路由格式（token 在 form body 中）。

Response: Table partial (刷新當前頁) + OOB toast

### POST /ui/keys/{token}/regenerate （新增）

重新產生 key。Form data: max_budget, tpm_limit, rpm_limit, budget_duration。

Response: Regenerate dialog 內容替換為新 key 展示
