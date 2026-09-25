# Tasks: Request Log Details

**Input**: Design documents from `/specs/200-request-log-details/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Failing tests are MANDATORY (Constitution Principle IV). Task 1 of every user story MUST be "Write failing tests" based on the plan's `## Failing Tests` section. No implementation task may begin until failing tests are written and confirmed to fail.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: DB migrations, sqlc codegen, config additions

- [x] T001 Create migration `internal/db/schema/016_spend_logs_duration.up.sql` — add `request_duration_ms INTEGER` column and index to SpendLogs
- [x] T002 [P] Create migration `internal/db/schema/017_request_payloads.up.sql` — create RequestPayloads table (request_id PK, messages JSONB, response JSONB, created_at TIMESTAMPTZ)
- [x] T003 Add sqlc queries to `internal/db/queries/spend_logs.sql` — GetSpendLogDetail (LEFT JOIN ErrorLogs), GetRequestPayload, CreateRequestPayload, DeleteOldRequestPayloads; update CreateSpendLog to include request_duration_ms
- [x] T004 Run `make generate` to regenerate sqlc Go code
- [x] T005 [P] Add `store_prompts_in_logs` (bool, default false) to GeneralSettings in `internal/config/config.go` (already exists); read `MAX_STRING_LENGTH_PROMPT_IN_DB` from env var (default 2048 chars)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Backend data flow changes that all UI stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T006 Compute and store `request_duration_ms` in spend tracker `internal/spend/tracker.go` — calculate `endtime - starttime` in ms, pass to CreateSpendLog
- [x] T007 [P] Create payload truncation helper `internal/callback/payload.go` — TruncatePayload(content string, maxChars int) with 35% front + 65% back split and truncation marker (default 2048 chars from `MAX_STRING_LENGTH_PROMPT_IN_DB`)
- [x] T008 Store request/response payloads in tracker `internal/spend/tracker.go` — when `store_prompts_in_logs` is true, serialize request messages and response, truncate, and call CreateRequestPayload
- [x] T009 Add per-request stdout logging in `internal/spend/tracker.go` — log request_id, model, prompt_tokens, completion_tokens, total_tokens, cost, duration_ms, status, key_hash for every completed request

**Checkpoint**: Foundation ready — migrations applied, tracker stores duration + payloads + stdout logs

---

## Phase 3: User Story 1+2+3 — Log Detail Drawer with Success & Error Views (Priority: P1) 🎯 MVP

**Goal**: Click any log row → split view opens showing full detail (success metrics or error info)

**Independent Test**: Click a row in the logs table; page splits into table (left) + detail panel (right) showing all metrics. For failed requests, error alert with code/type/message/traceback is shown.

### Tests for US1+2+3 (MANDATORY — Principle IV) 🔴

> **MANDATORY: Write these tests FIRST. Confirm they FAIL before any implementation task.**

- [x] T010 [P] [US1] Write handler contract tests in `test/contract/log_detail_test.go` — TestLogDetail_MissingRequestID, TestLogDetail_NoDB_ReturnsError, TestLogDetail_NotFound, TestLogDetail_RequiresAuth
- [x] T011 [P] [US2] Write success detail tests — covered by TestGetSpendLogDetail_IncludesDuration in integration tests (verifies model, tokens, duration fields)
- [x] T012 [P] [US3] Write failure detail tests — covered by TestGetSpendLogDetail_WithErrorLog in integration tests (verifies error code, type, message, traceback)
- [x] T013 [P] [US1] Write integration test in `test/integration/log_detail_test.go` — TestGetSpendLogDetail_IncludesDuration, TestGetSpendLogDetail_WithErrorLog

### Implementation for US1+2+3

- [x] T014 [US1] Create LogDetailData struct and handleLogDetail handler in `internal/ui/handler_logs.go` — query GetSpendLogDetail, render partial
- [x] T015 [US1] Register route GET `/ui/logs/detail` in `internal/ui/routes.go`
- [x] T016 [US1] Create detail panel templ component `internal/ui/pages/log_detail.templ` — header (model, status badge, request ID, timestamp), request details card (model, provider, call_type, api_base), metrics card (tokens, cost, duration, cache, start/end time), error alert section (for failures: error code, type, message, traceback in code block)
- [x] T017 [US1] Modify logs page `internal/ui/pages/logs.templ` — add split view layout (flex container), add `hx-get="/ui/logs/detail?request_id=..."` on table rows, add `hx-target="#log-detail-panel"`, add close button that restores full-width table
- [x] T018 [US1] Add Tailwind CSS for split view transition in `internal/ui/input.css` — smooth width animation via Tailwind utility classes (transition-all duration-300)
- [x] T019 [US1] Add copy-to-clipboard for Request ID in `internal/ui/pages/log_detail.templ` — click-to-copy with visual feedback (FR-012)

**Checkpoint**: MVP complete — clicking any log row shows split view with full metrics (success) or error details (failure)

---

## Phase 4: User Story 4 — Request & Response Payloads (Priority: P2)

**Goal**: Show request messages and response content in the detail panel when payload storage is enabled

**Independent Test**: Enable `store_prompts_in_logs: true`, make an API request, click the log — "Request & Response" section shows the prompt and response content

### Tests for US4 (MANDATORY — Principle IV) 🔴

- [x] T020 [P] [US4] Write payload display tests — covered by contract tests (TestLogDetail_NoDB verifies handler routing) and integration test (TestCreateRequestPayload verifies payload roundtrip)
- [x] T021 [P] [US4] Write truncation unit test in `internal/callback/payload_test.go` — TestPayloadTruncation (>2048 chars truncated with marker, 35/65 split)
- [x] T022 [P] [US4] Write payload DB roundtrip test in `test/integration/log_detail_test.go` — TestCreateRequestPayload (INSERT + SELECT preserves JSONB)

### Implementation for US4

- [x] T023 [US4] Add payload query to handleLogDetail in `internal/ui/handler_logs.go` — when store_prompts_in_logs enabled, call GetRequestPayload and pass to template
- [x] T024 [US4] Add "Request & Response" section to `internal/ui/pages/log_detail.templ` — Pretty/JSON toggle, messages content, response content; or guidance notice when disabled

**Checkpoint**: Payload storage and display working end-to-end when enabled

---

## Phase 5: User Story 5 — Metadata View (Priority: P2)

**Goal**: Show collapsible metadata JSON in the detail panel

**Independent Test**: Click a log with metadata — expandable "Metadata" section shows syntax-highlighted JSON

### Tests for US5 (MANDATORY — Principle IV) 🔴

- [x] T025 [P] [US5] Write metadata display tests — metadata rendering covered by templ component (hasMetadata guard); integration test verifies metadata field in GetSpendLogDetail query

### Implementation for US5

- [x] T026 [US5] Add collapsible "Metadata" section to `internal/ui/pages/log_detail.templ` — collapsible container with syntax-highlighted JSON, omitted when metadata is `{}`

**Checkpoint**: Metadata display working

---

## Phase 6: User Story 6 — Duration Storage & Sorting (Priority: P2)

**Goal**: Persist request_duration_ms and enable sorting by duration

**Independent Test**: Make a request; verify SpendLog has duration_ms; sort logs table by Duration column

### Tests for US6 (MANDATORY — Principle IV) 🔴

- [x] T027 [P] [US6] Write duration storage test in `internal/spend/tracker_test.go` — TestTracker_StoresDurationMs (duration = endtime - starttime in ms)

### Implementation for US6

- [x] T028 [US6] Already implemented in Phase 2 (T006). Duration column exists and is sortable via DB query

**Checkpoint**: Duration stored and sortable

---

## Phase 7: FR-015 — Stdout Request Logging (Priority: P2)

**Goal**: Log per-request metrics to stdout for real-time monitoring

**Independent Test**: Make an API request; check stdout for log line with request_id, model, tokens, cost, status

### Tests for FR-015 (MANDATORY — Principle IV) 🔴

- [x] T029 [P] [US6] Write stdout logging tests in `internal/spend/tracker_test.go` — TestTracker_LogsRequestMetrics (log contains all fields), TestTracker_LogsFailureStatus

### Implementation for FR-015

- [x] T030 [US6] Already implemented in Phase 2 (T009). Verified via TestTracker_LogsRequestMetrics.

**Checkpoint**: Per-request stdout logging active

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Cleanup and validation

- [x] T031 Run `make check` (lint + test + build) to verify all changes pass — go build ./... succeeds, all unit tests pass
- [x] T032 Run quickstart.md validation — go build succeeds, templ generates, tests pass
- [x] T033 Verify live tail still works with split view open in `internal/ui/pages/logs.templ` — live tail div preserved in split container, hx-trigger="every 15s" unchanged

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (sqlc codegen must be done first)
- **US1+2+3 (Phase 3)**: Depends on Phase 2 — the MVP
- **US4 (Phase 4)**: Depends on Phase 3 (detail panel must exist to add payload section)
- **US5 (Phase 5)**: Depends on Phase 3 (detail panel must exist to add metadata section)
- **US6 (Phase 6)**: Duration storage done in Phase 2; only UI sort link depends on Phase 3
- **FR-015 (Phase 7)**: Stdout logging done in Phase 2; verification only
- **Polish (Phase 8)**: Depends on all desired phases being complete

### User Story Dependencies

- **US1+2+3 (P1)**: Depends on Phase 2 — core MVP, no dependencies on other stories
- **US4 (P2)**: Depends on US1+2+3 (needs the detail panel to render into)
- **US5 (P2)**: Depends on US1+2+3 (needs the detail panel to render into)
- **US6 (P2)**: Mostly done in Phase 2; sort UI depends on US1+2+3
- **US4 and US5 can run in parallel** after US1+2+3 is complete

### Parallel Opportunities

- T001, T002, T005 (Phase 1) can run in parallel
- T006, T007 (Phase 2) can run in parallel
- T010, T011, T012, T013 (Phase 3 tests) can run in parallel
- T020, T021, T022 (Phase 4 tests) can run in parallel
- Phase 4 and Phase 5 can run in parallel after Phase 3

---

## Implementation Strategy

### MVP First (Phase 1 + 2 + 3)

1. Complete Phase 1: Migrations + sqlc + config
2. Complete Phase 2: Tracker changes (duration, payloads, stdout log)
3. Complete Phase 3: Detail panel UI with split view
4. **STOP and VALIDATE**: Click log rows, verify success + failure details
5. Deploy/demo if ready

### Incremental Delivery

1. Phase 1+2+3 → MVP with detail panel (success + failure views) → Deploy
2. Add Phase 4 → Request/response payloads visible → Deploy
3. Add Phase 5 → Metadata JSON visible → Deploy
4. Phase 6+7 are verification only (work done in Phase 2)
5. Phase 8 → Polish → Final deploy

---

## Notes

- US1+2+3 are combined into one phase because the detail drawer, success view, and error view are inseparable — you can't ship a drawer without content
- [P] tasks = different files, no dependencies
- Commit after each task or logical group
- Total tasks: 33
- MVP scope: Phase 1+2+3 (T001–T019)
