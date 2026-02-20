# Implementation Plan: UI Virtual Keys Management

**Branch**: `008-ui-virtual-keys` | **Date**: 2026-02-20 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/008-ui-virtual-keys/spec.md`

## Summary

在 TianjiLLM 管理面板中實作完整的 virtual keys 管理功能，對齊 LiteLLM Python UI 的核心行為。包含：擴充 `/key/list` API 支援伺服器端過濾/分頁/排序、新增 key 詳情頁（獨立 URL `/ui/keys/{id}`）、完善建立表單（返回一次性明文 key）、新增編輯/重新產生功能、改進刪除確認機制（輸入 alias 確認）、統一錯誤回饋（Toast OOB swap）。

技術方案：templ + HTMX + Tailwind（現有技術棧），sqlc 新增過濾查詢，chi router 新增詳情頁路由。

## Technical Context

**Language/Version**: Go 1.22+ (latest stable)
**Primary Dependencies**: chi v5 (HTTP router), templ (type-safe HTML templates), HTMX 2.x (server-driven UI), templUI v1.5.0 (shadcn-style components), Tailwind CSS v4
**Storage**: PostgreSQL (pgx/v5 + sqlc code generation)
**Testing**: `go test` + `testify` for assertions, `httptest` for handler tests
**Target Platform**: Linux server (admin web UI)
**Project Type**: web (Go server-rendered, no separate frontend)
**Performance Goals**: Key list page load < 2s for ≤1000 keys, filter/sort responses < 500ms
**Constraints**: sqlc-first DB access (Constitution VII), server-side rendering only (no client-side JS framework)
**Scale/Scope**: ≤10,000 virtual keys, single admin user (master key holder)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | PASS | Python LiteLLM `/key/list` endpoint fully analyzed — filter conditions, pagination, sorting, response format documented |
| II. Feature Parity | PASS | API contracts match Python `/key/list` parameters (page, size, team_id, key_alias, user_id, key_hash, sort_by, sort_order). UI feature set scoped to non-Premium features. |
| III. Research Before Build | PASS | Context7 docs for templ + HTMX queried, GitHub patterns searched (OOB toast, server-side sorting, copy-to-clipboard, dialog patterns) |
| IV. Test-Driven Migration | PASS | Contract tests planned for new `/key/list` filtering, handler tests for all UI endpoints |
| V. Go Best Practices | PASS | chi router, templ type-safe templates, sqlc queries, no hand-written SQL |
| VI. No Stale Knowledge | PASS | All patterns verified via Context7 + GitHub search |
| VII. sqlc-First DB Access | PASS | All new queries will be `.sql` files in `internal/db/queries/`, generated via `make generate` |

## Project Structure

### Documentation (this feature)

```text
specs/008-ui-virtual-keys/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── key-list-api.md  # Enhanced /key/list API contract
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── db/
│   ├── queries/
│   │   ├── verification_token.sql   # MODIFIED: add filtered list + count + regenerate-with-params queries
│   │   └── team.sql                 # MODIFIED: add ListTeamAliases query
│   └── *.sql.go                     # REGENERATED: sqlc generate
├── proxy/handler/
│   └── key.go                       # MODIFIED: enhance KeyList with filter/page/sort params
├── ui/
│   ├── routes.go                    # MODIFIED: add detail/edit/update/delete/regenerate routes
│   ├── handler_keys.go              # MODIFIED: major rewrite — detail page, create returns key,
│   │                                #           edit, regenerate, enhanced list, toast responses
│   └── pages/
│       ├── keys.templ               # MODIFIED: enhanced table, filters, pagination, create dialog
│       └── key_detail.templ         # NEW: detail page (overview + settings tabs, edit form,
│                                    #      delete confirm, regenerate dialog)
└── test/contract/
    └── handler_key_list_test.go     # NEW: filter/page/sort contract tests
```

**Structure Decision**: Single Go web application. No frontend project needed — templ + HTMX provides server-rendered UI with interactive behavior. All changes within existing `internal/` directory structure. New templ file `key_detail.templ` for detail page; rest is modifications to existing files.

## Complexity Tracking

No constitution violations. All changes use existing patterns (sqlc queries, templ components, chi routes, HTMX partials).

### Plan Review 修正記錄 (2026-02-20)

以下問題在 plan review 中被發現並已修正：

| 嚴重度 | 問題 | 修正 |
|--------|------|------|
| P0 | sqlc `$1::text IS NULL` 生成 `string` 非 `*string`，過濾器無法工作 | 改用 `sqlc.narg()` 宏生成 nullable 參數 |
| P0 | `RegenerateVerificationToken` 只接受 2 參數，無法同時修改屬性 | 新增 `RegenerateVerificationTokenWithParams` 查詢 |
| P1 | Toast OOB swap 到 `#toast-area` 容器，但 toast 組件自帶 `fixed` 定位 | 改用 `afterbegin:body` OOB swap，不需容器 |
| P1 | Toast 容器 ID 錯誤（`toast-area` vs 實際 `toast-container`） | 不再使用容器，直接追加到 body |
| P1 | 缺少 team_alias JOIN/查詢 | 新增 `ListTeamAliases` 分離查詢 |
| P1 | 提議 HX-Request 檢測但現有用分離路由 | 改為沿用分離路由模式 |
| P2 | KeyRow struct 類型與現有不符（*string vs string） | 修正為保持現有值類型模式 |
| P2 | KeysPageData 遺漏 `Search` 字段 | 保留向後兼容 |
| P2 | keysPerPage 是 20 不是 50 | 記錄需改為 50 |
| P2 | Create 表單字段名是 `key_name` 不是 `key_alias` | 擴充表單時同時改名 |
