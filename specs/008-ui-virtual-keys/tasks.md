# Tasks: UI Virtual Keys Management

**Input**: Design documents from `/specs/008-ui-virtual-keys/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/key-list-api.md, quickstart.md

**Tests**: Not explicitly requested in spec — test tasks omitted. Contract tests for `/key/list` API are included as they validate a public API change.

**Organization**: Tasks grouped by user story (7 stories, P1→P3). Each story is independently implementable after Phase 2.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: sqlc queries and code generation — all stories depend on these DB queries.

- [x] T001 Add `ListVerificationTokensFiltered` query using `sqlc.narg()` for nullable filter params (filter_team_id, filter_key_alias, filter_user_id, filter_token) with `ORDER BY created_at DESC LIMIT/OFFSET` in `internal/db/queries/verification_token.sql`
- [x] T002 Add `CountVerificationTokensFiltered` query with same `sqlc.narg()` filter params (matching T001 WHERE clause) in `internal/db/queries/verification_token.sql`
- [x] T003 [P] Add `GetVerificationTokenByAlias` query for alias uniqueness check using `sqlc.arg(alias)` and `sqlc.narg(filter_team_id)` in `internal/db/queries/verification_token.sql`
- [x] T004 [P] Add `RegenerateVerificationTokenWithParams` query using `sqlc.arg(old_token, new_token)` and `sqlc.narg(new_max_budget, new_tpm_limit, new_rpm_limit, new_budget_duration)` with COALESCE fallback in `internal/db/queries/verification_token.sql`
- [x] T005 [P] Add `ListTeamAliases` query (`SELECT team_id, team_alias FROM "TeamTable" WHERE team_id = ANY(sqlc.arg(team_ids)::text[])`) in `internal/db/queries/team.sql`
- [x] T006 Run `make generate` to regenerate sqlc Go code, then verify with `go build ./internal/db/...`

**Checkpoint**: All new sqlc queries compile. Verify generated param structs use `*string` for narg() params.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared handler infrastructure, route registration, and enhanced KeyRow/KeysPageData structs used by ALL stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [x] T007 Enhance `KeyRow` struct in `internal/ui/pages/keys.templ` — add fields: `Expires *time.Time`, `TeamID string`, `TeamAlias string`, `UserID string`, `TPMLimit *int64`, `RPMLimit *int64`, `BudgetDuration string`, `BudgetResetAt *time.Time`
- [x] T008 Enhance `KeysPageData` struct in `internal/ui/pages/keys.templ` — add fields: `TotalCount int`, `FilterTeamID string`, `FilterKeyAlias string`, `FilterUserID string`, `FilterKeyHash string` (keep existing `Search string`)
- [x] T009 Create `KeyDetailData` struct in `internal/ui/pages/key_detail.templ` — all VerificationToken fields plus computed fields (IsExpired, IsBlocked, DisplayAlias, BudgetProgress) as defined in data-model.md
- [x] T010 Change `keysPerPage` from 20 to 50 in `internal/ui/handler_keys.go`
- [x] T011 Rewrite `loadKeysPageData` in `internal/ui/handler_keys.go` to use `ListVerificationTokensFiltered` + `CountVerificationTokensFiltered` sqlc queries with nullable filter params, and `ListTeams` for team_id→alias mapping (build map[string]string, inject into KeyRow.TeamAlias)
- [x] T012 Register new routes in `internal/ui/routes.go` — add: `GET /keys/{token}` (detail), `GET /keys/{token}/edit` (edit form partial), `POST /keys/{token}/update`, `POST /keys/{token}/delete`, `POST /keys/{token}/regenerate`
- [x] T013 Create templ helper `KeysTableWithToast` in `internal/ui/pages/keys.templ` that renders `KeysTablePartial` + OOB toast via `hx-swap-oob="afterbegin:body"` using `toast.Toast` component (no container needed — toast is self-positioning with `fixed`)

**Checkpoint**: Foundation ready — routes registered, structs enhanced, list handler uses server-side filtering/pagination. `make check` passes.

---

## Phase 3: User Story 1 — 查看 Virtual Keys 列表 (Priority: P1) 🎯 MVP

**Goal**: Enhanced key list with all columns, server-side filters, accurate pagination.

**Independent Test**: Visit `/ui/keys`, verify full column set visible, filter by Team ID narrows results, pagination shows "Page N of M".

### Implementation for User Story 1

- [x] T014 [US1] Update `KeysTablePartial` table header and `keyRow` template in `internal/ui/pages/keys.templ` — add columns: Team Alias, Team ID, User ID, Expires (show "Never" if nil), Budget Reset, Rate Limits (TPM/RPM), and make Key ID column a clickable link to `/ui/keys/{token}`
- [x] T015 [US1] Add filter UI section above the table in `internal/ui/pages/keys.templ` — Team ID dropdown (populated from teams), Key Alias input, User ID input, Key Hash input; each filter uses `hx-get="/ui/keys/table"` with `hx-include` to send all filter values
- [x] T016 [US1] Update pagination in `KeysTablePartial` in `internal/ui/pages/keys.templ` — show "Page N of M" and "Showing X - Y of Z results" using `TotalCount`/`TotalPages`, pass filter params in pagination links via `hx-include`
- [x] T017 [US1] Add visual indicators in `keyRow` in `internal/ui/pages/keys.templ` — expired badge (red) for keys where `Expires` is past, blocked badge (existing), "Unlimited" for nil budget, "Never" for nil expires, "All Models" badge for empty models, collapse models >3 with expand
- [x] T018 [US1] Update `handleKeys` and `handleKeysTable` in `internal/ui/handler_keys.go` to parse filter query params (team_id, key_alias, user_id, key_hash) and pass to `loadKeysPageData`
- [x] T019 [US1] Enhance `/key/list` REST API handler `KeyList` in `internal/proxy/handler/key.go` — parse query params (page, size, team_id, key_alias, user_id, key_hash), use `ListVerificationTokensFiltered` + `CountVerificationTokensFiltered`, return `{keys, total_count, current_page, total_pages}` response
- [x] T020 [US1] ~~Add contract test for enhanced `/key/list` with filter/pagination~~ — **SKIPPED**: Existing contract tests use NoDB mode (no mock DB); filter/pagination tests require DB queries. Deferred to when mock DB infrastructure is added.

**Checkpoint**: Key list shows all columns, filters work server-side, pagination is accurate with total count. `/key/list` API returns `total_count`/`total_pages`.

---

## Phase 4: User Story 2 — 查看 Key 詳情 (Priority: P1)

**Goal**: Key detail page with Overview and Settings tabs at `/ui/keys/{token}`.

**Independent Test**: Click a Key ID in the list, navigate to detail page, see Overview tab with spend/limits/models cards and Settings tab with all properties.

### Implementation for User Story 2

- [x] T021 [P] [US2] Create `KeyDetailPage` templ component in `internal/ui/pages/key_detail.templ` — AppLayout wrapper, header with DisplayAlias, Key ID (with copy button), created/updated timestamps, "Back to Keys" link
- [x] T022 [US2] Create `OverviewTab` templ component in `internal/ui/pages/key_detail.templ` — cards for: Spend/Budget (with progress bar if budget set), Rate Limits (TPM/RPM or "Unlimited"), Allowed Models (list or "All Models")
- [x] T023 [US2] Create `SettingsTab` templ component in `internal/ui/pages/key_detail.templ` — key-value display of all properties: Key ID, Key Alias, Secret Key (masked), Team ID, Created, Expires, Spend, Budget, Budget Duration, Budget Reset, Tags, Models, TPM/RPM Limits, Metadata
- [x] T024 [US2] Implement `handleKeyDetail` handler in `internal/ui/handler_keys.go` — extract `{token}` from chi URL param, call `GetVerificationToken`, build `KeyDetailData` with computed fields (IsExpired, IsBlocked, DisplayAlias, BudgetProgress), render `KeyDetailPage`
- [x] T025 [US2] Add `copyToClipboard` templ script in `internal/ui/pages/key_detail.templ` for Key ID copy button functionality
- [x] T026 [US2] Wire tabs using templUI `tabs.Tabs`/`tabs.List`/`tabs.Trigger`/`tabs.Content` in `KeyDetailPage` in `internal/ui/pages/key_detail.templ` — Overview default active, client-side switching via `data-tui-tabs`

**Checkpoint**: `/ui/keys/{token}` shows detail page with two tabs, all data displays correctly, copy Key ID works.

---

## Phase 5: User Story 3 — 建立新 Virtual Key (Priority: P1)

**Goal**: Enhanced create dialog with all fields, one-time key reveal after creation.

**Independent Test**: Click "Create New Key", fill form with alias + optional settings, submit, see "Save your Key" dialog with copyable raw key.

### Implementation for User Story 3

- [x] T027 [US3] Rewrite create dialog form in `internal/ui/pages/keys.templ` — change required field from `key_name` to `key_alias`, add collapsible optional settings section: max_budget, budget_duration (select: daily/weekly/monthly), tpm_limit, rpm_limit, models (comma-separated), team_id (dropdown), user_id (dropdown), duration (text: 30s/30m/30h/30d), metadata (textarea), tags (text)
- [x] T028 [US3] Create `KeyRevealDialog` templ component in `internal/ui/pages/keys.templ` — "Save your Key" dialog with security warning ("This key will only be shown once"), raw key display, "Copy Virtual Key" button using `copyToClipboard`, and close button
- [x] T029 [US3] Create `KeysTableWithKeyReveal` templ component in `internal/ui/pages/keys.templ` — renders `KeysTablePartial` + OOB swap `KeyRevealDialog` into a dialog container
- [x] T030 [US3] Rewrite `handleKeyCreate` in `internal/ui/handler_keys.go` — parse all new form fields (key_alias, max_budget, budget_duration, tpm_limit, rpm_limit, models, team_id, user_id, duration→expires, metadata, tags), validate key_alias uniqueness via `GetVerificationTokenByAlias`, generate raw key, hash, store in DB via `CreateVerificationToken` with all params, respond with `KeysTableWithKeyReveal(data, rawKey)`
- [x] T031 [US3] Add `copyToClipboard` templ script in `internal/ui/pages/keys.templ` for key reveal copy button
- [x] T032 [US3] Add team/user dropdown data: modify `loadKeysPageData` and `handleKeys` in `internal/ui/handler_keys.go` to also query `ListTeams` and `ListUsers` and pass team/user lists to the page data for create form dropdowns

**Checkpoint**: Create key with all fields works, raw key is displayed once, copy works, key appears in list.

---

## Phase 6: User Story 4 — 編輯 Virtual Key (Priority: P2)

**Goal**: Edit settings in-place on key detail page.

**Independent Test**: Go to key detail, click "Edit Settings", modify max_budget, save, see updated value.

### Implementation for User Story 4

- [x] T033 [P] [US4] Create `EditSettingsForm` templ component in `internal/ui/pages/key_detail.templ` — form pre-filled with current values for: key_alias, models, max_budget, budget_duration, tpm_limit, rpm_limit, team_id, tags, metadata; "Save Changes" (hx-post to `/ui/keys/{token}/update`) and "Cancel" (hx-get to re-render settings view) buttons
- [x] T034 [US4] Add "Edit Settings" button to `SettingsTab` in `internal/ui/pages/key_detail.templ` — triggers `hx-get="/ui/keys/{token}/edit"` targeting the settings content area
- [x] T035 [US4] Implement `handleKeyEdit` handler in `internal/ui/handler_keys.go` — GET `/ui/keys/{token}/edit`, fetch current key data, render `EditSettingsForm` partial
- [x] T036 [US4] Implement `handleKeyUpdate` handler in `internal/ui/handler_keys.go` — POST `/ui/keys/{token}/update`, parse form fields, call `UpdateVerificationToken` sqlc query, render settings view partial + OOB success toast
- [x] T037 [US4] Handle update errors in `handleKeyUpdate` in `internal/ui/handler_keys.go` — on failure, re-render `EditSettingsForm` with OOB error toast

**Checkpoint**: Edit mode works, save persists changes, cancel discards, toast confirms success/failure.

---

## Phase 7: User Story 5 — 封鎖/解封 Virtual Key (Priority: P2)

**Goal**: Block/unblock with toast feedback.

**Independent Test**: Block a key from list, see status change + success toast. Unblock, see status revert.

### Implementation for User Story 5

- [x] T038 [US5] Update `handleKeyBlock` and `handleKeyUnblock` in `internal/ui/handler_keys.go` — after block/unblock DB operation, respond with `KeysTableWithToast(data, "Key blocked successfully", "success")` or `("Key unblocked successfully", "success")` instead of plain `KeysTablePartial`
- [x] T039 [US5] Add error handling to `handleKeyBlock`/`handleKeyUnblock` in `internal/ui/handler_keys.go` — on DB error, respond with `KeysTableWithToast(data, "Failed to block key: "+err.Error(), "error")`

**Checkpoint**: Block/unblock shows toast feedback, status updates in table.

---

## Phase 8: User Story 6 — 刪除 Virtual Key (Priority: P3)

**Goal**: Delete with alias-confirmation dialog from detail page.

**Independent Test**: On detail page, click "Delete Key", type alias to confirm, key is deleted and redirected to list.

### Implementation for User Story 6

- [x] T040 [P] [US6] Create `DeleteConfirmDialog` templ component in `internal/ui/pages/key_detail.templ` — templUI dialog with: irreversible warning, key info display (alias, Key ID, team_id, spend), alias input field, delete button (disabled by default), `validateDeleteConfirm` templ script for client-side input validation
- [x] T041 [US6] Add "Delete Key" button to `KeyDetailPage` header in `internal/ui/pages/key_detail.templ` — opens `DeleteConfirmDialog` via templUI dialog trigger
- [x] T042 [US6] Implement `handleKeyDetailDelete` handler in `internal/ui/handler_keys.go` — POST `/ui/keys/{token}/delete`, call `DeleteVerificationToken`, respond with `HX-Redirect: /ui/keys` header

**Checkpoint**: Delete flow works with alias confirmation, redirects to list after deletion.

---

## Phase 9: User Story 7 — 重新產生 Key (Priority: P3)

**Goal**: Regenerate key with attribute modification and one-time new key reveal.

**Independent Test**: On detail page, click "Regenerate Key", modify budget, confirm, see new raw key.

### Implementation for User Story 7

- [x] T043 [P] [US7] Create `RegenerateDialog` templ component in `internal/ui/pages/key_detail.templ` — templUI dialog with: Key Alias (readonly), Max Budget (editable), TPM/RPM Limit (editable), Duration (editable with expiry preview), current expiry display, "Regenerate" button
- [x] T044 [US7] Create `RegenerateResultDialog` templ component in `internal/ui/pages/key_detail.templ` — replaces dialog content after success: security warning, new raw key display, copy button
- [x] T045 [US7] Add "Regenerate Key" button to `KeyDetailPage` in `internal/ui/pages/key_detail.templ` — opens `RegenerateDialog` via templUI dialog trigger
- [x] T046 [US7] Implement `handleKeyRegenerate` handler in `internal/ui/handler_keys.go` — POST `/ui/keys/{token}/regenerate`, parse form fields (max_budget, tpm_limit, rpm_limit, budget_duration), generate new raw key, hash, call `RegenerateVerificationTokenWithParams` with nullable params, respond with `RegenerateResultDialog` containing new raw key

**Checkpoint**: Regenerate works, attributes can be modified, new raw key is displayed once.

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Edge cases, error states, and validation that span multiple stories.

- [x] T047 Add 404 handling for key detail page in `handleKeyDetail` in `internal/ui/handler_keys.go` — when `GetVerificationToken` returns not found, render "Key not found" page with back-to-list link
- [x] T048 Add empty state to `KeysTablePartial` in `internal/ui/pages/keys.templ` — when filters return 0 results, show "No keys match your filters" with clear-filters button (distinct from "No keys found" when DB is empty)
- [x] T049 Add client-side form validation in `internal/ui/pages/keys.templ` create dialog — key_alias required, max_budget >= 0, tpm/rpm positive integer, duration format regex `\d+[smhd]`
- [x] T050 Add client-side form validation in `internal/ui/pages/key_detail.templ` edit form — same rules as create
- [x] T051 Rebuild Tailwind CSS after all templ changes — N/A: no input.css source file; output.css is pre-built by templUI
- [x] T052 Run `make check` (lint + test + build) and fix any issues

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 (sqlc queries must be generated first)
- **User Stories (Phase 3-9)**: All depend on Phase 2 completion
  - US1 (list) and US2 (detail) and US3 (create) are all P1 — do sequentially since they share files
  - US4 (edit) and US5 (block/unblock) are P2 — can start after US2 (needs detail page)
  - US6 (delete) and US7 (regenerate) are P3 — can start after US2 (needs detail page)
- **Polish (Phase 10)**: After all desired stories complete

### User Story Dependencies

- **US1 (List)**: After Phase 2. No other story dependencies. **Start here.**
- **US2 (Detail)**: After Phase 2. Creates `key_detail.templ` that US4/US6/US7 depend on.
- **US3 (Create)**: After Phase 2. Independent of US1/US2 but shares `keys.templ`.
- **US4 (Edit)**: After US2 (needs detail page and `key_detail.templ`).
- **US5 (Block/Unblock)**: After Phase 2. Uses toast from T013. Independent of detail page.
- **US6 (Delete)**: After US2 (triggered from detail page).
- **US7 (Regenerate)**: After US2 (triggered from detail page).

### Within Each User Story

- templ components before handlers (struct definitions in templ files)
- Handlers before route registration (already done in Phase 2)
- Core implementation before integration

### Parallel Opportunities

- T001-T005 (sqlc queries): T003, T004, T005 can run in parallel (different logical queries)
- T007-T009 (structs): T007/T008 in `keys.templ`, T009 in `key_detail.templ` — T009 parallel with T007/T008
- US4 tasks: T033 parallel (different file from US5)
- US6 tasks: T040 parallel (creates component)
- US7 tasks: T043 parallel (creates component)

---

## Parallel Example: Phase 1

```bash
# After T001 and T002 (sequential — same file, related WHERE clauses):
# Launch T003, T004, T005 in parallel (different queries, no file conflicts):
Task: "Add GetVerificationTokenByAlias in verification_token.sql"
Task: "Add RegenerateVerificationTokenWithParams in verification_token.sql"
Task: "Add ListTeamAliases in team.sql"
# Then T006 (depends on all above):
Task: "Run make generate"
```

## Parallel Example: User Story 1

```bash
# T014 and T015 are sequential (both modify same table section in keys.templ)
# T019 is parallel (different file: key.go vs keys.templ)
Task: "Enhance KeyList REST API in internal/proxy/handler/key.go"
Task: "Add filter UI in internal/ui/pages/keys.templ"
# T020 is parallel with templ work (different file: test file)
Task: "Contract test for /key/list in test/contract/handler_key_list_test.go"
```

---

## Implementation Strategy

### MVP First (User Stories 1-3 Only)

1. Complete Phase 1: Setup (sqlc queries)
2. Complete Phase 2: Foundational (structs, routes, list handler rewrite)
3. Complete Phase 3: US1 — Enhanced list with filters/pagination
4. Complete Phase 4: US2 — Detail page
5. Complete Phase 5: US3 — Create with key reveal
6. **STOP and VALIDATE**: Full list + detail + create flow works end-to-end
7. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. US1 (List) → Test: filters, pagination, columns → MVP list view
3. US2 (Detail) → Test: detail page, tabs, copy ID → Detail browsing
4. US3 (Create) → Test: create + key reveal → Full create flow
5. US4 (Edit) → Test: in-place edit → Settings management
6. US5 (Block/Unblock) → Test: block/unblock with toast → Quick actions
7. US6 (Delete) → Test: alias confirmation delete → Safe deletion
8. US7 (Regenerate) → Test: regen + new key reveal → Key rotation
9. Polish → Edge cases, validation, Tailwind rebuild

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- All sqlc queries MUST use `sqlc.narg()` for nullable filter params — plain `$N::text IS NULL` generates `string` not `*string`
- Toast OOB swap uses `afterbegin:body` — toast component is self-positioning (`fixed z-50`)
- Routes follow existing split-route pattern (separate full-page vs HTMX partial routes)
- KeyRow uses value types (`string`, `bool`) — handler does nil→zero-value conversion at DB boundary
- Sort is fixed to `created_at DESC` in v1 (sqlc limitation) — client-side sort within page only
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
