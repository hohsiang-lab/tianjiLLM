# Tasks: Claude Code OAuth Token Usage & Limits

**Input**: Design documents from `/specs/197-claude-code-oauth-usage/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, quickstart.md

**Tests**: Failing tests are MANDATORY (Constitution Principle IV). Task 1 of every user story MUST be "Write failing tests" based on the plan's `## Failing Tests` section. No implementation task may begin until failing tests are written and confirmed to fail.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: New store, Usage API client, and DB migration — shared foundations used by all user stories.

- [x] T001 Create `OAuthUsageStore` interface and in-memory implementation in `internal/callback/oauth_usage_store.go` — fields: `SessionUsage`, `SessionResetAt`, `WeeklyUsage`, `WeeklyResetAt`, `SonnetUsage`, `SonnetResetAt`, `ExtraUsageEnabled/Limit/Used/Utilization`, `OverageStatus`, `OverageDisabledReason`, `OrgID`, `Error`, `FetchedAt`. Methods: `Get(tokenKey)`, `Set(tokenKey, state)`, `SetFromHeaders(tokenKey, headers)`, `GetAll()`, `IsLocked(tokenKey)`, `Lock(tokenKey, until)`
- [x] T002 [P] Create Usage API HTTP client in `internal/callback/oauth_usage_fetch.go` — function `FetchOAuthUsage(token string) (OAuthUsageState, error)` that calls `GET https://api.anthropic.com/api/oauth/usage` with `Authorization: Bearer <token>` and `anthropic-beta: oauth-2025-04-20`, 5s timeout, parses `five_hour/seven_day/extra_usage` JSON response
- [x] T003 [P] Create DB migration `internal/db/schema/014_oauth_token_metadata.up.sql` — table `OAuthTokenMetadata (token_key TEXT PRIMARY KEY, org_id TEXT NOT NULL, updated_at TIMESTAMPTZ NOT NULL DEFAULT now())`
- [x] T004 [P] Create sqlc queries in `internal/db/queries/oauth_token_metadata.sql` — `UpsertOAuthTokenMetadata`, `GetOAuthTokenMetadata`, `GetAllOAuthTokenMetadata`. Run `make generate`
- [x] T005 Add `OAuthUsageStore` field to `Handlers` struct in `internal/proxy/handler/handler.go` and `UIHandler` struct in `internal/ui/handler.go`
- [x] T006 Wire `OAuthUsageStore` creation and injection in `cmd/tianji/main.go` — create `NewInMemoryOAuthUsageStore()`, pass to both `Handlers` and `UIHandler`

**Checkpoint**: Store, API client, DB migration, and wiring ready. No UI or proxy changes yet.

---

## Phase 2: Foundational (Remove Old Widget)

**Purpose**: Delete the old "Anthropic OAuth Rate Limits" widget and its dependencies. MUST complete before new tab can be added.

- [x] T007 Delete old widget handler file `internal/ui/handler_ratelimit.go` (contains `handleRateLimitState`, `buildRateLimitWidgetData`, `toWidgetData`)
- [x] T008 [P] Delete old widget test files: `internal/ui/handler_ratelimit_seed_test.go`, `internal/ui/handler_ratelimit_ho82_test.go`
- [x] T009 [P] Delete old widget templ test files: `internal/ui/pages/usage_ratelimit_test.go`, `internal/ui/pages/usage_ratelimit_ho82_test.go`, `internal/ui/pages/overage_badge_class_test.go`
- [x] T010 Remove `/ui/api/rate-limit-state` route from `internal/ui/routes.go` (line 105)
- [x] T011 Remove `RateLimitWidget` call and `rateLimitTokens` parameter from `internal/ui/pages/usage.templ` — delete `@RateLimitWidget(rateLimitTokens)` (line 257), remove third parameter from `templ UsagePage()` signature, delete `RateLimitWidget` templ function, delete `rateLimitCard` templ function, delete `rateLimitPollingScript` templ function, delete `AnthropicRateLimitWidgetData` struct and helper functions (`rateLimitStatusClass`, `fmtUtilPct`, `utilBarWidth`, `utilBarColor`, `overageBadgeClass`, `overageBadgeLabel`)
- [x] T012 Update `internal/ui/handler_usage.go` — remove `buildRateLimitWidgetData()` call at line 101, update `UsagePage()` call to remove `rlTokens` argument
- [x] T013 Run `make ui` to regenerate templ Go files. Run `make lint && make test` to confirm no broken references

**Checkpoint**: Old widget fully removed. Usage page works with existing 4 tabs. No "Anthropic OAuth Rate Limits" section visible.

---

## Phase 3: User Story 2 — Passive Update from Response Headers (Priority: P1)

**Goal**: Extend response header parsing to capture `7d_sonnet` and `organization-id`, and populate the new `OAuthUsageStore` on every proxy response.

**Independent Test**: Send one proxy request, verify store has 5h + 7d + 7d_sonnet data and org ID in DB.

### Tests for User Story 2 (MANDATORY - Principle IV) 🔴

- [x] T014 [P] [US2] Write failing test `TestParseHeaders_7dSonnet` in `internal/callback/ratelimit_store_test.go` — assert `ParseAnthropicOAuthRateLimitHeaders` extracts `7d_sonnet` utilization/status/reset from headers
- [x] T015 [P] [US2] Write failing test `TestParseHeaders_OrgID` in `internal/callback/ratelimit_store_test.go` — assert `ParseAnthropicOAuthRateLimitHeaders` extracts `anthropic-organization-id`
- [x] T016 [P] [US2] Write failing test `TestProxyResponse_PopulatesUsageStore` in `test/contract/claude_code_usage_test.go` — after proxy response through mock upstream, assert `OAuthUsageStore` has 5h + 7d + 7d_sonnet data
- [x] T017 [P] [US2] Write failing test `TestProxyResponse_MissingOrgID` in `test/contract/claude_code_usage_test.go` — when response lacks `anthropic-organization-id`, store updated without error
- [x] T018 [P] [US2] Write failing test `TestParseHeaders_On429Response` in `internal/callback/ratelimit_store_test.go` — assert headers are parsed correctly when HTTP status is 429 (FR-003: parsing works on all statuses)
- [x] T019 [US2] Run tests, confirm all 5 fail: `go test ./internal/callback/... -run "TestParseHeaders" -v && go test ./test/contract/... -run "TestProxyResponse" -v`

### Implementation for User Story 2

- [x] T020 [US2] Extend `AnthropicOAuthRateLimitState` struct in `internal/callback/ratelimit_store.go` — add fields: `Unified7dSonnetStatus`, `Unified7dSonnetUtilization`, `Unified7dSonnetReset`, `OrganizationID`
- [x] T021 [US2] Extend `ParseAnthropicOAuthRateLimitHeaders()` in `internal/callback/ratelimit_store.go` — parse `anthropic-ratelimit-unified-7d_sonnet-status/utilization/reset` and `anthropic-organization-id`
- [x] T022 [US2] Implement `SetFromHeaders()` on `OAuthUsageStore` in `internal/callback/oauth_usage_store.go` — convert `AnthropicOAuthRateLimitState` fields to `OAuthUsageState` and store
- [x] T023 [US2] Modify `internal/proxy/handler/native_format.go` — after existing `RateLimitStore.Set()` calls (lines 137 and 153), add: `h.OAuthUsageStore.SetFromHeaders(tokenKey, rlState)` and async `go DB.UpsertOAuthTokenMetadata(tokenKey, rlState.OrganizationID)` when org ID is non-empty
- [x] T024 [US2] Run tests, confirm all 5 pass: `go test ./internal/callback/... -run "TestParseHeaders" -v && go test ./test/contract/... -run "TestProxyResponse" -v`

**Checkpoint**: Every proxy response now populates `OAuthUsageStore` with 5h, 7d, 7d_sonnet data + org ID persisted to DB.

---

## Phase 4: User Story 1 — View Per-Token Plan Usage Limits (Priority: P1) 🎯 MVP

**Goal**: New "Claude Code" tab on the Usage page displaying per-token progress bars for session, weekly, and sonnet-only utilization.

**Independent Test**: Navigate to Usage → Claude Code tab, see one card per configured OAuth token with 3 progress bars.

### Tests for User Story 1 (MANDATORY - Principle IV) 🔴

- [x] T025 [P] [US1] Write failing test `TestClaudeCodeTab_TwoTokens` in `internal/ui/handler_claude_code_test.go` — GET /ui/usage?tab=claude-code returns HTML with 2 token cards containing 3 progress bars each
- [x] T026 [P] [US1] Write failing test `TestClaudeCodeTab_DataFromStore` in `internal/ui/handler_claude_code_test.go` — when store has data, loads instantly without API call
- [x] T027 [P] [US1] Write failing test `TestClaudeCodeTab_ColdStartFallback` in `internal/ui/handler_claude_code_test.go` — when store empty, backend fetches from Usage API mock
- [x] T028 [P] [US1] Write failing test `TestClaudeCodeTab_ColdStart429` in `internal/ui/handler_claude_code_test.go` — when store empty and API returns 429, card shows "No data yet"
- [x] T029 [P] [US1] Write failing test `TestClaudeCodeTab_NoTokensConfigured` in `internal/ui/handler_claude_code_test.go` — no OAuth tokens → "No OAuth tokens configured"
- [x] T030 [P] [US1] Write failing test `TestOldWidgetRemoved` in `internal/ui/handler_claude_code_test.go` — GET /ui/api/rate-limit-state returns 404
- [x] T031 [US1] Run tests, confirm all 6 fail: `go test ./internal/ui/... -run "TestClaudeCode|TestOldWidget" -v`

### Implementation for User Story 1

- [x] T032 [US1] Add "claude-code" case to `activeTab()` in `internal/ui/handler_usage.go` (line 69)
- [x] T033 [US1] Create `internal/ui/handler_claude_code.go` — implement `loadClaudeCodeData()` that reads `OAuthUsageStore.GetAll()` + `DB.GetAllOAuthTokenMetadata()`, and for tokens with empty store data, calls `FetchOAuthUsage()` as cold-start fallback
- [x] T034 [US1] Add "claude-code" case to `handleUsageTab()` in `internal/ui/handler_usage.go` — call `loadClaudeCodeData()` and render `UsageClaudeCodeTab()`
- [x] T035 [US1] Add `/ui/api/claude-code-usage` route in `internal/ui/routes.go` for manual refresh JSON endpoint
- [x] T036 [US1] Create `ClaudeCodeTabData` struct and `ClaudeCodeTokenCard` struct in `internal/ui/pages/usage_claude_code.templ` — implement `UsageClaudeCodeTab()` templ component with per-token cards, 3 progress bars (5h, 7d, 7d_sonnet), percentage labels, color coding, reset countdowns, org ID title, "Last updated" timestamp, and manual refresh button
- [x] T037 [US1] Add `@usageTabTrigger("claude-code", "Claude Code", data)` to tab bar in `internal/ui/pages/usage.templ` (after "endpoint-activity" trigger, around line 264)
- [x] T038 [US1] Add `case ClaudeCodeTabData` to the `switch t := tab.(type)` in `UsagePage()` content area in `internal/ui/pages/usage.templ`
- [x] T039 [US1] Run `make ui` to regenerate templ. Run `make lint && make test` to confirm all tests pass
- [x] T040 [US1] Run full verification: `make check` (lint + test + build)

**Checkpoint**: Claude Code tab is fully functional. Progress bars show data from proxy responses. Cold-start fallback works. Old widget gone.

---

## Phase 5: User Story 3 — Organization ID Capture & Persistence (Priority: P2)

**Goal**: Token cards show org ID instead of hash prefix. Org ID survives restarts via DB persistence.

**Independent Test**: Send one proxy request, restart TianjiLLM, visit Claude Code tab — org ID still displayed.

### Tests for User Story 3 (MANDATORY - Principle IV) 🔴

- [x] T041 [P] [US3] Write failing test `TestOrgIDCapture_PersistsToDatabase` in `test/contract/claude_code_usage_test.go` — after proxy response with `anthropic-organization-id` header, assert DB row exists
- [x] T042 [P] [US3] Write failing test `TestClaudeCodeTab_ShowsOrgID` in `internal/ui/handler_claude_code_test.go` — when DB has org_id, card title shows org ID not hash
- [x] T043 [P] [US3] Write failing test `TestClaudeCodeTab_OrgIDSurvivesRestart` in `internal/ui/handler_claude_code_test.go` — when store empty but DB has org_id, card shows org ID
- [x] T044 [P] [US3] Write failing test `TestClaudeCodeTab_FallbackToHash` in `internal/ui/handler_claude_code_test.go` — when DB has no org_id, card shows hash prefix
- [x] T045 [P] [US3] Write failing test `TestUpsertOAuthTokenMetadata` in `test/contract/claude_code_usage_test.go` — upsert creates row, second upsert updates org_id
- [x] T046 [US3] Run tests, confirm all 5 fail

### Implementation for User Story 3

- [x] T047 [US3] Update `loadClaudeCodeData()` in `internal/ui/handler_claude_code.go` — query `DB.GetAllOAuthTokenMetadata()` and merge org IDs into card data
- [x] T048 [US3] Update `ClaudeCodeTokenCard` in `internal/ui/pages/usage_claude_code.templ` — display org ID as card title when available, fall back to hash prefix
- [x] T049 [US3] Run tests, confirm all 5 pass
- [x] T050 [US3] Run full verification: `make check`

**Checkpoint**: Token cards show org IDs. DB persistence works across restarts.

---

## Phase 6: Edge Cases & Usage API Fetch Tests

**Purpose**: Cover remaining edge cases from plan's failing tests section.

- [x] T051 [P] Write failing test `TestFetchOAuthUsage_401` in `internal/callback/oauth_usage_fetch_test.go` — API returns 401, returns error state
- [x] T052 [P] Write failing test `TestFetchOAuthUsage_Timeout` in `internal/callback/oauth_usage_fetch_test.go` — API times out, returns error state
- [x] T053 [P] Write failing test `TestFetchOAuthUsage_429NoRetryAfter` in `internal/callback/oauth_usage_fetch_test.go` — API returns 429 without Retry-After, returns rate-limited error
- [x] T054 [P] Write failing test `TestOAuthUsageStore_CacheTTL` in `internal/callback/oauth_usage_store_test.go` — after 180s, data marked stale
- [x] T055 Run edge case tests, confirm they fail: `go test ./internal/callback/... -run "TestFetchOAuthUsage|TestOAuthUsageStore_CacheTTL" -v`
- [x] T056 Implement edge case handling in `FetchOAuthUsage()` and `OAuthUsageStore` to make all tests pass
- [x] T057 Run full verification: `make check`

**Checkpoint**: All edge cases covered. Full test suite green.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final cleanup and documentation.

- [x] T058 Run `make e2e` (if PostgreSQL available) to verify Claude Code tab in browser
- [x] T059 Run quickstart.md validation — follow all verification steps manually
- [x] T060 Commit and push all changes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Remove Old Widget)**: Depends on T005 (struct fields added) — but can mostly run in parallel with Phase 1
- **Phase 3 (US2 — Headers)**: Depends on Phase 1 completion (store exists)
- **Phase 4 (US1 — UI Tab)**: Depends on Phase 2 (old widget removed) + Phase 3 (store populated)
- **Phase 5 (US3 — Org ID)**: Depends on Phase 4 (tab exists) + T003-T004 (DB migration)
- **Phase 6 (Edge Cases)**: Depends on Phase 1 (fetch client exists)
- **Phase 7 (Polish)**: Depends on all phases

### User Story Dependencies

- **US2 (Headers)**: Can start after Phase 1 — no UI dependency
- **US1 (Tab)**: Depends on US2 (needs store populated by headers) + Phase 2 (old widget removed)
- **US3 (Org ID)**: Depends on US1 (tab must exist to show org ID)

### Parallel Opportunities

Phase 1: T002, T003, T004 can run in parallel (different files)
Phase 2: T007-T009 can run in parallel (all deletions)
Phase 3: T014-T017 can run in parallel (different test files)
Phase 4: T024-T029 can run in parallel (all test writing)
Phase 5: T040-T044 can run in parallel (all test writing)
Phase 6: T050-T053 can run in parallel (different test files)

---

## Implementation Strategy

### MVP First (Phase 1 + 2 + 3 + 4)

1. Setup shared infrastructure (store, fetch client, migration)
2. Remove old widget
3. Extend header parsing + populate store
4. Build Claude Code tab UI
5. **STOP and VALIDATE**: Tab shows real data from proxy responses + cold-start fallback

### Incremental Delivery

1. Phase 1-4 → MVP with working Claude Code tab
2. Phase 5 → Add org ID display (nice-to-have)
3. Phase 6 → Harden edge cases
4. Phase 7 → Polish and ship

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- US2 is implemented before US1 because it provides the data US1 needs to display
- The Usage API (`/api/oauth/usage`) is cold-start fallback only — response headers are the primary data source
- `ParseAnthropicOAuthRateLimitHeaders` is extended in-place, not replaced — existing consumers (Discord alerter, throttle) are unaffected
- `make ui` must be run after any `.templ` file changes to regenerate `_templ.go` files
