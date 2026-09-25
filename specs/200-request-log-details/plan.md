# Implementation Plan: Request Log Details

**Branch**: `200-request-log-details` | **Date**: 2026-03-27 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/200-request-log-details/spec.md`

## Summary

Add a detail drawer to the Request Logs page that shows comprehensive request information (metrics, errors, payloads, metadata) when clicking a log row. Store request/response payloads in a separate `RequestPayloads` table (configurable opt-in). Add `request_duration_ms` to SpendLogs. Add per-request stdout logging for token usage monitoring.

## Technical Context

**Language/Version**: Go 1.26
**Primary Dependencies**: chi/v5 (router), templ + templUI (UI), HTMX 2.x (interactions), pgx/v5 (DB), sqlc (query codegen)
**Storage**: PostgreSQL (SpendLogs, ErrorLogs, RequestPayloads tables)
**Testing**: `go test` + `testify`, `httptest` for handler tests
**Target Platform**: Linux server (Kubernetes)
**Project Type**: Server-rendered web application (Go + templ + HTMX)
**Performance Goals**: Detail drawer loads within 1 second; SpendLogs list queries unaffected by payload storage
**Constraints**: Payload truncation at 2048 chars per field (env var `MAX_STRING_LENGTH_PROMPT_IN_DB`); payload storage opt-in via config
**Scale/Scope**: 12k+ existing SpendLogs entries, ~100 logs per active session

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | ✅ Pass | LiteLLM source studied for payload storage, config flags, and UI patterns |
| II. Feature Parity | ✅ Pass | Replicates LiteLLM's log detail view with improved payload storage (separate table) |
| III. Research Before Build | ✅ Pass | research.md completed with 7 decisions, external sources cited |
| IV. Failing-Tests-First | ✅ Pass | Failing tests defined below for all acceptance scenarios |
| V. Go Best Practices | ✅ Pass | Uses interfaces, sqlc, templ components, HTMX patterns |
| VI. No Stale Knowledge | ✅ Pass | PostgreSQL TOAST behavior verified via external sources |
| VII. sqlc-First DB Access | ✅ Pass | All new queries defined in data-model.md as named sqlc queries |

## Project Structure

### Documentation (this feature)

```text
specs/200-request-log-details/
├── plan.md
├── spec.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── ui-endpoints.md
└── checklists/
    └── requirements.md
```

### Source Code (repository root)

```text
internal/
├── db/
│   ├── schema/
│   │   ├── 016_spend_logs_duration.up.sql    # NEW: add request_duration_ms column
│   │   └── 017_request_payloads.up.sql       # NEW: RequestPayloads table
│   └── queries/
│       └── spend_logs.sql                    # MODIFIED: add GetSpendLogDetail, GetRequestPayload, CreateRequestPayload
├── spend/
│   └── tracker.go                           # MODIFIED: compute duration_ms, store payload, stdout log
├── config/
│   └── config.go                            # MODIFIED: add store_prompts_in_logs, max_payload_size_kb
├── ui/
│   ├── pages/
│   │   ├── logs.templ                       # MODIFIED: add drawer container, row click hx-get
│   │   └── log_detail.templ                 # NEW: detail drawer content component
│   ├── handler_logs.go                      # MODIFIED: add handleLogDetail endpoint
│   └── routes.go                            # MODIFIED: add GET /ui/logs/detail route
└── callback/
    └── payload.go                           # NEW: payload truncation helper

test/
├── contract/
│   └── log_detail_test.go                   # NEW: handler tests with mock upstream
└── integration/
    └── log_detail_test.go                   # NEW: full DB integration tests
```

**Structure Decision**: Server-rendered Go application. All changes within existing `internal/` package structure. New UI component in `pages/`, new handler method, new sqlc queries, two new migrations.

**UI Layout**: Split view — same as LiteLLM. When a row is clicked, the table shrinks to ~50% width on the left, and a detail panel appears on the right. NOT a Sheet/drawer overlay. Uses CSS flex/grid layout with HTMX to load the detail partial. Closing the panel restores the table to full width.

## Failing Tests

### User Story 1 Tests (Log Detail Drawer)

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestLogDetail_ReturnsHTMLPartial` | `test/contract/log_detail_test.go` | GET /ui/logs/detail?request_id=X returns 200 with HTML content-type | AS-1.1 |
| `TestLogDetail_NotFound` | `test/contract/log_detail_test.go` | GET /ui/logs/detail?request_id=nonexistent returns 404 | Edge: missing log |
| `TestLogDetail_MissingRequestID` | `test/contract/log_detail_test.go` | GET /ui/logs/detail (no param) returns 400 | Edge: bad request |

### User Story 2 Tests (Success Details)

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestLogDetail_SuccessRequest` | `test/contract/log_detail_test.go` | Response HTML contains model, tokens, cost, duration, cache status | AS-2.1 |
| `TestLogDetail_SuccessWithCacheHit` | `test/contract/log_detail_test.go` | Response HTML shows cache "Hit" badge when cache_hit="True" | AS-2.2 |

### User Story 3 Tests (Failed Request Details)

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestLogDetail_FailedRequest` | `test/contract/log_detail_test.go` | Response HTML contains error code, error type, error message from ErrorLogs | AS-3.1 |
| `TestLogDetail_FailedWithTraceback` | `test/contract/log_detail_test.go` | Response HTML contains traceback in code block | AS-3.1 |
| `TestLogDetail_FailedNoTraceback` | `test/contract/log_detail_test.go` | Response HTML omits traceback section when traceback is empty | AS-3.2 |
| `TestLogDetail_FailedNoErrorLog` | `test/contract/log_detail_test.go` | Failed status shown but error detail section omitted when no ErrorLog exists | Edge: missing ErrorLog |

### User Story 4 Tests (Request/Response Payloads)

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestLogDetail_WithPayload` | `test/contract/log_detail_test.go` | Response HTML contains "Request & Response" section with messages content | AS-4.1 |
| `TestLogDetail_PayloadDisabled` | `test/contract/log_detail_test.go` | Response HTML shows guidance notice when store_prompts_in_logs=false | AS-4.2 |
| `TestPayloadTruncation` | `internal/callback/payload_test.go` | Payload >32KB truncated with marker; keeps 35% front + 65% back | Edge: large payload |
| `TestCreateRequestPayload` | `test/integration/log_detail_test.go` | INSERT + SELECT roundtrip preserves JSONB content | AS-4.1 (data) |

### User Story 5 Tests (Metadata)

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestLogDetail_WithMetadata` | `test/contract/log_detail_test.go` | Response HTML contains "Metadata" section with JSON content | AS-5.1 |
| `TestLogDetail_EmptyMetadata` | `test/contract/log_detail_test.go` | Response HTML omits Metadata section when metadata is '{}' | AS-5.2 |

### User Story 6 Tests (Duration Storage)

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestTracker_StoresDurationMs` | `internal/spend/tracker_test.go` | SpendLog record has request_duration_ms = endtime - starttime in ms | AS-6.1 |
| `TestGetSpendLogDetail_IncludesDuration` | `test/integration/log_detail_test.go` | GetSpendLogDetail returns request_duration_ms field | AS-6.1 (query) |

### FR-015 Tests (Stdout Logging)

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestTracker_LogsRequestMetrics` | `internal/spend/tracker_test.go` | log output contains request_id, model, prompt_tokens, completion_tokens, cost, status | FR-015 |
| `TestTracker_LogsFailureStatus` | `internal/spend/tracker_test.go` | log output contains status=failure for error callbacks | FR-015 |

### Verification Command

```bash
# Run all failing tests to confirm they compile and fail:
go test ./test/contract/... -run "TestLogDetail" -v
go test ./test/integration/... -run "TestLogDetail|TestCreateRequestPayload|TestGetSpendLogDetail" -v
go test ./internal/spend/... -run "TestTracker_StoresDuration|TestTracker_LogsRequest" -v
go test ./internal/callback/... -run "TestPayloadTruncation" -v
```

## Complexity Tracking

No constitution violations. All patterns follow existing codebase conventions.
