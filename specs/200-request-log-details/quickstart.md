# Quickstart: Request Log Details

## What This Feature Does

Adds a detail drawer to the Request Logs page. Click any log row to see:
- Full request metrics (tokens, cost, duration, cache status)
- Error details for failed requests (error code, type, message, traceback)
- Request/response content (when enabled)
- Raw metadata JSON

Also adds per-request stdout logging for real-time token usage monitoring.

## Key Files

| Area | Files |
|------|-------|
| **Migration** | `internal/db/schema/016_spend_logs_duration.up.sql`, `017_request_payloads.up.sql` |
| **sqlc queries** | `internal/db/queries/spend_logs.sql` (GetSpendLogDetail, GetRequestPayload, CreateRequestPayload) |
| **UI drawer** | `internal/ui/pages/log_detail.templ` (new) |
| **UI handler** | `internal/ui/handler_logs.go` (add handleLogDetail) |
| **UI routes** | `internal/ui/routes.go` (add GET /ui/logs/detail) |
| **Callback** | `internal/spend/tracker.go` (add duration_ms, payload storage, stdout log) |
| **Config** | `internal/config/config.go` (add store_prompts_in_logs, max_payload_size_kb) |

## How to Test

```bash
# 1. Run migrations
psql -f internal/db/schema/016_spend_logs_duration.up.sql
psql -f internal/db/schema/017_request_payloads.up.sql

# 2. Regenerate sqlc
make generate

# 3. Run tests
go test ./internal/spend/... -run TestLogDetail -v
go test ./internal/ui/... -run TestLogDetail -v

# 4. Run dev server and check the UI
make dev
# Navigate to /ui/logs, click any row — drawer should open
```

## Configuration

```yaml
# proxy_config.yaml
general_settings:
  store_prompts_in_logs: true    # Enable request/response storage (default: false)
  max_payload_size_kb: 32        # Max size per payload field (default: 32)
```

## Architecture

```
Table row click
  → HTMX GET /ui/logs/detail?request_id=XXX
  → handler queries SpendLogs LEFT JOIN ErrorLogs
  → optionally queries RequestPayloads
  → renders LogDetailDrawer templ partial
  → Sheet component opens with content

Request completion (callback)
  → tracker.LogSuccess() computes duration_ms
  → writes SpendLog with duration_ms
  → if store_prompts_in_logs: writes RequestPayload (truncated)
  → logs to stdout: request_id, model, tokens, cost, duration, status
```
