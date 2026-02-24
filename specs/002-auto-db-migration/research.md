# Research: Auto-Run DB Migrations on Startup

**Feature**: 002-auto-db-migration
**Date**: 2026-02-25
**Updated**: 2026-02-25 (revised after context7 / source verification)

---

## Decision 1: Migration Library — golang-migrate/v4 ✅

**Decision**: Use `github.com/golang-migrate/migrate/v4` with:
- Source: `github.com/golang-migrate/migrate/v4/source/iofs` (embed.FS)
- Driver: `github.com/golang-migrate/migrate/v4/database/pgx/v5` (native pgx v5)
- File rename: `001_initial.sql` → `001_initial.up.sql` (all 10 files)

**Why the file rename is safe — confirmed sources**:

1. **sqlc officially supports golang-migrate `.up.sql` naming** (verified from sqlc 1.30.0 docs):
   > "sqlc is able to differentiate between up and down migrations.
   > sqlc ignores down migrations when parsing SQL files."
   > "sqlc supports parsing migrations from: golang-migrate, goose, ..."
   - sqlc docs show `001_initial.up.sql` as the exact recommended format
   - Existing 3-digit zero-padded prefix (`001_`..`010_`) matches sqlc's lexicographic
     ordering requirement — no additional changes needed

2. **golang-migrate iofs driver** (`source/parse.go`) requires `^([0-9]+)_(.*)\.(up|down)\.(.*)$`
   — silently skips non-matching files. Files must end in `.up.sql` ✅

3. **golang-migrate pgx/v5 driver** (`database/pgx/v5/pgx.go`):
   - Uses `database/sql` via `pgx/v5/stdlib` internally
   - Provides `WithInstance(*sql.DB, *Config)` for passing existing connections
   - Default migrations table: `schema_migrations` ✅
   - Advisory lock: handled internally by the driver ✅
   - `DefaultMigrationsTable = "schema_migrations"` confirmed in source

**Integration with existing pgxpool**:
```go
// Convert pool to *sql.DB using the stdlib bridge (already a transitive dep)
sqlDB := stdlib.OpenDBFromPool(pool)

driver, err := pgxv5.WithInstance(sqlDB, &pgxv5.Config{})
src, err    := iofs.New(migrationFiles, "schema")
m, err      := migrate.NewWithInstance("iofs", src, "pgx5", driver)
err          = m.Up() // migrate.ErrNoChange is not an error
```

**Why not a custom runner** (original decision revised):
- Custom runner was chosen because file rename was thought to break sqlc — INCORRECT
- sqlc officially and explicitly supports golang-migrate `.up.sql` format
- Using a proven library is better: advisory lock, tracking table, error handling all
  battle-tested; ~0 LOC of migration logic to maintain

**Alternatives considered**:
- `pressly/goose`: requires `-- +goose Up` directive in file body (modifying existing SQL)
- Custom runner: ~80 LOC, no new deps, but reinvents logic already in golang-migrate

**Sources**:
- sqlc docs (DDL / Handling SQL migrations): <https://docs.sqlc.dev/en/latest/howto/ddl.html>
- golang-migrate source/parse.go: <https://github.com/golang-migrate/migrate/blob/master/source/parse.go>
- golang-migrate database/pgx/v5/pgx.go: <https://github.com/golang-migrate/migrate/blob/master/database/pgx/v5/pgx.go>
- golang-migrate source/iofs/iofs.go: <https://github.com/golang-migrate/migrate/blob/master/source/iofs/iofs.go>

---

## Decision 2: Pool → sql.DB Bridge

**Decision**: `pgx/v5/stdlib.OpenDBFromPool(pool)` → pass to `golang-migrate`

**Rationale**: `pgx/v5/stdlib` is already an indirect dependency (pgx/v5 brings it in).
`WithInstance` accepts `*sql.DB`, so we bridge once at startup for migration only.
The main application continues using `pgxpool` directly.

**Sources**: golang-migrate pgx/v5 driver source confirms `WithInstance(*sql.DB, *Config)`

---

## Decision 3: embed.FS Directory

**Decision**: New file `internal/db/migrate/migrate.go` with:
```go
//go:embed ../schema/*.up.sql
var migrationFiles embed.FS
```

**Rationale**: After renaming to `.up.sql`, the embed glob picks up only up-migrations.
Follows existing project pattern (`pricing.go` uses `//go:embed`).

---

## Decision 4: File Rename Plan

10 files to rename in `internal/db/schema/`:

| Before | After |
|--------|-------|
| `001_initial.sql` | `001_initial.up.sql` |
| `002_management.sql` | `002_management.up.sql` |
| `003_organization.sql` | `003_organization.up.sql` |
| `004_credentials.sql` | `004_credentials.up.sql` |
| `005_phase3.sql` | `005_phase3.up.sql` |
| `006_mcp.sql` | `006_mcp.up.sql` |
| `007_audit.sql` | `007_audit.up.sql` |
| `008_skills_agents.sql` | `008_skills_agents.up.sql` |
| `009_extensions.sql` | `009_extensions.up.sql` |
| `010_org_membership.sql` | `010_org_membership.up.sql` |

`sqlc.yaml` schema path `"internal/db/schema/"` — **no change needed**.
sqlc reads the directory and natively handles `.up.sql` files.
