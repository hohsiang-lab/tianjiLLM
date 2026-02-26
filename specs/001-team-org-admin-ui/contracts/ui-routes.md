# UI Route Contracts: Team & Organization Admin UI

**Branch**: `001-team-org-admin-ui` | **Phase**: 1 | **Date**: 2026-02-26

All routes are registered under the `/ui` prefix via `UIHandler.RegisterRoutes`. All routes require session authentication (via `sessionAuth` middleware). Requests with `HX-Request: true` header get HTMX-specific responses (no full page reload, inline partials).

---

## Teams Routes

### GET /ui/teams

**Purpose**: Teams list page — full page load
**Handler**: `UIHandler.handleTeams`
**Auth**: Required (sessionAuth)
**Query Params**:
- `page` (int, default 1) — pagination
- `search` (string) — filter by team alias (case-insensitive)
- `filter_org_id` (string) — filter by organization ID

**Response**:
- `200 OK` — full HTML with `@AppLayout` + `@TeamsPage(data)`

**HTMX**: Initial page load only; filters trigger `GET /ui/teams/table`

---

### GET /ui/teams/table

**Purpose**: Teams table partial — HTMX swap target
**Handler**: `UIHandler.handleTeamsTable`
**Auth**: Required
**Query Params**: same as GET /ui/teams
**Response**:
- `200 OK` — partial HTML: `@TeamsTablePartial(data)` (only the `<div id="teams-table">` content)

**HTMX triggers**:
- Filter input `hx-trigger="input changed delay:300ms"` → replaces `#teams-table`
- After create/delete/block mutations → server triggers `teams-changed` event

---

### POST /ui/teams/create

**Purpose**: Create a new team
**Handler**: `UIHandler.handleTeamCreate`
**Auth**: Required
**Content-Type**: `application/x-www-form-urlencoded`

**Form Fields**:
| Field | Type | Required | Notes |
|-------|------|----------|-------|
| `team_alias` | string | Yes | Non-empty |
| `organization_id` | string | No | Must exist if provided |
| `max_budget` | float | No | Blank = unlimited |
| `tpm_limit` | int | No | Blank = unlimited |
| `rpm_limit` | int | No | Blank = unlimited |
| `budget_duration` | string | No | "daily" / "weekly" / "monthly" |
| `models` | string[] | No | Multiple values; empty = all models |

**Response** (HTMX):
- `200 OK` — `@TeamsTablePartial(data)` (refreshed table showing new team at top)
- `200 OK` with error banner — `@TeamsTablePartial(data)` with `data.Error` set (alias exists, org not found, etc.)

**DB calls**:
1. `h.DB.CreateTeam(ctx, db.CreateTeamParams{...})` — generates UUID for team_id
2. Reload table: `h.DB.ListTeams(ctx)`

---

### GET /ui/teams/{team_id}

**Purpose**: Team detail page — full page
**Handler**: `UIHandler.handleTeamDetail`
**Auth**: Required
**Path Params**: `team_id` (string, UUID)

**Response**:
- `200 OK` — full HTML with `@AppLayout` + `@TeamDetailPage(data)`
- `404` — redirect to `/ui/teams` if team not found

---

### POST /ui/teams/{team_id}/update

**Purpose**: Update team alias, budget, TPM/RPM limits
**Handler**: `UIHandler.handleTeamUpdate`
**Auth**: Required
**Content-Type**: `application/x-www-form-urlencoded`

**Form Fields**:
| Field | Type | Required |
|-------|------|----------|
| `team_alias` | string | Yes |
| `max_budget` | float | No |
| `tpm_limit` | int | No |
| `rpm_limit` | int | No |
| `budget_duration` | string | No |

**Response** (HTMX):
- `200 OK` — `@TeamDetailHeader(data)` (partial: team alias + budget info panel only)
- `200 OK` with error banner on validation failure

**HTMX target**: `#team-detail-header`

---

### POST /ui/teams/{team_id}/delete

**Purpose**: Delete team (after dialog confirmation)
**Handler**: `UIHandler.handleTeamDelete`
**Auth**: Required
**Content-Type**: `application/x-www-form-urlencoded`

**Response** (HTMX):
- `200 OK` — redirect header `HX-Redirect: /ui/teams` (navigate back to list)
- `200 OK` with error — if delete fails (should not happen for teams)

**DB calls**: `h.DB.DeleteTeam(ctx, team_id)`

---

### POST /ui/teams/{team_id}/block

**Purpose**: Block a team
**Handler**: `UIHandler.handleTeamBlock`
**Auth**: Required

**Response** (HTMX):
- `200 OK` — `@TeamStatusBadge(blocked=true)` partial — replaces `#team-status-{id}`

**DB calls**: `h.DB.BlockTeam(ctx, team_id)`

---

### POST /ui/teams/{team_id}/unblock

**Purpose**: Unblock a team
**Handler**: `UIHandler.handleTeamUnblock`
**Auth**: Required

**Response** (HTMX):
- `200 OK` — `@TeamStatusBadge(blocked=false)` partial — replaces `#team-status-{id}`

**DB calls**: `h.DB.UnblockTeam(ctx, team_id)`

---

### POST /ui/teams/{team_id}/members/add

**Purpose**: Add a user to the team
**Handler**: `UIHandler.handleTeamMemberAdd`
**Auth**: Required
**Content-Type**: `application/x-www-form-urlencoded`

**Form Fields**:
| Field | Type | Required |
|-------|------|----------|
| `user_id` | string | Yes |
| `role` | string | No (default: "member") |

**Response** (HTMX):
- `200 OK` — `@TeamMembersTablePartial(members)` — replaces `#team-members-table`
- `200 OK` with error banner — if user already a member

**DB calls**:
1. Check if user_id in team.Members[] — if yes, return error
2. `h.DB.AddTeamMember(ctx, team_id, user_id)`
3. Parse current members_with_roles + append new entry + `h.DB.UpdateTeamMemberRole(ctx, ...)`

---

### POST /ui/teams/{team_id}/members/remove

**Purpose**: Remove a user from the team
**Handler**: `UIHandler.handleTeamMemberRemove`
**Auth**: Required
**Form Fields**: `user_id` (string, required)

**Response** (HTMX):
- `200 OK` — `@TeamMembersTablePartial(members)` — replaces `#team-members-table`

**DB calls**:
1. `h.DB.RemoveTeamMember(ctx, team_id, user_id)`
2. Remove from members_with_roles JSONB + `h.DB.UpdateTeamMemberRole(ctx, ...)`

---

### POST /ui/teams/{team_id}/models/add

**Purpose**: Add a model to team's allowed list
**Handler**: `UIHandler.handleTeamModelAdd`
**Auth**: Required
**Form Fields**: `model_name` (string, required)

**Response** (HTMX):
- `200 OK` — `@TeamModelsListPartial(models)` — replaces `#team-models-list`

**DB calls**: `h.DB.AddTeamModel(ctx, team_id, model_name)`

---

### POST /ui/teams/{team_id}/models/remove

**Purpose**: Remove a model from team's allowed list
**Handler**: `UIHandler.handleTeamModelRemove`
**Auth**: Required
**Form Fields**: `model_name` (string, required)

**Response** (HTMX):
- `200 OK` — `@TeamModelsListPartial(models)` — replaces `#team-models-list`

**DB calls**: `h.DB.RemoveTeamModel(ctx, team_id, model_name)`

---

## Organizations Routes

### GET /ui/orgs

**Purpose**: Organizations list page — full page
**Handler**: `UIHandler.handleOrgs`
**Auth**: Required
**Query Params**:
- `page` (int, default 1)
- `search` (string) — filter by org alias

**Response**: `200 OK` — full HTML `@AppLayout` + `@OrgsPage(data)`

---

### GET /ui/orgs/table

**Purpose**: Orgs table partial — HTMX swap
**Handler**: `UIHandler.handleOrgsTable`
**Auth**: Required
**Query Params**: same as GET /ui/orgs
**Response**: `200 OK` — `@OrgsTablePartial(data)` partial only

---

### POST /ui/orgs/create

**Purpose**: Create a new organization
**Handler**: `UIHandler.handleOrgCreate`
**Auth**: Required
**Form Fields**:
| Field | Type | Required |
|-------|------|----------|
| `org_alias` | string | Yes |
| `max_budget` | float | No |
| `tpm_limit` | int | No |
| `rpm_limit` | int | No |
| `models` | string[] | No |

**Response** (HTMX):
- `200 OK` — `@OrgsTablePartial(data)` — refreshed table with new org at top
- `200 OK` with error — validation error

**DB calls**: `h.DB.CreateOrganization(ctx, db.CreateOrganizationParams{...})`

---

### GET /ui/orgs/{org_id}

**Purpose**: Organization detail page — full page
**Handler**: `UIHandler.handleOrgDetail`
**Auth**: Required
**Path Params**: `org_id` (string, UUID)

**Response**:
- `200 OK` — full HTML `@AppLayout` + `@OrgDetailPage(data)`
- `404` — redirect to `/ui/orgs` if org not found

---

### POST /ui/orgs/{org_id}/update

**Purpose**: Update org alias, budget limits
**Handler**: `UIHandler.handleOrgUpdate`
**Auth**: Required
**Form Fields**:
| Field | Type | Required |
|-------|------|----------|
| `org_alias` | string | Yes |
| `max_budget` | float | No |

**Response** (HTMX):
- `200 OK` — `@OrgDetailHeaderPartial(data)` — replaces `#org-detail-header`

**DB calls**: `h.DB.UpdateOrganization(ctx, db.UpdateOrganizationParams{...})`

---

### POST /ui/orgs/{org_id}/delete

**Purpose**: Delete organization
**Handler**: `UIHandler.handleOrgDelete`
**Auth**: Required

**Response** (HTMX):
- `200 OK` with `HX-Redirect: /ui/orgs` on success
- `200 OK` with error banner — if org has dependent teams

**DB calls**:
1. Check `h.DB.ListTeamsByOrganization(ctx, org_id)` — if len > 0, return error
2. `h.DB.DeleteOrganization(ctx, org_id)`

---

### POST /ui/orgs/{org_id}/members/add

**Purpose**: Add a user to the organization
**Handler**: `UIHandler.handleOrgMemberAdd`
**Auth**: Required
**Form Fields**:
| Field | Type | Required |
|-------|------|----------|
| `user_id` | string | Yes |
| `user_role` | string | Yes | One of: admin, member, proxy_admin, org_admin |

**Response** (HTMX):
- `200 OK` — `@OrgMembersTablePartial(members)` — replaces `#org-members-table`
- `200 OK` with error — duplicate member

**DB calls**: `h.DB.AddOrgMember(ctx, db.AddOrgMemberParams{...})`

---

### POST /ui/orgs/{org_id}/members/update

**Purpose**: Update a member's role
**Handler**: `UIHandler.handleOrgMemberUpdate`
**Auth**: Required
**Form Fields**: `user_id` (string), `user_role` (string)

**Response** (HTMX):
- `200 OK` — `@OrgMembersTablePartial(members)` — replaces `#org-members-table`

**DB calls**: `h.DB.UpdateOrgMember(ctx, db.UpdateOrgMemberParams{...})`

---

### POST /ui/orgs/{org_id}/members/remove

**Purpose**: Remove a member from the organization
**Handler**: `UIHandler.handleOrgMemberRemove`
**Auth**: Required
**Form Fields**: `user_id` (string)

**Response** (HTMX):
- `200 OK` — `@OrgMembersTablePartial(members)` — replaces `#org-members-table`

**DB calls**: `h.DB.DeleteOrgMember(ctx, db.DeleteOrgMemberParams{...})`

---

## File Modifications Required

### `internal/ui/routes.go`

Add to the protected group (inside `r.Use(h.sessionAuth)` block):

```go
// Teams
r.Get("/teams", h.handleTeams)
r.Get("/teams/table", h.handleTeamsTable)
r.Post("/teams/create", h.handleTeamCreate)
r.Get("/teams/{team_id}", h.handleTeamDetail)
r.Post("/teams/{team_id}/update", h.handleTeamUpdate)
r.Post("/teams/{team_id}/delete", h.handleTeamDelete)
r.Post("/teams/{team_id}/block", h.handleTeamBlock)
r.Post("/teams/{team_id}/unblock", h.handleTeamUnblock)
r.Post("/teams/{team_id}/members/add", h.handleTeamMemberAdd)
r.Post("/teams/{team_id}/members/remove", h.handleTeamMemberRemove)
r.Post("/teams/{team_id}/models/add", h.handleTeamModelAdd)
r.Post("/teams/{team_id}/models/remove", h.handleTeamModelRemove)

// Organizations
r.Get("/orgs", h.handleOrgs)
r.Get("/orgs/table", h.handleOrgsTable)
r.Post("/orgs/create", h.handleOrgCreate)
r.Get("/orgs/{org_id}", h.handleOrgDetail)
r.Post("/orgs/{org_id}/update", h.handleOrgUpdate)
r.Post("/orgs/{org_id}/delete", h.handleOrgDelete)
r.Post("/orgs/{org_id}/members/add", h.handleOrgMemberAdd)
r.Post("/orgs/{org_id}/members/update", h.handleOrgMemberUpdate)
r.Post("/orgs/{org_id}/members/remove", h.handleOrgMemberRemove)
```

### `internal/ui/pages/layout.templ`

Add sidebar nav items (after Models, before Usage):

```templ
@navItem("/ui/teams", "Teams", icon.Users(icon.Props{Size: 16}), activePath)
@navItem("/ui/orgs", "Organizations", icon.Building2(icon.Props{Size: 16}), activePath)
```

---

## New Files to Create

| File | Purpose |
|------|---------|
| `internal/ui/handler_teams.go` | Teams list + create + block/unblock handlers |
| `internal/ui/handler_teams_detail.go` | Team detail + member/model management handlers |
| `internal/ui/handler_orgs.go` | Orgs list + create handlers |
| `internal/ui/handler_orgs_detail.go` | Org detail + member management handlers |
| `internal/ui/pages/teams.templ` | Teams page + detail templates |
| `internal/ui/pages/orgs.templ` | Orgs page + detail templates |
