# Tasks: Upstream Token in SpendLogs + Key Alias + ErrorLogs Enrichment

**Input**: Design documents from `/specs/001-upstream-token-spendlogs/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Failing tests are MANDATORY (Constitution Principle IV). Task 1 of every user story MUST be "Write failing tests" based on the plan's `## Failing Tests` section. No implementation task may begin until failing tests are written and confirmed to fail.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: DB migration and sqlc codegen — foundation for all stories

- [x] T001 Create migration up file `internal/db/schema/019_upstream_token.up.sql` — add `upstream_token_key TEXT NOT NULL DEFAULT ''` + index to SpendLogs; add `upstream_token_key`, `end_user`, `organization_id` + index to ErrorLogs (per contracts/migration-019.sql)
- [x] T002 Create migration down file `internal/db/schema/019_upstream_token.down.sql` — reverse all changes from T001
- [x] T003 Add `ContextKeyUpstreamToken` constant in `internal/proxy/middleware/helpers.go` (same pattern as ContextKeyTokenHash, ContextKeyUserID, etc.)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Modify SQL queries + `make generate` → new Go structs used by all stories

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T004 Modify `internal/db/queries/spend_logs.sql` — add `upstream_token_key` to CreateSpendLog INSERT columns and VALUES (becomes $25)
- [x] T005 [P] Modify `internal/db/queries/error_log.sql` — add `upstream_token_key`, `end_user`, `organization_id` to InsertErrorLog INSERT columns and VALUES ($10, $11, $12)
- [x] T006 [P] Modify `internal/db/queries/spend_views.sql` ListRequestLogs — add `LEFT JOIN "VerificationToken"` to both UNION halves, add `COALESCE(vt.key_alias, '') AS key_alias` and `upstream_token_key` SELECT columns, add `filter_upstream_token` WHERE clause to both halves (per contracts/query-changes.sql)
- [x] T007 [P] Modify `internal/db/queries/spend_views.sql` CountRequestLogs — add `filter_upstream_token` WHERE clause to both halves (same pattern as ListRequestLogs)
- [x] T008 [P] Modify `internal/db/queries/spend_logs.sql` GetSpendLogDetail — add `LEFT JOIN "VerificationToken" vt ON sl.api_key = vt.token`, add `COALESCE(vt.key_alias, '') AS key_alias` to SELECT (upstream_token_key auto-included via `sl.*`)
- [x] T009 Run `make generate` to regenerate all sqlc Go code — verify `CreateSpendLogParams`, `InsertErrorLogParams`, `ListRequestLogsRow`, `GetSpendLogDetailRow` all have new fields
- [x] T010 Add `UpstreamTokenKey string` field to `callback.LogData` struct in `internal/callback/callback.go`
- [x] T011 [P] Add `UpstreamTokenKey string` field to `spend.SpendRecord` struct in `internal/spend/tracker.go`; map it in `LogSuccess()` (line ~73) and pass to `CreateSpendLogParams` in `Record()` (line ~142)
- [x] T012 Add `UpstreamTokenKey` extraction to `buildBaseLogData()` in `internal/proxy/handler/handler.go` — read from `ctx.Value(middleware.ContextKeyUpstreamToken)` (same pattern as lines 220-234 for APIKey/UserID/TeamID/OrgID/RequesterIP)

**Checkpoint**: Foundation ready — `make generate` succeeds, all new struct fields available, context extraction wired

---

## Phase 3: User Story 1 — Record upstream token in SpendLogs + ErrorLogs (Priority: P1) 🎯 MVP

**Goal**: Every native proxy request records which upstream OAuth token was used, in both SpendLogs (success) and ErrorLogs (failure). Standard provider path records empty string.

**Independent Test**: Send a request through native proxy, query `SELECT upstream_token_key FROM "SpendLogs" ORDER BY starttime DESC LIMIT 1` — should return non-empty 12-char hex hash.

### Tests for User Story 1 (MANDATORY - Principle IV) 🔴

> **MANDATORY: Write these tests FIRST. Confirm they FAIL before any implementation task.**

- [x] T013 [P] [US1] Write `TestNativeSpend_RecordsUpstreamTokenKey` in `internal/proxy/handler/native_format_test.go` — assert SpendLog entry has non-empty `upstream_token_key` matching SHA256(apiKey)[:12]
- [x] T014 [P] [US1] Write `TestStandardProvider_EmptyUpstreamToken` in `internal/proxy/handler/chat_test.go` — assert SpendLog entry has empty `upstream_token_key`
- [x] T015 [P] [US1] Write `TestNativeSpend_MultipleTokens_CorrectMapping` in `internal/proxy/handler/native_format_test.go` — assert each SpendLog entry maps to the actual token used when round-robin distributes
- [x] T016 [P] [US1] Write `TestNativeSpend_FailedRequest_StillRecordsToken` in `internal/proxy/handler/native_format_test.go` — assert ErrorLog entry has non-empty upstream_token_key when upstream returns non-200
- [x] T017 [P] [US1] Write `TestNativeError_RecordsModel` in `internal/proxy/handler/native_format_test.go` — assert ErrorLog entry has non-empty model field (was previously empty string)
- [x] T018 [P] [US1] Write `TestNativeError_RecordsEndUserAndOrg` in `internal/proxy/handler/native_format_test.go` — assert ErrorLog entry has end_user and organization_id from context
- [x] T019 [P] [US1] Write `TestStandardError_RecordsUpstreamToken` in `internal/proxy/handler/chat_test.go` — assert ErrorLog entry has empty upstream_token_key for standard provider errors
- [x] T020 [US1] Run `go test ./internal/proxy/handler/... -run "TestNativeSpend|TestNativeError|TestStandardProvider|TestStandardError" -v` — confirm all tests compile and FAIL

### Implementation for User Story 1

- [x] T021 [US1] In `internal/proxy/handler/native_format.go` nativeProxy() — after line 58 (`baseURL, apiKey := upstream.BaseURL, upstream.APIKey`), add `ctx = context.WithValue(ctx, middleware.ContextKeyUpstreamToken, tokenKeyFromAPIKey(apiKey))` where tokenKeyFromAPIKey computes sha256[:12] (import crypto/sha256, encoding/hex)
- [x] T022 [US1] In `internal/proxy/handler/native_format.go` ModifyResponse error path (lines 114-132) — fill `Model: requestModel` (was `""`), add `UpstreamTokenKey`, `EndUser`, `OrganizationID` fields to InsertErrorLogParams by extracting from ctx
- [x] T023 [US1] In `internal/proxy/handler/chat.go` recordErrorLog (lines 598-612) — add `UpstreamTokenKey`, `EndUser`, `OrganizationID` fields to InsertErrorLogParams by extracting from ctx
- [x] T024 [US1] Run failing tests again — confirm all T013-T019 now PASS

**Checkpoint**: `upstream_token_key` recorded in SpendLogs + ErrorLogs for native proxy; empty for standard path. All 7 tests pass.

---

## Phase 4: User Story 2 — Show key alias in request log table (Priority: P2)

**Goal**: Request log table displays human-readable key alias (e.g. "sean", "hoh lobster") next to key hash.

**Independent Test**: Visit `/ui/logs` — confirm each row with a known key hash shows the alias from the keys page.

### Tests for User Story 2 (MANDATORY - Principle IV) 🔴

- [x] T025 [P] [US2] Write `TestListRequestLogs_ReturnsKeyAlias` in `internal/ui/handler_logs_test.go` — assert ListRequestLogs row includes key_alias matching VerificationToken.key_alias for known key hash
- [x] T026 [P] [US2] Write `TestListRequestLogs_DeletedKey_EmptyAlias` in `internal/ui/handler_logs_test.go` — assert key_alias is empty when key hash has no matching VerificationToken (deleted key or never existed)
- [x] T027 [P] [US2] Write `TestListRequestLogs_NoKeyHash_EmptyAlias` in `internal/ui/handler_logs_test.go` — assert both key_hash and key_alias are empty for auth failure entries
- [x] T028 [US2] Run `go test ./internal/ui/... -run "TestListRequestLogs_" -v` — confirm all tests compile and FAIL

### Implementation for User Story 2

- [x] T029 [US2] Add `KeyAlias string` and `UpstreamTokenKey string` fields to `RequestLogRow` struct in `internal/ui/pages/logs.templ` (line ~17)
- [x] T030 [US2] Map `KeyAlias` and `UpstreamTokenKey` in `toLogRow()` in `internal/ui/handler_logs.go` (line ~149) from `db.ListRequestLogsRow` new fields
- [x] T031 [US2] Add "Key Alias" column header in `internal/ui/pages/logs.templ` LogsTablePartial (line ~380, after "Key Hash")
- [x] T032 [US2] Add key alias cell rendering in `internal/ui/pages/logs.templ` logRow() (after Key Hash cell, line ~548) — show alias text or "–" if empty
- [x] T033 [US2] Update colspan from "9" to "10" in empty/error states in `internal/ui/pages/logs.templ` (lines ~390, ~398) — Key Alias adds 1 column; Upstream Token column added in US3
- [x] T034 [US2] Run `make ui` to regenerate templ Go files
- [x] T035 [US2] Run failing tests again — confirm all T025-T027 now PASS

**Checkpoint**: Request log table shows Key Alias column. All 3 tests pass.

---

## Phase 5: User Story 3 — Filter by upstream token (Priority: P3)

**Goal**: Operators can filter the request log by upstream token to answer "who is consuming token X's quota?"

**Independent Test**: Filter by `b545c01499c2` on `/ui/logs` — only requests routed through that token appear.

### Tests for User Story 3 (MANDATORY - Principle IV) 🔴

- [x] T036 [P] [US3] Write `TestListRequestLogs_FilterByUpstreamToken` in `internal/ui/handler_logs_test.go` — assert only entries with matching upstream_token_key returned when filter active
- [x] T037 [P] [US3] Write `TestListRequestLogs_ClearTokenFilter_ShowsAll` in `internal/ui/handler_logs_test.go` — assert all entries returned when filter is nil
- [x] T038 [US3] Run `go test ./internal/ui/... -run "TestListRequestLogs_Filter" -v` — confirm tests compile and FAIL

### Implementation for User Story 3

- [x] T039 [US3] Add `FilterUpstreamToken *string` field to `LogsPageData` struct in `internal/ui/pages/logs.templ` (line ~50, after FilterRequestID)
- [x] T040 [US3] Update `FilterQueryString()` and `filterQSWithoutPage()` and `hasActiveFilters()` methods in `internal/ui/pages/logs.templ` to include FilterUpstreamToken
- [x] T041 [US3] In `internal/ui/handler_logs.go` loadLogsPageData() — parse `filter_upstream_token` URL query param, pass to ListRequestLogs and CountRequestLogs params
- [x] T042 [US3] Add "Upstream Token" column header in `internal/ui/pages/logs.templ` LogsTablePartial (after Key Alias column)
- [x] T043 [US3] Add upstream token cell rendering in `internal/ui/pages/logs.templ` logRow() — show token hash or "–" if empty
- [x] T044 [US3] Add upstream token filter input in `internal/ui/pages/logs.templ` filter bar (same pattern as Key Hash filter input)
- [x] T045 [US3] Update colspan from "10" to "11" in empty/error states in `internal/ui/pages/logs.templ` — Upstream Token adds 1 column
- [x] T046 [US3] Run `make ui` to regenerate templ Go files
- [x] T047 [US3] Run failing tests again — confirm all T036-T037 now PASS

**Checkpoint**: Filter by upstream token works. Both success and failure entries filterable. All 2 tests pass.

---

## Phase 6: User Story 4 — Upstream token in log detail view (Priority: P4)

**Goal**: Spend log detail page shows which upstream OAuth token was used.

**Independent Test**: Click a native proxy log entry → detail panel shows upstream token identifier.

### Tests for User Story 4 (MANDATORY - Principle IV) 🔴

- [x] T048 [P] [US4] Write `TestLogDetail_ShowsUpstreamToken` in `internal/ui/handler_logs_test.go` — assert LogDetailData includes non-empty UpstreamTokenKey for native proxy entry
- [x] T049 [P] [US4] Write `TestLogDetail_StandardProvider_NoToken` in `internal/ui/handler_logs_test.go` — assert LogDetailData has empty UpstreamTokenKey for standard provider entry
- [x] T050 [US4] Run `go test ./internal/ui/... -run "TestLogDetail_" -v` — confirm tests compile and FAIL

### Implementation for User Story 4

- [x] T051 [US4] Add `UpstreamTokenKey string` field to `LogDetailData` struct in `internal/ui/pages/log_detail.templ`
- [x] T052 [US4] Map `UpstreamTokenKey` in `handleLogDetail()` in `internal/ui/handler_logs.go` (line ~204) from `GetSpendLogDetailRow`
- [x] T053 [US4] Add upstream token display row in `internal/ui/pages/log_detail.templ` LogDetailPanel (after Key Hash row) — show token or "N/A" if empty
- [x] T054 [US4] Run `make ui` to regenerate templ Go files
- [x] T055 [US4] Run failing tests again — confirm T048-T049 now PASS

**Checkpoint**: Detail view shows upstream token. All 2 tests pass.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Validation, cleanup, full test suite

- [x] T056 Run `make check` (lint + test + build) — confirm zero regressions; manually verify p99 latency post-deploy (FR-007)
- [x] T057 Run `make generate` one final time — confirm all sqlc generated code matches queries
- [x] T058 Run quickstart.md validation steps: send native proxy request → check SpendLogs → check UI → filter → detail
- [x] T059 Verify historical data: confirm pre-migration SpendLog/ErrorLog entries show empty upstream_token_key and "–" for key alias (no data corruption per SC-004)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Phase 2 — BLOCKS US3 and US4 (they need upstream_token_key data to exist)
- **User Story 2 (Phase 4)**: Depends on Phase 2 only — can run in parallel with US1
- **User Story 3 (Phase 5)**: Depends on Phase 2 + Phase 3 (needs upstream_token_key data). Independent of US2.
- **User Story 4 (Phase 6)**: Depends on Phase 2 + Phase 3 (needs upstream_token_key in DB)
- **Polish (Phase 7)**: Depends on all stories complete

### User Story Dependencies

- **US1 (P1)**: Foundation only → MVP, can ship alone
- **US2 (P2)**: Foundation only → independent of US1, can run in parallel
- **US3 (P3)**: Foundation + US1 (needs upstream_token_key data in DB) → sequential after US1. Independent of US2 (Key Alias column is separate).
- **US4 (P4)**: Foundation + US1 (needs data) → sequential after US1

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- SQL/migration before Go code
- Go structs before handler logic
- Handler before templ
- `make ui` after all templ changes
- Story complete before moving to next priority

### Parallel Opportunities

- T001, T002, T003 can run in parallel (Phase 1)
- T004, T005, T006, T007, T008 — T005-T008 can run in parallel after T004
- T013-T019 can all run in parallel (US1 tests)
- T025-T027 can all run in parallel (US2 tests)
- **US1 and US2 can run in parallel** after Phase 2 (different files, no dependencies)
- T036-T037 can run in parallel (US3 tests)
- T048-T049 can run in parallel (US4 tests)

---

## Parallel Example: Phase 2 Foundational

```bash
# After T004 (CreateSpendLog), these can all run in parallel:
Task T005: "Modify error_log.sql InsertErrorLog"
Task T006: "Modify spend_views.sql ListRequestLogs"
Task T007: "Modify spend_views.sql CountRequestLogs"
Task T008: "Modify spend_logs.sql GetSpendLogDetail"

# After T009 (make generate), these can run in parallel:
Task T010: "Add UpstreamTokenKey to callback.LogData"
Task T011: "Add UpstreamTokenKey to spend.SpendRecord + Record()"
```

## Parallel Example: US1 + US2 in parallel

```bash
# After Phase 2, both can start simultaneously:

# Developer A — US1:
Task T013-T019: Write failing tests (parallel)
Task T020: Confirm tests fail
Task T021-T023: Implementation
Task T024: Confirm tests pass

# Developer B — US2:
Task T025-T027: Write failing tests (parallel)
Task T028: Confirm tests fail
Task T029-T034: Implementation
Task T035: Confirm tests pass
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (migration + context key)
2. Complete Phase 2: Foundational (SQL queries + Go structs)
3. Complete Phase 3: User Story 1 (upstream token recording)
4. **STOP and VALIDATE**: Query SpendLogs → confirm upstream_token_key populated
5. Deploy if ready — operators can already use SQL to investigate token usage

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. US1 → upstream token recorded in SpendLogs + ErrorLogs → Deploy (MVP!)
3. US2 → key alias visible in request log table → Deploy
4. US3 → filter by upstream token in UI → Deploy
5. US4 → upstream token in detail view → Deploy
6. Each story adds value without breaking previous stories

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- `make generate` (T009) must run after ALL SQL query changes — do not run it incrementally
- `make ui` must run after ALL templ changes within a story
- `db_validator.go LogAuthError` does NOT need modification — new InsertErrorLogParams fields default to zero value `""` which is correct for auth failures
- Commit after each phase checkpoint
