# Implementation Plan: Team & Organization Admin UI

**Branch**: `001-team-org-admin-ui` | **Date**: 2026-02-26 | **Spec**: `specs/001-team-org-admin-ui/spec.md`

## Summary

為 tianjiLLM 新增 Team 和 Organization 的管理 UI，遵循現有 handler_keys.go 的 pattern（Go handler + templ + HTMX）。DB queries 已就緒，只需新增 handler + views + routes。

## Technical Context

- **Language**: Go 1.23 + templ + HTMX
- **UI Framework**: Tailwind CSS + 現有 component library（sidebar, dropdown, toast, popover, modal）
- **Storage**: PostgreSQL（sqlc generated queries，已存在）
- **Testing**: Go test + Playwright E2E

## Implementation Phases

### Phase 1: Teams 列表頁（P1）
**路由**: `GET /ui/teams`
**檔案**:
- `internal/ui/handler_teams.go` — TeamsHandler struct + ListTeams, CreateTeam, UpdateTeam, DeleteTeam, BlockTeam, UnblockTeam
- `internal/ui/views/teams/list.templ` — 列表頁 template
- `internal/ui/views/teams/form.templ` — 新增/編輯 modal
- `internal/ui/views/teams/row.templ` — HTMX partial（單行更新）
- `internal/ui/routes.go` — 註冊 /ui/teams routes
- `internal/ui/views/components/sidebar/sidebar.templ` — 加 Teams nav item

**功能**:
- 列表顯示：team_alias, org, member count, budget, status
- 新增 team（alias, org_id, max_budget, models）
- 編輯 team（inline or modal）
- 刪除 team（確認 dialog）
- Block / Unblock toggle

### Phase 2: Team 詳情頁（P2）
**路由**: `GET /ui/teams/{team_id}`
**檔案**:
- `internal/ui/handler_teams.go` — GetTeam, AddMember, RemoveMember, AddModel, RemoveModel
- `internal/ui/views/teams/detail.templ` — 詳情頁
- `internal/ui/views/teams/members.templ` — Members tab
- `internal/ui/views/teams/models.templ` — Models tab
- `internal/ui/views/teams/spend.templ` — Spend 統計 tab

### Phase 3: Organizations 列表頁（P3）
**路由**: `GET /ui/orgs`
**檔案**:
- `internal/ui/handler_orgs.go` — OrgsHandler struct + CRUD
- `internal/ui/views/orgs/list.templ` — 列表頁
- `internal/ui/views/orgs/form.templ` — 新增/編輯 modal
- `internal/ui/views/orgs/row.templ` — HTMX partial
- `internal/ui/routes.go` — 註冊 /ui/orgs routes
- sidebar.templ — 加 Organizations nav item

### Phase 4: Organization 詳情頁（P4）
**路由**: `GET /ui/orgs/{org_id}`
**檔案**:
- `internal/ui/handler_orgs.go` — GetOrg, AddMember, RemoveMember, UpdateMemberRole
- `internal/ui/views/orgs/detail.templ` — 詳情頁
- `internal/ui/views/orgs/members.templ` — Membership 管理

## 驗收條件
1. `go vet ./...` + `go build ./...` PASS
2. 所有新頁面可透過 sidebar 導航
3. CRUD 操作即時反映（HTMX swap，無整頁刷新）
4. Block/Unblock 狀態切換正確
5. Member 新增/移除即時更新
6. Spend 統計數據與 DB 一致
