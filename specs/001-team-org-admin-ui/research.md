# Research: Team & Organization Admin UI

**Branch**: `001-team-org-admin-ui` | **Phase**: 0 | **Date**: 2026-02-26

## Summary

This feature adds server-rendered admin UI pages for Team and Organization management. The Go backend REST API handlers for teams/orgs already exist and have been verified for Python parity. This research resolves all NEEDS CLARIFICATION items before design.

---

## Decision 1: Python TianjiLLM Reference (Constitution Principle I)

**Decision**: Python TianjiLLM does **not** have a web UI — it is an API-only server. The team and org management UI is a Go-only addition. This feature is **exempt from Python behavioral parity requirements** for the UI layer.

**Rationale**: The Go REST API handlers (`/team/*`, `/organization/*`) already replicate Python API behavior. The UI layer calls those handlers' underlying DB methods directly (not via HTTP). UI design follows Go project conventions.

**Source**: Code exploration of existing `internal/proxy/handler/team.go`, `organization.go`, `server.go`.

**Documented Deviation**: UI feature has no Python equivalent. Constitution Principle I satisfied by confirming Go REST API handlers are parity-complete.

---

## Decision 2: UI Technology Stack

**Decision**: Reuse existing stack — templ + HTMX 2.x + templUI v1.5.0 + Tailwind CSS v4. No new libraries introduced.

**Rationale**: All four existing UI pages (keys, models, usage, logs) use this exact stack. Introducing any new library would create inconsistency and require template changes across all pages. The existing stack is fully capable of the required CRUD patterns.

**Evidence from codebase**:
- `internal/ui/pages/keys.templ` — uses templUI dialog, table, button, badge, input, icon
- `internal/ui/pages/layout.templ` — sidebar layout pattern
- `internal/ui/handler_keys.go` — HTMX partial swap pattern via `hx-get`/`hx-target`

**Alternatives rejected**:
- SPA (React/Vue): Would require separate build pipeline, JS bundle, API contracts — major complexity with zero benefit for admin UI.
- Alpine.js: Existing pages don't use it; adding it for one feature creates inconsistency.

---

## Decision 3: Database Queries — Existing vs New

**Finding**: Explored `internal/db/queries/` directory.

**Existing queries (reuse directly)**:
| File | Query | Purpose |
|------|-------|---------|
| `team.sql` | `ListTeams` | Teams list page (returns all fields) |
| `team.sql` | `GetTeam` | Team detail page |
| `team.sql` | `CreateTeam` | POST /ui/teams/create |
| `team.sql` | `UpdateTeam` | POST /ui/teams/{id}/update |
| `team.sql` | `DeleteTeam` | POST /ui/teams/{id}/delete |
| `team.sql` | `BlockTeam` / `UnblockTeam` | Toggle blocked status |
| `team.sql` | `AddTeamMember` / `RemoveTeamMember` | Member management |
| `team.sql` | `UpdateTeamMemberRole` | Role update |
| `team.sql` | `AddTeamModel` / `RemoveTeamModel` | Model management |
| `organization.sql` | `ListOrganizations` | Orgs list page |
| `organization.sql` | `GetOrganization` | Org detail page |
| `organization.sql` | `CreateOrganization` | POST /ui/orgs/create |
| `organization.sql` | `UpdateOrganization` | POST /ui/orgs/{id}/update |
| `organization.sql` | `DeleteOrganization` | POST /ui/orgs/{id}/delete |
| `org_membership.sql` | `ListOrgMembers` | Org detail — members table |
| `org_membership.sql` | `AddOrgMember` | POST /ui/orgs/{id}/members/add |
| `org_membership.sql` | `UpdateOrgMember` | POST /ui/orgs/{id}/members/update |
| `org_membership.sql` | `DeleteOrgMember` | POST /ui/orgs/{id}/members/remove |

**New queries needed** (add to existing .sql files):
| File | Query to Add | Reason |
|------|-------------|--------|
| `team.sql` | `ListTeamsByOrganization` | Org detail page needs teams belonging to that org |
| `organization.sql` | `CountTeamsPerOrganization` | Orgs list page needs team count per org (avoid N+1) |
| `organization.sql` | `CountMembersPerOrganization` | Orgs list page needs member count per org |

**Rationale for N+1 avoidance**: The org list page must show `team count` and `member count` per org (FR-010). Doing individual COUNT queries per org row is O(n) DB round-trips. Instead, add two aggregate queries that return all counts in one shot.

**Alternative considered**: `ListOrganizationsWithCounts` (single complex JOIN query). Rejected because sqlc handles simple named queries better, and two separate aggregate queries are clearer and easier to maintain.

---

## Decision 4: HTMX Interaction Pattern for Detail Pages

**Decision**: Detail pages use the same HTMX partial-swap pattern as list pages. No full page navigation for CRUD operations on detail pages.

**Pattern** (from `keys.templ`):
```
Full page: GET /ui/teams/{id}  → full HTML with @AppLayout
Partial:   POST /ui/teams/{id}/members/add → returns updated members table partial
           POST /ui/teams/{id}/block       → returns updated status badge partial
```

**Detail**: Each mutating action on the detail page targets a specific `<div id="...">` and the server returns only that partial. This keeps user context (they stay on the detail page).

---

## Decision 5: members_with_roles JSONB Parsing

**Decision**: Parse `members_with_roles` JSONB column in the UI handler into `[]TeamMemberRow{UserID, Role string}`. If parsing fails, fall back to displaying raw member IDs from the `members[]` array with empty roles.

**Schema** (from DB exploration):
```
TeamTable.members_with_roles: JSONB
Structure: [{"user_id": "u1", "role": "admin"}, ...]
```

**Implementation**: Use `encoding/json.Unmarshal` on the JSONB bytes returned by sqlc. sqlc maps JSONB columns to `[]byte` in Go.

**Edge case**: If `members_with_roles` is null but `members[]` has entries, show members without roles (graceful degradation). Spec assumption §5 confirms this structure.

---

## Decision 6: Organization List — Team Count and Member Count

**Decision**: Add two sqlc aggregate queries:
1. `CountTeamsPerOrganization` → `SELECT organization_id, COUNT(*) FROM "TeamTable" WHERE organization_id IS NOT NULL GROUP BY organization_id`
2. `CountMembersPerOrganization` → `SELECT organization_id, COUNT(*) FROM "OrganizationMembership" GROUP BY organization_id`

Then in `loadOrgsPageData`, merge counts into `OrgRow` structs using map lookups. O(1) per org after initial queries.

**Why not JOIN**: sqlc works best with named queries returning typed rows. A JOIN across three tables returns a mixed result that's harder to type safely.

---

## Decision 7: Pagination Strategy

**Decision**: Server-side pagination for both list pages. Use the same offset-based approach as the keys page: `LIMIT 50 OFFSET (page-1)*50`. No cursor-based pagination (not needed for admin UI at this scale).

**Evidence**: `handler_keys.go` uses `ListVerificationTokensFiltered` with limit/offset params. Same pattern applies.

**SC-008 compliance**: 200+ entries must be usable. With page size 50, 200 entries = 4 pages. Each page load fetches 50 rows max — well within 3s response time.

---

## Decision 8: Sidebar Navigation Addition

**Decision**: Add "Teams" and "Organizations" nav items to the sidebar. Insert between existing "Models" and "Usage" items.

**Location**: `internal/ui/pages/layout.templ` (sidebar nav section). Pattern from existing nav items:
```templ
@navItem("/ui/teams", "Teams", icon.Users(icon.Props{Size: 16}), activePath)
@navItem("/ui/orgs", "Organizations", icon.Building(icon.Props{Size: 16}), activePath)
```

**Icon availability**: templUI v1.5.0 includes Lucide icons. `Users` and `Building2` icons are available. Verify final icon names against `internal/ui/components/icon/` when implementing.

---

## Decision 9: Confirmation Dialog Pattern

**Decision**: Use templUI `dialog` component for destructive confirmations (delete team, delete org, remove member). Follow the same pattern as the "delete key" dialog in `keys.templ`.

**Pattern**:
```templ
@dialog.Dialog(dialog.Props{ID: "delete-team-dialog"}) {
    @dialog.Trigger(...) { Delete button }
    @dialog.Content(...) {
        @dialog.Header() { Confirm deletion }
        <p>Are you sure you want to delete team "{team.Alias}"?</p>
        <form hx-post="/ui/teams/{id}/delete" hx-target="#teams-table">
            <input type="hidden" name="team_id" value="{id}" />
            Submit button
        </form>
    }
}
```

---

## Decision 10: Error Feedback

**Decision**: Return HTTP 200 with inline error HTML for user-facing errors (duplicate member, non-existent user). Return HTTP 422 for server errors. HTMX renders the error message inline without a toast.

**Rationale**: The keys page uses a similar pattern — server returns the partial with an error banner. This avoids JavaScript event handling complexity.

**Edge cases from spec**:
- Duplicate member → inline error: "User {id} is already a member of this team"
- Non-existent org → inline error: "Organization {id} not found"
- Budget < current spend → allow with warning banner (spec §edge-cases)
- Delete org with teams → inline error: "Remove all teams from this organization first"

---

## Unresolved Items

None. All NEEDS CLARIFICATION items resolved.

---

## Source References

- Code exploration: `internal/db/queries/team.sql`, `organization.sql`, `org_membership.sql`
- Code exploration: `internal/ui/handler_keys.go`, `pages/keys.templ`, `routes.go`
- Code exploration: `internal/db/schema/002_management.up.sql`, `003_organization.up.sql`, `010_org_membership.up.sql`
- Spec: `specs/001-team-org-admin-ui/spec.md`
- Constitution: `.specify/memory/constitution.md` v1.2.0
