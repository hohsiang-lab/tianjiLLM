# Data Model: Team & Organization Admin UI

**Branch**: `001-team-org-admin-ui` | **Phase**: 1 | **Date**: 2026-02-26

---

## Database Entities (Existing — No Schema Changes)

All DB tables already exist. This feature adds **no new migrations**.

### TeamTable

```sql
CREATE TABLE "TeamTable" (
    team_id          TEXT PRIMARY KEY,      -- UUID string
    team_alias       TEXT,
    organization_id  TEXT,                  -- FK → OrganizationTable (nullable)
    admins           TEXT[],                -- user IDs who are admins
    members          TEXT[],                -- user IDs
    members_with_roles JSONB,              -- [{user_id, role}] structured roles
    models           TEXT[],               -- allowed model names (empty = all)
    max_budget       DOUBLE PRECISION,
    spend            DOUBLE PRECISION NOT NULL DEFAULT 0,
    budget_id        TEXT,                 -- FK → BudgetTable
    tpm_limit        BIGINT,
    rpm_limit        BIGINT,
    budget_duration  TEXT,                 -- "daily" | "weekly" | "monthly"
    budget_reset_at  TIMESTAMP WITH TIME ZONE,
    blocked          BOOLEAN NOT NULL DEFAULT FALSE,
    metadata         JSONB,
    created_by       TEXT,
    created_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by       TEXT,
    updated_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
```

### OrganizationTable

```sql
CREATE TABLE "OrganizationTable" (
    organization_id    TEXT PRIMARY KEY,    -- UUID string
    organization_alias TEXT,
    max_budget         DOUBLE PRECISION,
    spend              DOUBLE PRECISION NOT NULL DEFAULT 0,
    budget_id          TEXT,
    tpm_limit          BIGINT,
    rpm_limit          BIGINT,
    budget_reset_at    TIMESTAMP WITH TIME ZONE,
    models             TEXT[],
    metadata           JSONB,
    created_by         TEXT,
    created_at         TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_by         TEXT,
    updated_at         TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
```

### OrganizationMembership

```sql
CREATE TABLE "OrganizationMembership" (
    user_id         TEXT NOT NULL,
    organization_id TEXT NOT NULL,           -- FK → OrganizationTable
    user_role       TEXT,                    -- "admin" | "member" | "proxy_admin" | "org_admin"
    spend           DOUBLE PRECISION DEFAULT 0,
    budget_id       TEXT,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, organization_id)
);
```

---

## New sqlc Queries

### team.sql additions

```sql
-- name: ListTeamsByOrganization :many
SELECT * FROM "TeamTable"
WHERE organization_id = $1
ORDER BY created_at DESC;
```

### organization.sql additions

```sql
-- name: CountTeamsPerOrganization :many
SELECT organization_id, COUNT(*)::bigint AS team_count
FROM "TeamTable"
WHERE organization_id IS NOT NULL
GROUP BY organization_id;

-- name: CountMembersPerOrganization :many
SELECT organization_id, COUNT(*)::bigint AS member_count
FROM "OrganizationMembership"
GROUP BY organization_id;
```

---

## UI Data Structures (Go types in `internal/ui/pages/`)

These are the Go structs passed to templ templates. They live in the templ files alongside the templates.

### Teams List Page

```go
// File: internal/ui/pages/teams.templ

type TeamRow struct {
    TeamID        string
    TeamAlias     string
    OrgID         string    // empty if no org
    OrgAlias      string    // empty if no org ("No Organization" displayed)
    MemberCount   int       // len(members)
    Spend         float64
    MaxBudget     *float64  // nil = unlimited
    ModelCount    int       // len(models), 0 = all models
    Blocked       bool
    CreatedAt     time.Time
}

type TeamsPageData struct {
    Teams      []TeamRow
    Page       int
    TotalPages int
    TotalCount int
    Search     string       // search term (filters by alias)
    FilterOrgID string      // filter by org
    Orgs       []OrgOption  // for filter dropdown
    AvailableModels []string // for create form multi-select
    Error      string       // inline error message (empty = no error)
}

type OrgOption struct {
    ID    string
    Alias string
}
```

### Team Detail Page

```go
type TeamMemberRow struct {
    UserID string
    Role   string  // from members_with_roles JSONB; empty if not in roles map
}

type TeamDetailData struct {
    TeamID         string
    TeamAlias      string
    OrgID          string       // empty if no org
    OrgAlias       string       // empty if no org
    Admins         []string     // admin user IDs
    Members        []TeamMemberRow
    Models         []string     // allowed models (empty = all)
    Spend          float64
    MaxBudget      *float64
    TPMLimit       *int64
    RPMLimit       *int64
    BudgetDuration string       // "daily" | "weekly" | "monthly" | ""
    BudgetResetAt  *time.Time
    Blocked        bool
    MetadataJSON   string       // raw JSON string for display/edit
    AvailableModels []string    // for model add dropdown
    Error          string       // inline error (empty = no error)
    Success        string       // inline success message
}
```

### Organizations List Page

```go
// File: internal/ui/pages/orgs.templ

type OrgRow struct {
    OrgID       string
    OrgAlias    string
    TeamCount   int64
    MemberCount int64
    Spend       float64
    MaxBudget   *float64  // nil = unlimited
    ModelCount  int       // len(models)
    CreatedAt   time.Time
}

type OrgsPageData struct {
    Orgs       []OrgRow
    Page       int
    TotalPages int
    TotalCount int
    Search     string    // filters by alias
    AvailableModels []string // for create form multi-select
    Error      string
}
```

### Organization Detail Page

```go
type OrgMemberRow struct {
    UserID    string
    Role      string
    Spend     float64
    CreatedAt time.Time
}

type OrgDetailData struct {
    OrgID       string
    OrgAlias    string
    Spend       float64
    MaxBudget   *float64
    Models      []string
    TPMLimit    *int64
    RPMLimit    *int64
    Members     []OrgMemberRow
    Teams       []TeamRow    // teams belonging to this org (summary view)
    MetadataJSON string
    AvailableModels []string  // for model management
    AvailableRoles  []string  // ["admin", "member", "proxy_admin", "org_admin"]
    Error       string
    Success     string
}
```

---

## Entity Relationships (for UI navigation)

```
OrganizationTable
    │
    ├── has many TeamTable (via organization_id)
    │       │
    │       └── has members: TeamTable.members[] (user IDs)
    │               └── roles in: TeamTable.members_with_roles JSONB
    │
    └── has many OrganizationMembership (via organization_id)
            └── user_id, user_role, spend, created_at
```

**Navigation flow**:
```
/ui/teams           → Teams list
/ui/teams/{id}      → Team detail (members, models, budget, metadata)
/ui/orgs            → Orgs list (with team count, member count)
/ui/orgs/{id}       → Org detail (members table, teams summary, budget)
```

---

## State Transitions

### Team blocked flag

```
Active (blocked=false)
    → [click Block]    → Blocked (blocked=true)
    → [click Unblock]  → Active (blocked=false)
```

UI: Status badge changes color + button label toggles. HTMX targets `#team-status-{id}` partial.

### Member addition

```
No member
    → [submit add form with user_id]
    → Validate: user_id not already in members[]
    → DB: AddTeamMember (append to members[]) + UpdateTeamMemberRole (upsert JSONB)
    → Success: member row appears in table
    → Error: "User already a member" banner
```

---

## Validation Rules

| Field | Rule | Error Message |
|-------|------|---------------|
| team_alias | Non-empty string | "Team alias is required" |
| org_id (create team) | Optional; if provided, must exist in OrganizationTable | "Organization not found" |
| user_id (add member) | Non-empty; not already in members[] | "User already a member of this team" |
| user_id (add org member) | Non-empty; not already in OrganizationMembership | "User already a member of this organization" |
| max_budget | Optional; if provided, must be ≥ 0 | "Budget must be non-negative" |
| org_alias | Non-empty string | "Organization alias is required" |
| user_role (org member) | One of: admin, member, proxy_admin, org_admin | "Invalid role" |
| delete org | No dependent teams | "Remove all teams before deleting this organization" |

---

## Handler Data Loading (Phase 1 Design)

### Teams List (`loadTeamsPageData`)

```
1. Parse query params: page, search, filter_org_id
2. h.DB.ListTeams(ctx)
3. Filter in Go: by search (alias contains) and filter_org_id
4. h.DB.ListOrganizations(ctx) → build OrgID→Alias map for OrgAlias lookup
5. Build []TeamRow (compute MemberCount = len(members), ModelCount = len(models))
6. Paginate slice: [offset:offset+50]
7. h.DB.ListOrganizations(ctx) → []OrgOption for filter dropdown
8. h.loadAvailableModelNames(ctx) → AvailableModels
9. Return TeamsPageData
```

### Team Detail (`loadTeamDetailData`)

```
1. chi.URLParam(r, "team_id")
2. h.DB.GetTeam(ctx, team_id)
3. Parse members_with_roles JSONB → []TeamMemberRow
4. Look up OrgAlias if org_id != ""
5. h.loadAvailableModelNames(ctx)
6. Return TeamDetailData
```

### Orgs List (`loadOrgsPageData`)

```
1. Parse query params: page, search
2. h.DB.ListOrganizations(ctx)
3. h.DB.CountTeamsPerOrganization(ctx) → map[org_id]int64
4. h.DB.CountMembersPerOrganization(ctx) → map[org_id]int64
5. Filter in Go by search
6. Build []OrgRow (merge counts from maps)
7. Paginate slice
8. h.loadAvailableModelNames(ctx)
9. Return OrgsPageData
```

### Org Detail (`loadOrgDetailData`)

```
1. chi.URLParam(r, "org_id")
2. h.DB.GetOrganization(ctx, org_id)
3. h.DB.ListOrgMembers(ctx, org_id) → []OrgMemberRow
4. h.DB.ListTeamsByOrganization(ctx, org_id) → []TeamRow (summary)
5. h.loadAvailableModelNames(ctx)
6. Return OrgDetailData
```
