# Implementation Plan: Upstream Token in SpendLogs + Key Alias

**Branch**: `001-upstream-token-spendlogs` | **Date**: 2026-04-02 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-upstream-token-spendlogs/spec.md`

## Summary

Add upstream OAuth token tracking to SpendLogs + ErrorLogs, enrich ErrorLogs with missing fields (model, end_user, organization_id), and display key alias in the request log table. Enables operators to diagnose which users/keys are consuming a specific token's quota across both success and failure requests. Requires: DB migration (SpendLogs + ErrorLogs new columns + indexes), spend/error callback pipeline changes, SQL query JOINs for alias resolution, and UI updates (new columns + filter + detail field).

## Technical Context

**Language/Version**: Go 1.26  
**Primary Dependencies**: chi/v5 (router), pgx/v5 (PostgreSQL), sqlc (query codegen), templ + HTMX (UI)  
**Storage**: PostgreSQL (SpendLogs, VerificationToken), Redis (rate limit counters)  
**Testing**: go test + testify  
**Target Platform**: Linux server (Docker/K8s)  
**Project Type**: Web application (Go backend + server-rendered UI)  
**Performance Goals**: Zero increase in p99 request latency  
**Constraints**: Migration must be backward-compatible (DEFAULT '' for new column)  
**Scale/Scope**: ~5,600 request logs visible in current UI; LEFT JOIN VerificationToken adds negligible cost

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | N/A | This is a TianjiLLM-Go-only feature (upstream token tracking doesn't exist in Python LiteLLM) |
| II. Feature Parity | N/A | New feature, not a port |
| III. Research Before Build | PASS | Research phase completed — all paths traced in codebase |
| IV. Failing-Tests-First | PASS | Tests listed below for each user story |
| V. Go Best Practices | PASS | Uses existing patterns (sqlc, templ, callback pipeline) |
| VI. No Stale Knowledge | PASS | All file paths and struct fields verified by reading source |
| VII. sqlc-First DB Access | PASS | All queries in .sql files, `make generate` for codegen |

## Project Structure

### Documentation (this feature)

```text
specs/001-upstream-token-spendlogs/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (files to modify)

```text
internal/
├── db/
│   ├── schema/019_upstream_token.up.sql      # NEW: migration (SpendLogs + ErrorLogs)
│   ├── schema/019_upstream_token.down.sql     # NEW: rollback
│   ├── queries/spend_logs.sql                 # MODIFY: add upstream_token_key to CreateSpendLog
│   ├── queries/error_log.sql                  # MODIFY: add upstream_token_key, end_user, organization_id to InsertErrorLog
│   └── queries/spend_views.sql                # MODIFY: add key_alias JOIN + upstream_token_key to ListRequestLogs
├── callback/
│   └── callback.go                            # MODIFY: add UpstreamTokenKey to LogData
├── spend/
│   └── tracker.go                             # MODIFY: add UpstreamTokenKey to SpendRecord + Record()
├── proxy/
│   ├── middleware/helpers.go                   # MODIFY: add ContextKeyUpstreamToken constant
│   └── handler/
│       ├── native_format.go                   # MODIFY: (1) ctx = context.WithValue(ctx, ContextKeyUpstreamToken, sha256[:12])
│       │                                      #         (2) error path: fill Model with requestModel
│       │                                      #         (3) error path: fill UpstreamTokenKey, EndUser, OrganizationID from ctx
│       ├── chat.go                            # MODIFY: recordErrorLog — fill UpstreamTokenKey, EndUser, OrganizationID from ctx
│       └── handler.go                         # MODIFY: buildBaseLogData() extracts UpstreamTokenKey from ctx
├── ui/
│   ├── pages/logs.templ                       # MODIFY: add KeyAlias + UpstreamToken columns
│   ├── pages/log_detail.templ                 # MODIFY: add UpstreamToken field
│   ├── handler_logs.go                        # MODIFY: map new fields in toLogRow(), add filter param
│   └── handler_logs_test.go                   # NEW: tests
└── test/
    └── contract/native_spend_test.go          # NEW: tests
```

**Structure Decision**: All changes fit within existing directory structure. No new packages needed.

**Files NOT modified (intentionally)**:
- `internal/proxy/middleware/db_validator.go` (`LogAuthError`): Uses partial struct literal for `InsertErrorLogParams`. New fields (`UpstreamTokenKey`, `EndUser`, `OrganizationID`) default to zero value `""`, which is correct — auth failures have no upstream token or end_user context. No change needed.

## Failing Tests

### User Story 1 Tests — Record upstream token in SpendLogs

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestNativeSpend_RecordsUpstreamTokenKey` | `internal/proxy/handler/native_format_test.go` | SpendLog entry has non-empty `upstream_token_key` matching SHA256(apiKey)[:12] after native proxy request | AS-1.1 |
| `TestStandardProvider_EmptyUpstreamToken` | `internal/proxy/handler/chat_test.go` | SpendLog entry has empty `upstream_token_key` after standard provider request | AS-1.2 |
| `TestNativeSpend_MultipleTokens_CorrectMapping` | `internal/proxy/handler/native_format_test.go` | Each SpendLog entry maps to the actual token used by that request when round-robin distributes across tokens | AS-1.3 |
| `TestNativeSpend_FailedRequest_StillRecordsToken` | `internal/proxy/handler/native_format_test.go` | ErrorLog entry has non-empty upstream_token_key when upstream returns error | Edge Case 1 + FR-011 |
| `TestNativeError_RecordsModel` | `internal/proxy/handler/native_format_test.go` | ErrorLog entry has non-empty model field (was previously empty string) | FR-010 |
| `TestNativeError_RecordsEndUserAndOrg` | `internal/proxy/handler/native_format_test.go` | ErrorLog entry has end_user and organization_id from context when available | FR-009 |
| `TestStandardError_RecordsUpstreamToken` | `internal/proxy/handler/chat_test.go` | ErrorLog entry has empty upstream_token_key for standard provider errors (context has no token) | FR-009 consistency |

### User Story 2 Tests — Key alias in request log table

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestListRequestLogs_ReturnsKeyAlias` | `internal/ui/handler_logs_test.go` | ListRequestLogs row includes `key_alias` field matching VerificationToken.key_alias for known key hash | AS-2.1 |
| `TestListRequestLogs_DeletedKey_EmptyAlias` | `internal/ui/handler_logs_test.go` | key_alias is empty string when key hash has no matching VerificationToken | AS-2.2 |
| `TestListRequestLogs_NoKeyHash_EmptyAlias` | `internal/ui/handler_logs_test.go` | Both key_hash and key_alias are empty/dash for auth failure entries | AS-2.3 |

### User Story 3 Tests — Filter by upstream token

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestListRequestLogs_FilterByUpstreamToken` | `internal/ui/handler_logs_test.go` | Only entries with matching upstream_token_key returned when filter active | AS-3.1 |
| `TestListRequestLogs_ClearTokenFilter_ShowsAll` | `internal/ui/handler_logs_test.go` | All entries returned when upstream token filter is nil | AS-3.2 |

### User Story 4 Tests — Upstream token in detail view

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestLogDetail_ShowsUpstreamToken` | `internal/ui/handler_logs_test.go` | LogDetailData includes non-empty UpstreamTokenKey for native proxy entry | AS-4.1 |
| `TestLogDetail_StandardProvider_NoToken` | `internal/ui/handler_logs_test.go` | LogDetailData has empty UpstreamTokenKey for standard provider entry | AS-4.2 |

### Edge Case Tests

Edge Case 5 (deleted key alias) is covered by `TestListRequestLogs_DeletedKey_EmptyAlias` in US2 tests — no separate test needed.

### Verification Command

```bash
# Run all failing tests to confirm they compile and fail:
go test ./internal/proxy/handler/... -run "TestNativeSpend|TestNativeError|TestStandardProvider|TestStandardError" -v
go test ./internal/ui/... -run "TestListRequestLogs_|TestLogDetail_|TestDeletedKey_" -v
```

## Complexity Tracking

No constitution violations. All changes follow existing patterns:
- DB migration: same pattern as 016/017/018
- sqlc query modification: same pattern as GetTopKeysBySpend (already JOINs VerificationToken)
- LogData field addition: same pattern as existing fields (Provider, CallType, etc.)
- templ column addition: same pattern as existing columns in logs.templ
