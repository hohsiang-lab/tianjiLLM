# Quickstart: Team & Organization Admin UI

**Branch**: `001-team-org-admin-ui` | **Phase**: 1 | **Date**: 2026-02-26

## Prerequisites

- Go 1.24.4+
- PostgreSQL running (or use `make e2e` containerized setup)
- `proxy_config.yaml` with `database_url` set
- `.env` file with `MASTER_KEY` and `DATABASE_URL`

## Development Setup

```bash
# 1. Start the server with hot reload
make dev
# Watches .go, .templ, .css — auto-rebuilds on save

# 2. (Alternative) Manual build cycle
make ui    # templ generate + tailwind build (no Go compile)
make build # full build → bin/tianji
make run   # run the server
```

## Adding New sqlc Queries

After modifying any `.sql` file in `internal/db/queries/`:

```bash
make generate  # runs sqlc generate → updates internal/db/*.sql.go
```

**Files modified for this feature**:
- `internal/db/queries/team.sql` — add `ListTeamsByOrganization`
- `internal/db/queries/organization.sql` — add `CountTeamsPerOrganization`, `CountMembersPerOrganization`

## Templ Workflow

```bash
# After editing .templ files:
templ generate   # converts .templ → _templ.go
# OR
make ui          # templ + tailwind together

# The generated _templ.go files MUST be committed alongside .templ files.
```

## Testing

```bash
# Unit tests
go test ./internal/ui/... -v

# E2E tests (requires PostgreSQL at postgres://tianji:tianji@localhost:5433/tianji_e2e)
make e2e
make e2e-headed   # with visible browser

# Lint
make lint
```

## Implementation Order

1. **Add sqlc queries** to `.sql` files → run `make generate`
2. **Create** `internal/ui/pages/teams.templ` + `orgs.templ` → run `templ generate`
3. **Create** `internal/ui/handler_teams.go` + `handler_teams_detail.go`
4. **Create** `internal/ui/handler_orgs.go` + `handler_orgs_detail.go`
5. **Modify** `internal/ui/routes.go` — add new routes
6. **Modify** `internal/ui/pages/layout.templ` — add sidebar nav items → run `templ generate`
7. **Build**: `make build`
8. **Test**: `make check` (lint + test + build)

## Key File Locations

| File | Purpose |
|------|---------|
| `internal/db/queries/team.sql` | Team sqlc queries |
| `internal/db/queries/organization.sql` | Org sqlc queries |
| `internal/db/queries/org_membership.sql` | Org membership queries |
| `internal/ui/routes.go` | Route registration |
| `internal/ui/handler_keys.go` | Reference implementation for handler pattern |
| `internal/ui/pages/keys.templ` | Reference implementation for page template pattern |
| `internal/ui/pages/layout.templ` | AppLayout + sidebar nav |
| `internal/ui/pages/teams.templ` | New: Teams list + detail |
| `internal/ui/pages/orgs.templ` | New: Orgs list + detail |
| `internal/ui/handler_teams.go` | New: Teams list handlers |
| `internal/ui/handler_teams_detail.go` | New: Team detail handlers |
| `internal/ui/handler_orgs.go` | New: Orgs list handlers |
| `internal/ui/handler_orgs_detail.go` | New: Org detail handlers |

## Accessing the UI

After `make run` (default port 4000):
- Teams list: http://localhost:4000/ui/teams
- Team detail: http://localhost:4000/ui/teams/{team_id}
- Orgs list:   http://localhost:4000/ui/orgs
- Org detail:  http://localhost:4000/ui/orgs/{org_id}

Login required with master key at http://localhost:4000/ui/login

## Common Pitfalls

1. **Forget `make generate`** after editing `.sql` → sqlc Go types won't match
2. **Forget `templ generate`** after editing `.templ` → stale generated Go code
3. **members_with_roles JSONB** is `[]byte` in sqlc — must `json.Unmarshal` before use
4. **chi URL params**: use `chi.URLParam(r, "team_id")` not `r.URL.Query().Get("team_id")`
5. **HTMX CORS headers**: already handled by existing middleware — no changes needed
