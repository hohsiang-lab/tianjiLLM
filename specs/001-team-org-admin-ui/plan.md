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
