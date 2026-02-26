# Tasks: Team & Organization Admin UI

**Input**: Design documents from `/specs/001-team-org-admin-ui/`
**Prerequisites**: plan.md ✅, spec.md ✅, data-model.md ✅, contracts/ui-routes.md ✅, quickstart.md ✅

**Tests**: Included — plan.md testing strategy explicitly defines unit tests, E2E tests (Playwright), and sqlc contract tests.

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US4)

---

## Phase 1: Setup (sqlc Queries + Code Generation)

**Purpose**: Add the three new sqlc queries and regenerate Go types. No other work can reference the new DB methods until this phase is complete.

- [x] T001 [P] Add `ListTeamsByOrganization` query to `internal/db/queries/team.sql`
- [x] T002 [P] Add `CountTeamsPerOrganization` and `CountMembersPerOrganization` queries to `internal/db/queries/organization.sql`
- [x] T003 Run `make generate` to regenerate sqlc Go types in `internal/db/` (depends T001, T002)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared UI infrastructure that all user story pages depend on. Complete before any handler work.

**⚠️ CRITICAL**: No user story implementation can compile until Phase 1 is complete; sidebar nav must exist before UI pages are testable.

- [x] T004 [P] Add Teams (`users` icon) and Organizations (`building-2` icon) sidebar nav items to `internal/ui/pages/layout.templ`, then run `templ generate` to regenerate `layout_templ.go`
- [x] T005 [P] Create sqlc contract tests (build tag: `integration`) for `ListTeamsByOrganization`, `CountTeamsPerOrganization`, `CountMembersPerOrganization` in `test/contract/teams_orgs_test.go` (depends T003)

**Checkpoint**: Foundation ready — sidebar shows Teams + Organizations links; new sqlc methods verified against real PostgreSQL.

---

## Phase 3: User Story 1 — Browse and Manage Teams (Priority: P1) 🎯 MVP

**Goal**: A fully functional `/ui/teams` list page where admins can view all teams, create new ones, block/unblock, and delete — all without full page reloads.

**Independent Test**: Navigate to `/ui/teams`, verify paginated table shows team alias/status/counts. Create a new team — confirm it appears at top without reload. Block a team — confirm badge switches to "Blocked".

### Tests for User Story 1

> **Write these first — they MUST compile with stub functions and initially FAIL**

- [x] T006 [P] [US1] Write unit tests for `parsePage` (boundary cases), `parseMaxBudget` (empty/float/invalid), `parseMembersWithRoles` (valid JSON / null / malformed), and search filter sanitization (`%`, `_`, `'`) in `internal/ui/handler_teams_test.go`
- [ ] T007 [P] [US1] Write Playwright E2E test skeletons for teams list fields, search/filter narrowing, and 200+ entries performance assertion (`< 3s`) in `test/e2e/teams_list_test.go`
- [ ] T008 [P] [US1] Write Playwright E2E tests for team create happy path (alias → submit → row appears) and validation errors (alias missing, duplicate) in `test/e2e/teams_create_test.go`
- [ ] T009 [P] [US1] Write Playwright E2E tests for block/unblock badge state toggle in `test/e2e/teams_block_test.go`
- [ ] T010 [P] [US1] Write Playwright E2E tests for delete: confirmation dialog appears; cancel keeps row; confirm removes row (verify via `waitForSelector state:"detached"`) in `test/e2e/teams_delete_test.go`

### Implementation for User Story 1

- [x] T011 [P] [US1] Create `TeamsPage`, `TeamsTablePartial`, `TeamStatusBadge`, and "New Team" create-dialog form templates in `internal/ui/pages/teams.templ`, then run `templ generate`
- [x] T012 [US1] Implement `loadTeamsPageData` helper and `handleTeams` + `handleTeamsTable` handlers in `internal/ui/handler_teams.go` (depends T011; loads `ListTeams`, filters in Go, paginates, builds `TeamsPageData`)
- [x] T013 [US1] Implement `handleTeamCreate` handler in `internal/ui/handler_teams.go` (depends T012; calls `CreateTeam`, returns `TeamsTablePartial` or error banner; generates UUID for `team_id`)
- [x] T014 [US1] Implement `handleTeamBlock` and `handleTeamUnblock` handlers in `internal/ui/handler_teams.go` (depends T012; calls `BlockTeam`/`UnblockTeam`, returns `TeamStatusBadge` partial targeting `#team-status-{id}`)
- [x] T015 [US1] Implement `handleTeamDelete` handler in `internal/ui/handler_teams.go` (depends T012; calls `DeleteTeam`, returns `HX-Redirect: /ui/teams`)
- [x] T016 [US1] Register teams list routes in `internal/ui/routes.go`: `GET /ui/teams`, `GET /ui/teams/table`, `POST /ui/teams/create`, `POST /ui/teams/{team_id}/block`, `POST /ui/teams/{team_id}/unblock`, `POST /ui/teams/{team_id}/delete` (depends T013–T015; build must pass after this task)

**Checkpoint**: Run `make build` → server compiles. Navigate to `/ui/teams`. All US1 acceptance scenarios pass.

---

## Phase 4: User Story 2 — View Team Details and Manage Members/Models (Priority: P2)

**Goal**: A `/ui/teams/{team_id}` detail page showing complete team configuration with inline member and model management — no page reloads for mutations.

**Independent Test**: Click any team from the list → land on detail page. Verify member list, model list, spend/budget, TPM/RPM limits shown. Add a member → row appears. Remove a member → row disappears. Add/remove a model → list updates.

### Tests for User Story 2

- [ ] T017 [P] [US2] Write Playwright E2E tests for team detail page fields (alias, org link, admins, members, models, spend/budget, TPM/RPM, metadata JSON, blocked badge) in `test/e2e/teams_detail_test.go`
- [ ] T018 [P] [US2] Write Playwright E2E tests for member management: add (user appears), add duplicate (error), add non-existent user_id (error), remove (row disappears) in `test/e2e/teams_members_test.go`
- [ ] T019 [P] [US2] Write Playwright E2E tests for model management: add model (appears in list), remove model (disappears), empty models shows "Inherited / All models" label in `test/e2e/teams_models_test.go`

### Implementation for User Story 2

- [ ] T020 [US2] Add `TeamDetailPage`, `TeamDetailHeader`, `TeamMembersTablePartial`, `TeamModelsListPartial`, and team edit modal dialog templates to `internal/ui/pages/teams.templ`, then run `templ generate` (depends T011; extends the same file)
- [ ] T021 [US2] Implement `loadTeamDetailData` helper and `handleTeamDetail` handler in `internal/ui/handler_teams_detail.go` (depends T020; calls `GetTeam`, `json.Unmarshal` members_with_roles JSONB → `[]TeamMemberRow`, looks up OrgAlias)
- [ ] T022 [US2] Implement `handleTeamUpdate` handler in `internal/ui/handler_teams_detail.go` (depends T021; validates alias non-empty, updates via `UpdateTeam`, warns if budget < current spend, returns `TeamDetailHeader` partial targeting `#team-detail-header`)
- [ ] T023 [US2] Implement `handleTeamMemberAdd` and `handleTeamMemberRemove` handlers in `internal/ui/handler_teams_detail.go` (depends T021; add: checks duplicate in `members[]`, calls `AddTeamMember` + `UpdateTeamMemberRole`; remove: `RemoveTeamMember` + updates JSONB; both return `TeamMembersTablePartial`)
- [ ] T024 [US2] Implement `handleTeamModelAdd` and `handleTeamModelRemove` handlers in `internal/ui/handler_teams_detail.go` (depends T021; calls `AddTeamModel`/`RemoveTeamModel`; returns `TeamModelsListPartial` with "Inherited / All models" when empty)
- [ ] T025 [US2] Register team detail routes in `internal/ui/routes.go`: `GET /ui/teams/{team_id}`, `POST /ui/teams/{team_id}/update`, `POST /ui/teams/{team_id}/members/add`, `POST /ui/teams/{team_id}/members/remove`, `POST /ui/teams/{team_id}/models/add`, `POST /ui/teams/{team_id}/models/remove` (depends T022–T024; build must pass)

**Checkpoint**: Run `make build`. Navigate to `/ui/teams/{team_id}`. US1 and US2 acceptance scenarios pass independently.

---

## Phase 5: User Story 3 — Browse and Manage Organizations (Priority: P3)

**Goal**: A functional `/ui/orgs` list page where admins can see all organizations with team/member counts, create new ones, and manage them without page reloads.

**Independent Test**: Navigate to `/ui/orgs`, verify table shows org alias, team count, member count, spend, budget. Create a new org → appears in list. Search filter narrows results. 200+ orgs load in < 3s.

### Tests for User Story 3

- [ ] T026 [P] [US3] Write Playwright E2E tests for orgs list fields (alias, team count from `CountTeamsPerOrganization`, member count from `CountMembersPerOrganization`, spend, budget) and 200+ entries performance assertion in `test/e2e/orgs_list_test.go`
- [ ] T027 [P] [US3] Write Playwright E2E tests for org create happy path (alias → submit → appears in list) and validation errors (missing alias, duplicate name if DB-constrained) in `test/e2e/orgs_create_test.go`

### Implementation for User Story 3

- [ ] T028 [P] [US3] Create `OrgsPage`, `OrgsTablePartial`, and "New Organization" create-dialog form templates in `internal/ui/pages/orgs.templ`, then run `templ generate`
- [ ] T029 [US3] Implement `loadOrgsPageData` helper and `handleOrgs` + `handleOrgsTable` handlers in `internal/ui/handler_orgs.go` (depends T028; calls `ListOrganizations`, `CountTeamsPerOrganization`, `CountMembersPerOrganization`; merges into `[]OrgRow` map; paginates)
- [ ] T030 [US3] Implement `handleOrgCreate` handler in `internal/ui/handler_orgs.go` (depends T029; validates org_alias non-empty, calls `CreateOrganization`, returns `OrgsTablePartial` or error banner)
- [ ] T031 [US3] Register orgs list routes in `internal/ui/routes.go`: `GET /ui/orgs`, `GET /ui/orgs/table`, `POST /ui/orgs/create` (depends T029–T030; build must pass)

**Checkpoint**: Run `make build`. Navigate to `/ui/orgs`. US3 acceptance scenarios pass independently.

---

## Phase 6: User Story 4 — View Organization Details and Manage Membership (Priority: P4)

**Goal**: A `/ui/orgs/{org_id}` detail page showing org configuration with full membership management (add, change role, remove), teams summary, and safe delete with server-side team-count check.

**Independent Test**: Open `/ui/orgs/{org_id}`. Verify member list with roles, spend, join dates. Add member with role → appears. Change role via dropdown → updates. Remove member → disappears. Delete org with teams → inline error. Delete org without teams → redirects to list.

### Tests for User Story 4

- [ ] T032 [P] [US4] Write Playwright E2E tests for org detail fields (alias, budget, members table with user_id/role/spend/join date, teams summary, metadata JSON) in `test/e2e/orgs_detail_test.go`
- [ ] T033 [P] [US4] Write Playwright E2E tests for org member management: add with role (row appears), add duplicate (error), add non-existent user_id (error), change role via `<select>` (updates displayed), remove (row disappears) in `test/e2e/orgs_members_test.go`
- [ ] T034 [P] [US4] Write Playwright E2E tests for org delete: org with teams → error toast shown, org row remains in list; org without teams → redirect to `/ui/orgs`, org row gone in `test/e2e/orgs_delete_test.go`

### Implementation for User Story 4

- [ ] T035 [US4] Add `OrgDetailPage`, `OrgDetailHeaderPartial`, `OrgMembersTablePartial`, org edit modal dialog, and inline role `<select>` (`hx-trigger="change"`) templates to `internal/ui/pages/orgs.templ`, then run `templ generate` (depends T028; extends the same file)
- [ ] T036 [US4] Implement `loadOrgDetailData` helper and `handleOrgDetail` handler in `internal/ui/handler_orgs_detail.go` (depends T035; calls `GetOrganization`, `ListOrgMembers`, `ListTeamsByOrganization` for teams summary panel)
- [ ] T037 [US4] Implement `handleOrgUpdate` handler in `internal/ui/handler_orgs_detail.go` (depends T036; validates org_alias, calls `UpdateOrganization`, returns `OrgDetailHeaderPartial` targeting `#org-detail-header`)
- [ ] T038 [US4] Implement `handleOrgDelete` handler in `internal/ui/handler_orgs_detail.go` (depends T036; calls `ListTeamsByOrganization` to check for dependent teams — if `len > 0` return error banner; else `DeleteOrganization` + `HX-Redirect: /ui/orgs`)
- [ ] T039 [US4] Implement `handleOrgMemberAdd`, `handleOrgMemberUpdate`, `handleOrgMemberRemove` handlers in `internal/ui/handler_orgs_detail.go` (depends T036; add: validate user_role in allowed set, check duplicate via `AddOrgMember`; update: `UpdateOrgMember`; remove: `DeleteOrgMember`; all return `OrgMembersTablePartial`)
- [ ] T040 [US4] Register org detail routes in `internal/ui/routes.go`: `GET /ui/orgs/{org_id}`, `POST /ui/orgs/{org_id}/update`, `POST /ui/orgs/{org_id}/delete`, `POST /ui/orgs/{org_id}/members/add`, `POST /ui/orgs/{org_id}/members/update`, `POST /ui/orgs/{org_id}/members/remove` (depends T037–T039; build must pass)

**Checkpoint**: Run `make build`. Navigate to `/ui/orgs/{org_id}`. All four US acceptance scenarios pass independently.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final validation, edge case unit tests, and full test suite run.

- [ ] T041 [P] Write unit tests for `orgDeletePreCheck` (teamCount > 0 → reject, == 0 → allow), duplicate member check for orgs, and user_role validation in `internal/ui/handler_orgs_test.go`
- [ ] T042 Run `make check` (golangci-lint + go test -race -cover ./internal/ui/... + build) and fix any lint or test failures
- [ ] T043 Run `make e2e` against containerized PostgreSQL (`postgres://tianji:tianji@localhost:5433/tianji_e2e`) and verify all teams + orgs E2E tests pass, including 200+ entries performance assertions (< 3s load per SC-008)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — T001 and T002 run in parallel, then T003
- **Phase 2 (Foundational)**: Depends on Phase 1 — T004 and T005 can run in parallel
- **Phase 3 (US1)**: Depends on Phase 2 completion — tests [T006–T010] can run in parallel; implementation is sequential
- **Phase 4 (US2)**: Depends on Phase 3 completion (T011 layout in teams.templ must exist) — tests [T017–T019] can run in parallel; T020 extends T011
- **Phase 5 (US3)**: Depends on Phase 2 completion — independent of US1/US2; tests [T026–T027] can run in parallel
- **Phase 6 (US4)**: Depends on Phase 5 completion (T028 orgs.templ must exist) — tests [T032–T034] can run in parallel; T035 extends T028
- **Phase 7 (Polish)**: Depends on all US phases complete

### User Story Dependencies

- **US1 (P1)**: Can start after Foundational — no dependency on US2/US3/US4
- **US2 (P2)**: Can start after Foundational — shares `teams.templ` with US1 (sequential, not truly parallel)
- **US3 (P3)**: Can start after Foundational — completely independent of US1/US2
- **US4 (P4)**: Can start after US3 (shares `orgs.templ`) — independent of US1/US2

### Parallel Opportunities Per Phase

```bash
# Phase 1: Run in parallel
Task: "Add ListTeamsByOrganization to internal/db/queries/team.sql"       # T001
Task: "Add CountTeams/CountMembers queries to internal/db/queries/organization.sql"  # T002

# Phase 2: Run in parallel
Task: "Add sidebar nav items to internal/ui/pages/layout.templ"           # T004
Task: "Create sqlc contract tests in test/contract/teams_orgs_test.go"    # T005

# Phase 3 US1 tests: Run in parallel
Task: "Unit tests in internal/ui/handler_teams_test.go"                   # T006
Task: "E2E skeleton in test/e2e/teams_list_test.go"                       # T007
Task: "E2E tests in test/e2e/teams_create_test.go"                        # T008
Task: "E2E tests in test/e2e/teams_block_test.go"                         # T009
Task: "E2E tests in test/e2e/teams_delete_test.go"                        # T010

# Phase 3 US1 templ: Run in parallel with tests
Task: "Create TeamsPage/TeamsTablePartial/TeamStatusBadge in teams.templ" # T011

# Phase 4 US2 tests: Run in parallel
Task: "E2E tests in test/e2e/teams_detail_test.go"                        # T017
Task: "E2E tests in test/e2e/teams_members_test.go"                       # T018
Task: "E2E tests in test/e2e/teams_models_test.go"                        # T019

# Phase 5 US3: Run in parallel
Task: "E2E tests in test/e2e/orgs_list_test.go"                           # T026
Task: "E2E tests in test/e2e/orgs_create_test.go"                         # T027
Task: "Create OrgsPage/OrgsTablePartial in orgs.templ"                    # T028
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (`make generate`)
2. Complete Phase 2: Foundational (sidebar nav + contract tests)
3. Complete Phase 3: User Story 1 (`/ui/teams` list page)
4. **STOP and VALIDATE**: `make build` → navigate `/ui/teams` → verify all US1 scenarios
5. Demo to stakeholders

### Incremental Delivery

1. Setup + Foundational → DB types ready, sidebar updated
2. Add US1 → `/ui/teams` fully functional → Demo (MVP!)
3. Add US2 → `/ui/teams/{id}` detail functional → Demo
4. Add US3 → `/ui/orgs` list functional → Demo
5. Add US4 → `/ui/orgs/{id}` detail functional → Full feature complete
6. Polish → All tests green, `make check` passes

### Parallel Team Strategy (2 developers)

After Phase 2 (Foundational) completes:

- **Developer A**: Phase 3 (US1) → Phase 4 (US2) — all in `handler_teams*.go` + `pages/teams.templ`
- **Developer B**: Phase 5 (US3) → Phase 6 (US4) — all in `handler_orgs*.go` + `pages/orgs.templ`

No file conflicts: teams and orgs files are completely separate. Merge point: `internal/ui/routes.go` (each developer appends their route block sequentially).

---

## Notes

- **`make generate` required** after every edit to `internal/db/queries/*.sql` — sqlc Go types won't match otherwise
- **`templ generate` required** after every edit to `*.templ` — stale generated Go code will cause build failure
- **`members_with_roles` JSONB** comes from sqlc as `[]byte` — always `json.Unmarshal` before use
- **chi URL params**: use `chi.URLParam(r, "team_id")` not `r.URL.Query().Get("team_id")`
- **routes.go registration**: register routes only after all referenced handler methods exist (Go package must compile)
- **[P] tasks** = different files or independent operations, no data dependencies
- Each phase checkpoint = `make build` + manual smoke test; Phase 7 = full `make check` + `make e2e`
