# Implementation Plan: Team & Organization Admin UI

**Branch**: `001-team-org-admin-ui` | **Date**: 2026-02-26 | **Spec**: `specs/001-team-org-admin-ui/spec.md`

## Summary

Add admin UI pages for Team and Organization management to the TianjiLLM Go proxy. Four pages: Teams list (`/ui/teams`), Team detail (`/ui/teams/{id}`), Organizations list (`/ui/orgs`), and Org detail (`/ui/orgs/{id}`). All pages are server-rendered using the existing templ + HTMX + templUI stack — zero new libraries. All DB queries go through the existing sqlc pipeline; three new queries needed (`ListTeamsByOrganization`, `CountTeamsPerOrganization`, `CountMembersPerOrganization`). No schema migrations required.

## Technical Context

**Language/Version**: Go 1.24.4
**Primary Dependencies**: chi/v5 (router), templ (templates), HTMX 2.x (partials), templUI v1.5.0 (components), Tailwind CSS v4
**Storage**: PostgreSQL via pgx/v5 + sqlc codegen. Tables: `TeamTable`, `OrganizationTable`, `OrganizationMembership`. No new migrations.
**Testing**: `go test ./internal/ui/...` (unit) + Playwright E2E (`make e2e`)
**Target Platform**: Linux server (admin web UI, accessible at `/ui/teams` and `/ui/orgs`)
**Project Type**: Single Go project — web application server
**Performance Goals**: List pages load < 3s with 200+ entries (SC-008). Mutations visible < 3s (SC-001, SC-003).
**Constraints**: No full page reloads for CRUD operations. All destructive actions require confirmation (SC-006).
**Scale/Scope**: 200+ teams, 200+ orgs per deployment (SC-008 requirement).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | ✅ PASS | Python TianjiLLM has no web UI. Go REST API handlers (`/team/*`, `/organization/*`) are already Python-parity-complete. UI layer is Go-only. |
| II. Feature Parity | ✅ PASS | All CRUD operations match the existing Go REST API contracts. No deviations from existing data structures. |
| III. Research Before Build | ✅ PASS | Research complete — see `research.md`. Technology choices verified against existing codebase patterns. |
| IV. Test-Driven Migration | ✅ PASS | Unit tests for handler helpers; E2E Playwright tests for acceptance scenarios from spec. |
| V. Go Best Practices | ✅ PASS | Handler functions per file pattern, `context.Context` propagation, `fmt.Errorf("%w")` error wrapping, no global state. |
| VI. No Stale Knowledge | ✅ PASS | All patterns verified from existing codebase (`handler_keys.go`, `pages/keys.templ`, `routes.go`). No reliance on cached knowledge. |
| VII. sqlc-First DB Access | ✅ PASS | Three new queries added to `.sql` files; `make generate` required before implementation. No hand-written SQL. |

**Post-design re-check**: ✅ All gates pass. No violations. No Complexity Tracking needed.

## Project Structure

### Documentation (this feature)

```text
specs/001-team-org-admin-ui/
├── plan.md              # This file
├── research.md          # Phase 0 output — decisions, unknowns resolved
├── data-model.md        # Phase 1 output — DB entities, Go view structs, handler data flow
├── quickstart.md        # Phase 1 output — dev setup, build commands, pitfalls
├── contracts/
│   └── ui-routes.md     # Phase 1 output — all UI route contracts with form fields
└── tasks.md             # Phase 2 output (run /speckit.tasks)
```

### Source Code (affected files)

```text
internal/db/queries/
├── team.sql                     # + ListTeamsByOrganization query
└── organization.sql             # + CountTeamsPerOrganization, CountMembersPerOrganization

internal/ui/
├── routes.go                    # Modified: add teams + orgs route group
├── handler_teams.go             # New: list, create, block/unblock handlers
├── handler_teams_detail.go      # New: detail, update, delete, member/model management
├── handler_orgs.go              # New: list, create handlers
├── handler_orgs_detail.go       # New: detail, update, delete, member management
└── pages/
    ├── layout.templ             # Modified: add Teams + Organizations sidebar nav items
    ├── teams.templ              # New: TeamsPage, TeamsTablePartial, TeamDetailPage, partials
    └── orgs.templ               # New: OrgsPage, OrgsTablePartial, OrgDetailPage, partials

test/e2e/
└── teams_orgs_test.go           # New: Playwright E2E tests for spec acceptance scenarios
```

**Structure Decision**: Single project (existing Go monorepo). UI code follows the established `internal/ui/` layout. New templ files parallel `pages/keys.templ`. New handler files parallel `handler_keys.go`. No new packages created.

## Complexity Tracking

No constitution violations. No additional complexity justification needed.

## Testing Strategy

### 1. Unit Test 範圍與 Mock 策略

**原則**：純邏輯用 unit test，handler 與 DB 的交互靠 E2E。
**原因**：`UIHandler` 直接持有 `*db.Queries` concrete struct（非 interface），mock 成本高且脆，改由 E2E 覆蓋 DB 路徑。

Unit test 目標（放在 `internal/ui/handler_teams_test.go` / `handler_orgs_test.go`）：

| 測試對象 | 測試案例 |
|---------|---------|
| `parseMaxBudget()` | 空字串 → unlimited（`nil`）；合法浮點數 → 正確值；非數字字串 → error |
| `parseMembersWithRoles()` | 正常 JSON array → 正確解析；`null` / 空字串 → 空 slice 不 panic；malformed JSON → 返回 error + fallback 空 slice |
| `parsePage()` | `page=0` → 第 1 頁；`page=-1` → 第 1 頁；`page=999999` → 正常（交由 DB offset 處理）；非數字 → 第 1 頁 |
| 搜尋 filter 清理 | 空字串 → `""` 不注入；含 `%` / `_` / `'` 的特殊字元 → 正確 escape 或 sanitize |
| Org 刪除前置檢查 | `teamCount > 0` → 回傳拒絕錯誤；`teamCount == 0` → 允許刪除 |
| 重複成員檢查 | 同一 `user_id` 在成員列表出現 → 回傳重複錯誤 |

### 2. E2E Test Case 完整清單

路徑：`test/e2e/`，每個功能領域拆獨立檔案，對應 spec 的 4 個 User Story + 16 個 Acceptance Scenario + 6 個 Edge Case。

#### Teams（6 個檔案）

| 檔案 | 覆蓋 Scenario |
|-----|-------------|
| `teams_list_test.go` | 列表欄位完整性（name、alias、member count、budget、status）；搜尋 filter 可 narrow 結果；分頁（seed 50+ 記錄，驗證 page 2 可到達） |
| `teams_create_test.go` | Happy path：填表 → submit → 出現在列表；alias 重複 → 顯示 error message；必填欄位空白 → 表單阻擋 |
| `teams_detail_test.go` | 所有欄位正確顯示；編輯 alias 和 budget → 儲存後 DOM 反映新值；Budget < current spend → 顯示 warn toast 但仍允許儲存 |
| `teams_members_test.go` | 新增成員 → member row 出現；新增不存在 user_id → error；重複新增已有成員 → error；移除成員 → row 消失 |
| `teams_models_test.go` | 新增 model → 列表出現；移除 model → 列表消失；models 清空 → 顯示「Inherited / All models」label |
| `teams_block_test.go` | Block → badge 從 Active 改為 Blocked；Unblock → badge 恢復 Active；Block 後其他 UI 狀態一致 |
| `teams_delete_test.go` | 點擊刪除 → confirmation dialog 出現；取消 → 不刪除；確認 → 從列表消失（`waitForSelector` 驗證 row 不存在） |

#### Organizations（5 個檔案）

| 檔案 | 覆蓋 Scenario |
|-----|-------------|
| `orgs_list_test.go` | 列表顯示 team count 和 member count（由 `CountTeamsPerOrganization` / `CountMembersPerOrganization` 提供）；欄位完整 |
| `orgs_create_test.go` | Happy path：建立 org → 出現在列表；name 重複 → error（若 DB 有 unique constraint） |
| `orgs_detail_test.go` | 所有欄位正確顯示；編輯 org name / description → DOM 更新 |
| `orgs_members_test.go` | 新增成員並指定 role → 出現在成員表；修改 role → 更新顯示；移除成員 → row 消失；重複新增 → error |
| `orgs_delete_test.go` | 無 teams → 刪除成功，從列表消失；有 teams → error message 顯示，org 仍在列表 |

### 3. Edge Case 對應表

| Edge Case | 對應 test file | 驗證方式 |
|----------|--------------|---------|
| 刪除有 teams 的 org | `orgs_delete_test.go` | assert error toast 出現；org row 仍存在列表 |
| 不存在的 user_id 新增為 Team member | `teams_members_test.go` | assert error message；member count 不變 |
| 不存在的 user_id 新增為 Org member | `orgs_members_test.go` | assert error message；member count 不變 |
| Budget < current spend | `teams_detail_test.go` | assert warn toast 出現；儲存仍成功（budget 更新） |
| Blocked team 的 badge 狀態與 toggle | `teams_block_test.go` | assert badge text / CSS class 改變 |
| Empty models → "Inherited / All models" | `teams_models_test.go` | assert 特定 label element 出現 |
| 重複成員（Team） | `teams_members_test.go` | assert error toast；member count 不變 |
| 重複成員（Org） | `orgs_members_test.go` | assert error toast；member count 不變 |

### 4. sqlc Query Contract Tests

新增 `test/contract/teams_orgs_test.go`，使用真實 PostgreSQL（同 E2E 的 `postgres://tianji:tianji@localhost:5433/tianji_e2e`）：

| Query | 測試案例 |
|-------|---------|
| `ListTeamsByOrganization` | 正常回傳：seed org + teams → 查詢結果符合；空結果：無 teams 的 org → 回傳空 slice 不 panic |
| `CountTeamsPerOrganization` | 有 teams 的 org → count > 0；無 teams 的 org → count == 0 |
| `CountMembersPerOrganization` | 有 members 的 org → count > 0；無 members 的 org → count == 0 |

Build tag：`//go:build integration`，執行方式：`go test -tags=integration ./test/contract/...`

### 5. 效能測試

在 `teams_list_test.go` 補充效能驗證 case（Playwright）：

```
Seed 200+ teams → 導覽至 /ui/teams
→ waitForSelector("[data-testid=teams-table]", { timeout: 3000 })
→ assert 頁面載入 < 3 秒（符合 SC-008 / Performance Goals）
```

同樣在 `orgs_list_test.go` 對 200+ orgs 做相同驗證。

### 6. HTMX Partial 回歸原則

每個 mutation（Create / Update / Delete / Block / Add member / Remove member）的 E2E test **不能只 assert HTTP 200**，必須驗證 DOM 實際變化：

| Mutation | DOM 驗證要點 |
|---------|------------|
| 建立 Team / Org | 新 row 出現在 table（`waitForSelector` by name/alias） |
| 更新 alias / budget | 對應 cell 文字改變（`assertContains` new value） |
| 刪除 Team / Org | 對應 row 從 DOM 消失（`waitForSelector` with `state: "detached"`） |
| Block / Unblock | badge element 的 text 或 class 改變 |
| 新增成員 | members table 新增 row，顯示正確 user + role |
| 移除成員 | members table 對應 row 消失 |
| 新增 / 移除 model | models list 對應 item 出現或消失 |
| Error case | error toast / inline error element 出現（`waitForSelector` by data-testid） |
