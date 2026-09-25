# Data Model: Remove Codex failed-SSE synthetic cooldown

## No new data model

This feature removes a transient proxy-side state write. It adds no database table, column, migration, serialized field, cache entry, or public response field.

## Existing state that remains unchanged

### OpenAI quota state

- **Owner**: existing `OpenAIQuotaState` and handler-local rate-limit map.
- **Purpose**: represent official upstream quota/header signals and existing mock/credential states used by routing.
- **Preserved fields**: status, request/token dimensions, reset deadlines, utilization, subject ID, and update time.
- **Change boundary**: this feature no longer creates a rejected state from a post-commit Codex failed SSE. Existing independently recorded states continue to gate candidates.

### Codex failed-SSE event

- **Owner**: existing parser in `internal/proxy/handler/responses.go`.
- **Purpose**: safe failure detection and logging after response passthrough.
- **Fields used**: event type, response status/id, safe error type/code/message.
- **Persistence**: none; no raw event body or credential material is added to storage.

### Subscription credential resolution

- **Owner**: existing OpenAI subscription routing path.
- **Purpose**: select usable credentials and report independent disabled/rate-limited/auth failures.
- **Change boundary**: no new status is introduced and no existing independent status is cleared.

## State transitions

```text
HTTP 200/SSE copied to client
  -> failed event parsed/logged
  -> no synthetic quota-state write
  -> future resolution sees only pre-existing independent state
```

No migration or generated data changes are required.
