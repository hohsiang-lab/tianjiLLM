# UI Design Spec: Team & Organization Admin UI

**Branch**: `001-team-org-admin-ui` | **Date**: 2026-02-26  
**Author**: 張大千 (UI/UX Designer)

---

## Design Principles

1. **Copy keys.templ patterns exactly** — same layout, same component usage, same Tailwind classes
2. **Card-wrapped tables** — `card.Card > card.Content(p-0) > table.Table`
3. **Dialog-based forms** — all create/edit via `dialog.Dialog` + `dialog.Content`
4. **HTMX partial swap** — mutations target `#xxx-table` div, no full page reload
5. **Toast for feedback** — `toast.Toast` via OOB swap after mutations
6. **Pagination** — `pagination.CreatePagination` + HTMX `hx-get` on each page link

---

## Sidebar Navigation Update

### Position
Add two items after "Models" and before "Usage" in `layout.templ`:

```templ
@navItem("/ui/teams", "Teams", "users", activePath)
@navItem("/ui/orgs", "Organizations", "building-2", activePath)
```

### Icons
- **Teams**: `users` (Lucide users icon — two people)
- **Organizations**: `building-2` (Lucide building icon)

Both icons exist in `icon_data.go` and `icon_defs.go`.

---

## Page 1: Teams List (`/ui/teams`)

### ASCII Wireframe

```
┌─────────────────────────────────────────────────────────────┐
│ [Sidebar]  │  ☰  Teams                                     │
│            │─────────────────────────────────────────────────│
│ Dashboard  │                                                │
│ API Keys   │  [ 🔍 Filter by alias...  ] [Org ▼] [+ New Team]│
│ Models     │                                                │
│ ★ Teams    │  ┌─Card──────────────────────────────────────┐ │
│ ★ Orgs     │  │ Alias │ Org │ Members│Spend/Budget│Models│…│ │
│ Usage      │  │───────┼─────┼────────┼────────────┼──────┼─│ │
│ Logs       │  │ team-a│ Acme│   5    │ $12 / $100 │  3   │…│ │
│            │  │ team-b│  -  │   2    │ $0 / Unlim │  All │…│ │
│            │  │       │     │        │            │      │ │ │
│            │  └───────────────────────────────────────────┘ │
│            │                                                │
│            │  Showing 1–50 of 120    « 1 2 3 ... 3 »       │
│            │                                                │
│ Logout     │                                                │
└─────────────────────────────────────────────────────────────┘
```

### Components Used
| Component | Source | Notes |
|-----------|--------|-------|
| `AppLayout` | `layout.templ` | Standard page wrapper |
| `card.Card` + `card.Content(p-0)` | `card/` | Wraps table |
| `table.*` | `table/` | Table, Header, Body, Row, Head, Cell |
| `badge.Badge` | `badge/` | Status (Active/Blocked), model count |
| `button.Button` | `button/` | Create, Block/Unblock, Delete |
| `dialog.*` | `dialog/` | Create Team form |
| `input.Input` | `input/` | Form fields |
| `icon.*` | `icon/` | Search, Plus, etc. |
| `pagination.*` | `pagination/` | Page nav |
| `toast.*` | `toast/` | Success/error feedback |

### Table Columns

| # | Column | Content | Class |
|---|--------|---------|-------|
| 1 | Alias | Clickable link → `/ui/teams/{id}` | `font-medium text-primary hover:underline` |
| 2 | Organization | OrgAlias or `<span class="text-muted-foreground text-xs">-</span>` | `text-xs` |
| 3 | Members | `len(members)` count | `text-xs` |
| 4 | Spend / Budget | `$X.XX / $Y.YY` or `Unlimited` | `text-xs` (same as keys page) |
| 5 | Models | Count or `badge.Badge(Secondary) "All Models"` | Same pattern as keys |
| 6 | Status | `badge.Badge(Default)` Active / `badge.Badge(Destructive)` Blocked | |
| 7 | Actions | Block/Unblock + Delete buttons | `text-right`, `flex justify-end gap-1` |

### Filter Toolbar
Layout: `grid items-center gap-2` with `grid-template-columns: 2fr 1fr auto`

```
[ 🔍 Filter by alias... (hx-get=/ui/teams/table) ] [ Org dropdown ] [ + New Team dialog ]
```

- Alias filter: `<input>` with search icon (same pattern as keys page)
- Org filter: `<select>` dropdown with OrgOption list
- Both filters: `hx-trigger="input changed delay:300ms"`, `hx-target="#teams-table"`, `hx-include="#team-filters"`

### Create Team Dialog
Trigger: `dialog.Dialog(ID: "create-team-dialog")`

Form fields:
- **Team Alias** (required) — `input.Input`
- **Optional Settings** (collapsible `<details>`) — same pattern as keys create form:
  - Organization dropdown (`<select>`)
  - Max Budget / Budget Duration (grid-cols-2)
  - TPM / RPM Limit (grid-cols-2)
  - Models multi-select (reuse `modelsMultiSelect` pattern)

Footer: Cancel (`dialog.Close`) + Create (`button.Button type=submit`)

HTMX: `hx-post="/ui/teams/create"` → `hx-target="#teams-table"`

### Interaction Flows

| Action | Trigger | HTMX | Target | Response |
|--------|---------|------|--------|----------|
| Filter by alias | input change 300ms | GET /ui/teams/table | #teams-table | TeamsTablePartial |
| Filter by org | select change | GET /ui/teams/table | #teams-table | TeamsTablePartial |
| Create team | form submit | POST /ui/teams/create | #teams-table | TeamsTableWithToast |
| Block team | click Block btn | POST /ui/teams/{id}/block | #teams-table | TeamsTableWithToast |
| Unblock team | click Unblock btn | POST /ui/teams/{id}/unblock | #teams-table | TeamsTableWithToast |
| Delete team | click Delete → confirm | POST /ui/teams/{id}/delete | #teams-table | TeamsTableWithToast |
| Navigate to detail | click alias link | browser nav | full page | TeamDetailPage |

### States

**Empty state** (no teams):
```templ
@table.Row() {
    @table.Cell(table.CellProps{Attributes: templ.Attributes{"colspan": "7"}}) {
        <div class="py-8 text-center text-muted-foreground">No teams found</div>
    }
}
```

**Filtered empty**: `"No teams match your filters"`

**Error state**: Toast OOB with `toast.VariantDestructive`

**Loading**: HTMX adds `.htmx-request` class automatically; no custom skeleton needed (consistent with keys page which has no loading skeleton)

---

## Page 2: Team Detail (`/ui/teams/{id}`)

### ASCII Wireframe

```
┌─────────────────────────────────────────────────────────────┐
│ [Sidebar]  │  ☰  team-alpha — Team Detail                  │
│            │─────────────────────────────────────────────────│
│            │                                                │
│            │  team-alpha  [Active]              [Edit] [Del]│
│            │  Org: Acme Corp · Created 2026-01-15          │
│            │                               [← Back to Teams]│
│            │                                                │
│            │  ┌─────────┐ ┌──────────┐ ┌──────────────┐    │
│            │  │Spend/Bdg│ │Rate Limit│ │Allowed Models│    │
│            │  │ $45.00  │ │TPM 10000 │ │ gpt-4o       │    │
│            │  │ /$100   │ │RPM 100   │ │ claude-3     │    │
│            │  │ 45% used│ │          │ │              │    │
│            │  └─────────┘ └──────────┘ └──────────────┘    │
│            │                                                │
│            │  Members                [user_id] [role▼] [Add]│
│            │  ┌─Card────────────────────────────────────┐  │
│            │  │ User ID    │ Role   │ Actions           │  │
│            │  │────────────┼────────┼───────────────────│  │
│            │  │ user-001   │ admin  │ [Remove]          │  │
│            │  │ user-002   │ member │ [Remove]          │  │
│            │  └─────────────────────────────────────────┘  │
│            │                                                │
│            │  Models                   [model ▼] [Add]     │
│            │  ┌─Card────────────────────────────────────┐  │
│            │  │ gpt-4o [Remove]  claude-3 [Remove]      │  │
│            │  └─────────────────────────────────────────┘  │
│            │                                                │
│            │  Metadata                                      │
│            │  ┌─Card────────────────────────────────────┐  │
│            │  │ { "env": "prod" }                       │  │
│            │  └─────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Layout Structure

Follows `key_detail.templ` pattern exactly:

1. **Header section** (`id="team-detail-header"`)
   - Team alias + Status badge (Active/Blocked)
   - Org link + Created date (subtext)
   - Action buttons: Edit (dialog), Block/Unblock, Delete (dialog), Back to Teams

2. **Overview cards** — `grid gap-4 md:grid-cols-3 pt-4`
   - **Spend/Budget card** — same as key detail: progress bar, budget duration, reset date
   - **Rate Limits card** — TPM/RPM display (same as key detail)
   - **Allowed Models card** — badges for each model, or "All models allowed"

3. **Members section** (`id="team-members-table"`)
   - Add member form: `[input: user_id] [select: role] [Add button]`
   - Table: User ID, Role, Actions (Remove)
   - HTMX: POST /ui/teams/{id}/members/add → swap #team-members-table

4. **Models section** (`id="team-models-list"`)
   - Add model: `[select: model_name from AvailableModels] [Add button]`
   - Display: flex-wrap badges with Remove buttons
   - Empty: "Inherited / All models" label

5. **Metadata section**
   - Read-only `<pre>` block with raw JSON (same as key detail settings)

### Edit Team Dialog
Modal with: Team Alias, Max Budget, Budget Duration, TPM/RPM limits.

HTMX: `hx-post="/ui/teams/{id}/update"` → target `#team-detail-header`

### Delete Team Dialog
Same pattern as `DeleteConfirmDialog` in `key_detail.templ`:
- Warning box with destructive styling
- Type alias to confirm
- Submit → `HX-Redirect: /ui/teams`

### Interaction Flows

| Action | HTMX | Target |
|--------|------|--------|
| Edit team | POST /ui/teams/{id}/update | #team-detail-header |
| Block/Unblock | POST /ui/teams/{id}/block or /unblock | #team-detail-header |
| Delete team | POST /ui/teams/{id}/delete | HX-Redirect: /ui/teams |
| Add member | POST /ui/teams/{id}/members/add | #team-members-table |
| Remove member | POST /ui/teams/{id}/members/remove | #team-members-table |
| Add model | POST /ui/teams/{id}/models/add | #team-models-list |
| Remove model | POST /ui/teams/{id}/models/remove | #team-models-list |

### States

**Empty members**: `<div class="py-4 text-center text-muted-foreground text-sm">No members yet</div>`

**Empty models**: `<div class="text-sm text-muted-foreground">Inherited / All models</div>`

**Not found**: Redirect to `/ui/teams` (same as key not found pattern)

**Error (add member)**: Toast with `"User already a member of this team"` or `"User ID is required"`

---

## Page 3: Organizations List (`/ui/orgs`)

### ASCII Wireframe

```
┌─────────────────────────────────────────────────────────────┐
│ [Sidebar]  │  ☰  Organizations                             │
│            │─────────────────────────────────────────────────│
│            │                                                │
│            │  [ 🔍 Filter by alias...  ]       [+ New Org]  │
│            │                                                │
│            │  ┌─Card──────────────────────────────────────┐ │
│            │  │ Alias │ Teams│Members│Spend/Budget│Models│…│ │
│            │  │───────┼──────┼───────┼────────────┼──────┼─│ │
│            │  │ Acme  │   3  │  12   │$50 / $500  │  5   │…│ │
│            │  │ Beta  │   1  │   4   │$0 / Unlim  │ All  │…│ │
│            │  └───────────────────────────────────────────┘ │
│            │                                                │
│            │  Showing 1–50 of 20     « 1 »                  │
└─────────────────────────────────────────────────────────────┘
```

### Components Used
Same as Teams list (card-wrapped table, pagination, dialog, toast).

### Table Columns

| # | Column | Content |
|---|--------|---------|
| 1 | Alias | Clickable link → `/ui/orgs/{id}` (`font-medium text-primary hover:underline`) |
| 2 | Teams | Team count (from `CountTeamsPerOrganization`) |
| 3 | Members | Member count (from `CountMembersPerOrganization`) |
| 4 | Spend / Budget | `$X.XX / $Y.YY` or `Unlimited` |
| 5 | Models | Count or `badge "All Models"` |
| 6 | Created | Date `text-xs` |
| 7 | Actions | Edit (dialog), Delete (`hx-confirm`) |

### Filter Toolbar
Layout: `flex items-center gap-2` (simpler than teams — no org dropdown)

```
[ 🔍 Filter by alias... ] [+ New Organization]
```

### Create Org Dialog
Fields: Org Alias (required), Max Budget, TPM/RPM Limit, Models multi-select.
Same form pattern as create team.

### Interaction Flows

| Action | HTMX | Target |
|--------|------|--------|
| Filter | GET /ui/orgs/table | #orgs-table |
| Create org | POST /ui/orgs/create | #orgs-table |
| Delete org | POST /ui/orgs/{id}/delete + hx-confirm | #orgs-table |
| Navigate to detail | click alias link | full page |

### States

**Empty**: `"No organizations found"` (colspan full row)

**Filtered empty**: `"No organizations match your search"`

**Delete error** (has teams): Toast `"Remove all teams before deleting this organization"`

---

## Page 4: Org Detail (`/ui/orgs/{id}`)

### ASCII Wireframe

```
┌─────────────────────────────────────────────────────────────┐
│ [Sidebar]  │  ☰  Acme Corp — Organization Detail           │
│            │─────────────────────────────────────────────────│
│            │                                                │
│            │  Acme Corp                     [Edit] [Delete] │
│            │  Created 2026-01-10           [← Back to Orgs] │
│            │                                                │
│            │  ┌─────────┐ ┌──────────┐ ┌──────────────┐    │
│            │  │Spend/Bdg│ │Rate Limit│ │Allowed Models│    │
│            │  │ $50.00  │ │TPM 50000 │ │ gpt-4o       │    │
│            │  │ /$500   │ │RPM 500   │ │ claude-3     │    │
│            │  └─────────┘ └──────────┘ └──────────────┘    │
│            │                                                │
│            │  Members          [user_id] [role ▼] [Add]    │
│            │  ┌─Card────────────────────────────────────┐  │
│            │  │ User ID  │ Role     │ Spend │ Joined│Act│  │
│            │  │──────────┼─────────┼───────┼───────┼───│  │
│            │  │ user-001 │org_admin│ $20   │ 01-15 │ … │  │
│            │  │ user-002 │member   │ $5    │ 01-20 │ … │  │
│            │  └─────────────────────────────────────────┘  │
│            │                                                │
│            │  Teams in this Organization                    │
│            │  ┌─Card────────────────────────────────────┐  │
│            │  │ team-alpha (5 members) → link            │  │
│            │  │ team-beta (2 members)  → link            │  │
│            │  └─────────────────────────────────────────┘  │
│            │                                                │
│            │  Metadata                                      │
│            │  ┌─Card────────────────────────────────────┐  │
│            │  │ { "tier": "enterprise" }                 │  │
│            │  └─────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### Layout Structure

1. **Header** (`id="org-detail-header"`)
   - Org alias + Created date
   - Actions: Edit (dialog), Delete (dialog), Back to Orgs

2. **Overview cards** — `grid gap-4 md:grid-cols-3 pt-4`
   - Spend/Budget, Rate Limits, Allowed Models (same card pattern)

3. **Members section** (`id="org-members-table"`)
   - Add form: `[input: user_id] [select: role (admin/member/proxy_admin/org_admin)] [Add]`
   - Table columns: User ID, Role (with change dropdown), Spend, Joined, Actions (Remove)
   - Role change: inline `<select>` with `hx-post="/ui/orgs/{id}/members/update"` on change
   - HTMX target: `#org-members-table`

4. **Teams section** (read-only list)
   - Card with list of teams belonging to this org
   - Each team: clickable link to `/ui/teams/{team_id}` with member count
   - Empty: `"No teams in this organization"`

5. **Metadata section**
   - Read-only `<pre>` JSON block

### Edit Org Dialog
Fields: Org Alias, Max Budget.
HTMX: `hx-post="/ui/orgs/{id}/update"` → target `#org-detail-header`

### Delete Org Dialog
Same confirm-by-typing pattern as team delete.
Server checks for dependent teams → returns error if any exist.

### Members Table — Role Change

Inline role update via `<select>`:
```html
<select hx-post="/ui/orgs/{id}/members/update"
        hx-vals='{"user_id":"xxx"}'
        hx-target="#org-members-table"
        hx-trigger="change"
        name="user_role">
  <option value="admin">admin</option>
  <option value="member">member</option>
  <option value="proxy_admin">proxy_admin</option>
  <option value="org_admin">org_admin</option>
</select>
```

### Interaction Flows

| Action | HTMX | Target |
|--------|------|--------|
| Edit org | POST /ui/orgs/{id}/update | #org-detail-header |
| Delete org | POST /ui/orgs/{id}/delete | HX-Redirect or error toast |
| Add member | POST /ui/orgs/{id}/members/add | #org-members-table |
| Change role | POST /ui/orgs/{id}/members/update | #org-members-table |
| Remove member | POST /ui/orgs/{id}/members/remove | #org-members-table |

### States

**Empty members**: `"No members yet"`

**Empty teams**: `"No teams in this organization"`

**Not found**: Redirect to `/ui/orgs`

**Delete error**: Toast `"Remove all teams before deleting this organization"`

---

## Component Reuse Summary

### Existing Components — Direct Reuse (No Changes)

| Component | Usage |
|-----------|-------|
| `card.Card`, `card.Content`, `card.Header`, `card.Title` | Wrap all tables and detail overview cards |
| `table.Table`, `Header`, `Body`, `Row`, `Head`, `Cell` | All list tables + members tables |
| `badge.Badge` (Default, Destructive, Secondary, Outline) | Status, model counts |
| `button.Button` (all variants + sizes) | All actions |
| `dialog.*` (Dialog, Trigger, Content, Header, Title, Description, Footer, Close, Script) | Create/Edit/Delete modals |
| `input.Input` + `input.Script` | All form inputs |
| `pagination.*` | List page pagination |
| `toast.Toast` + `toast.Script` | Mutation feedback |
| `icon.*` (Search, Plus, Users, Building2, etc.) | Sidebar nav, filter bar, buttons |
| `sidebar.*` | Nav items (via `navItem` helper) |

### Existing Patterns — Copy & Adapt

| Pattern | Source | Adaptation |
|---------|--------|------------|
| `KeysPage` layout | `keys.templ` | Same structure: filter bar → dialog → table div |
| `KeysTablePartial` | `keys.templ` | Same card-wrapped table + pagination block |
| `KeysTableWithToast` | `keys.templ` | Same OOB toast pattern for mutations |
| `createKeyForm` | `keys.templ` | Same form structure with `<details>` for optional fields |
| `modelsMultiSelect` | `keys.templ` | Reuse for team/org create forms (import from same package) |
| `KeyDetailPage` header | `key_detail.templ` | Same alias + badge + actions + back link layout |
| Overview cards (3-col grid) | `key_detail.templ` | Same Spend/Budget, Rate Limits, Models cards |
| `DeleteConfirmDialog` | `key_detail.templ` | Same type-to-confirm pattern |
| `settingsRow` / `settingsRowPre` | `key_detail.templ` | For metadata display |

### New Components / Templates to Create

| Template | File | Purpose |
|----------|------|---------|
| `TeamsPage` | `teams.templ` | Full page with AppLayout |
| `TeamsTablePartial` | `teams.templ` | Card table + pagination (HTMX target) |
| `TeamsTableWithToast` | `teams.templ` | Table + OOB toast |
| `teamRow` | `teams.templ` | Single table row |
| `createTeamForm` | `teams.templ` | Dialog form |
| `TeamDetailPage` | `teams.templ` | Full detail page |
| `TeamDetailHeader` | `teams.templ` | Header partial (HTMX swap target) |
| `TeamMembersTablePartial` | `teams.templ` | Members sub-table |
| `TeamModelsListPartial` | `teams.templ` | Models badge list |
| `OrgsPage` | `orgs.templ` | Full page |
| `OrgsTablePartial` | `orgs.templ` | Card table + pagination |
| `OrgsTableWithToast` | `orgs.templ` | Table + OOB toast |
| `orgRow` | `orgs.templ` | Single table row |
| `createOrgForm` | `orgs.templ` | Dialog form |
| `OrgDetailPage` | `orgs.templ` | Full detail page |
| `OrgDetailHeaderPartial` | `orgs.templ` | Header partial |
| `OrgMembersTablePartial` | `orgs.templ` | Members sub-table with role dropdown |

**No new shared components needed.** All UI is built from existing templUI primitives.

---

## Tailwind Class Conventions (from existing pages)

### Common patterns

| Element | Classes |
|---------|---------|
| Page content wrapper | `space-y-3` |
| Filter toolbar | `grid items-center gap-2` or `flex items-center gap-2` |
| Search input | `flex h-9 w-full rounded-md border border-input bg-transparent pl-8 pr-3 py-1 text-sm shadow-xs outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]` |
| Select dropdown | `flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm` |
| Form field label | `text-sm font-medium` |
| Required marker | `<span class="text-destructive">*</span>` |
| Form section spacing | `space-y-4 py-4` |
| Grid 2-col form fields | `grid grid-cols-2 gap-4` |
| Detail page header | `flex items-center justify-between` |
| Overview cards | `grid gap-4 md:grid-cols-3 pt-4` |
| Card stat number | `text-2xl font-bold` |
| Card stat sub | `text-sm text-muted-foreground` |
| Muted placeholder | `text-muted-foreground text-xs` |
| Monospace values | `font-mono text-xs` |
| Empty state cell | `py-8 text-center text-muted-foreground` |
| Actions cell | `text-right` + `flex justify-end gap-1` |
| Back link | `button.Button(VariantGhost)` wrapping text with ← |
| Section heading | `text-sm font-medium` |
| Add-member inline form | `flex items-center gap-2` |

### Color semantics

| State | Badge Variant | Color |
|-------|--------------|-------|
| Active | `Default` | Primary bg |
| Blocked | `Destructive` | Red bg |
| All Models | `Secondary` | Muted bg |
| Expired (if applicable) | `Outline` | Border only |
