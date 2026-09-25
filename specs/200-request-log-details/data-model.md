# Data Model: Request Log Details

**Feature**: 200-request-log-details
**Date**: 2026-03-27

## Entity Relationship

```
SpendLogs 1──0..1 ErrorLogs       (joined by request_id)
SpendLogs 1──0..1 RequestPayloads (joined by request_id)
```

## Entities

### SpendLogs (existing — schema change)

Add one column:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `request_duration_ms` | INTEGER | NULL | Request duration in milliseconds, computed at insert time |

Migration: `016_spend_logs_duration.up.sql`

```sql
ALTER TABLE "SpendLogs" ADD COLUMN request_duration_ms INTEGER;
CREATE INDEX IF NOT EXISTS idx_spend_logs_duration ON "SpendLogs" (request_duration_ms);
```

### RequestPayloads (new table)

| Field | Type | Nullable | Description |
|-------|------|----------|-------------|
| `request_id` | TEXT | NOT NULL, PK | FK to SpendLogs.request_id |
| `messages` | JSONB | NOT NULL, DEFAULT '{}' | Request messages (prompt content) |
| `response` | JSONB | NOT NULL, DEFAULT '{}' | Response content |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | When the payload was stored |

Migration: `017_request_payloads.up.sql`

```sql
CREATE TABLE IF NOT EXISTS "RequestPayloads" (
    request_id TEXT PRIMARY KEY,
    messages JSONB NOT NULL DEFAULT '{}',
    response JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_request_payloads_created ON "RequestPayloads" (created_at);
```

**Storage characteristics**:
- TOAST will be used for large JSONB values — this is acceptable because this table is only queried when viewing a specific log detail (by PK lookup)
- Payload fields are truncated at write time (default 2048 chars, configurable via `MAX_STRING_LENGTH_PROMPT_IN_DB` env var)
- Table can be independently partitioned by `created_at` or vacuumed without affecting SpendLogs

### ErrorLogs (existing — no changes)

Already contains all needed fields:

| Field | Type | Description |
|-------|------|-------------|
| `id` | TEXT, PK | Auto-generated UUID |
| `request_id` | TEXT | Links to SpendLogs |
| `api_key_hash` | TEXT | Hashed API key |
| `model` | TEXT | Model name |
| `provider` | TEXT | Provider name |
| `status_code` | INTEGER | HTTP status code |
| `error_type` | TEXT | Error classification |
| `error_message` | TEXT | Error description |
| `traceback` | TEXT | Stack trace |
| `team_id` | TEXT | Team identifier |
| `created_at` | TIMESTAMPTZ | Timestamp |

### LogDetailView (composite — not persisted)

Assembled at query time by LEFT JOINing SpendLogs + ErrorLogs + RequestPayloads:

| Field | Source | Description |
|-------|--------|-------------|
| All SpendLogs fields | SpendLogs | Core metrics |
| `request_duration_ms` | SpendLogs | Duration (new column) |
| `error_status_code` | ErrorLogs | HTTP error code (NULL for success) |
| `error_type` | ErrorLogs | Error classification (NULL for success) |
| `error_message` | ErrorLogs | Error description (NULL for success) |
| `error_traceback` | ErrorLogs | Stack trace (NULL for success) |
| `payload_messages` | RequestPayloads | Request content (NULL if storage disabled) |
| `payload_response` | RequestPayloads | Response content (NULL if storage disabled) |

## sqlc Queries

### New queries (`internal/db/queries/spend_logs.sql`)

```sql
-- name: GetSpendLogDetail :one
SELECT
    sl.*,
    el.status_code AS error_status_code,
    el.error_type,
    el.error_message,
    el.traceback AS error_traceback
FROM "SpendLogs" sl
LEFT JOIN "ErrorLogs" el ON el.request_id = sl.request_id
WHERE sl.request_id = $1;

-- name: GetRequestPayload :one
SELECT messages, response
FROM "RequestPayloads"
WHERE request_id = $1;

-- name: CreateRequestPayload :exec
INSERT INTO "RequestPayloads" (request_id, messages, response)
VALUES ($1, $2, $3);

-- name: DeleteOldRequestPayloads :exec
DELETE FROM "RequestPayloads"
WHERE created_at < $1;
```

### Modified queries

Update `CreateSpendLog` to include `request_duration_ms` as a 22nd parameter.

## Configuration

Add to `general_settings` in `proxy_config.yaml`:

```yaml
general_settings:
  store_prompts_in_logs: false  # default: false
```

Truncation controlled via environment variable (same as LiteLLM):

```bash
MAX_STRING_LENGTH_PROMPT_IN_DB=2048  # default: 2048 characters
```
