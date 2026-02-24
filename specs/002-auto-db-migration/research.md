# Research: Auto-Run DB Migrations on Startup

**Feature**: 002-auto-db-migration
**Date**: 2026-02-25

---

## Decision 1: Migration Library vs Custom Runner

**Decision**: Custom lightweight runner (~80 LOC) using `pgx/v5` directly

**Rationale**:
- `golang-migrate/v4`: requires files named `{version}_{title}.up.sql`. Our existing
  schema files use `{NNN}_{title}.sql` (no `.up.sql` suffix). Renaming all 10 files
  would also require updating `sqlc.yaml` schema references and risks introducing
  errors into the validated schema pipeline. Overhead not justified for a feature
  this bounded.
- `pressly/goose`: requires `-- +goose Up` directive comments in every migration file.
  Modifying existing, proven migration SQL is risky; goose also requires a
  `pgx/v5/stdlib` bridge (`sql.DB` adapter) whereas we use `pgxpool` natively.
- Custom runner: ~80 lines, no new dependencies, existing file naming preserved,
  native pgx/v5 pool usage, full control over advisory lock and transaction semantics.

**Alternatives Considered**:
- `golang-migrate/v4` + `source/iofs` + pgx/v5 driver: rejected due to file rename requirement
- `pressly/goose` v3: rejected due to directive comments requirement and stdlib bridge
- `jackc/tern`: minimal library, fewer stars, less community adoption

**Sources**:
- <https://pkg.go.dev/github.com/golang-migrate/migrate/v4/source/iofs> (confirmed `.up.sql` requirement)
- <https://stackoverflow.com/questions/76865674/how-to-use-goose-migrations-with-pgx> (goose + pgx bridge pattern)
- Constitution §III: "prefer well-maintained, widely-adopted Go libraries" — custom justified when library friction is high and feature is bounded

---

## Decision 2: Advisory Lock Key

**Decision**: Use PostgreSQL advisory lock with a fixed numeric key derived from
the application name hash. Specifically: `pg_advisory_lock(hashtext('tianjiLLM-migrations'))`
cast to `bigint`.

**Rationale**: Advisory lock key must be stable across all instances and releases.
Using a hash of a well-known string ensures uniqueness without manual coordination.
`pg_advisory_lock` (session-level) blocks until lock is acquired (vs `pg_try_advisory_lock`
which returns false immediately). Session-level lock is appropriate here: we want
the second instance to *wait* until the first finishes, not fail.

**Sources**:
- PostgreSQL docs on advisory locks
- Constitution §V: "Use context.Context for cancellation" → lock acquisition respects ctx timeout

---

## Decision 3: Tracking Table Schema

**Decision**:
```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version   INTEGER     PRIMARY KEY,
    name      TEXT        NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Rationale**: Minimal schema. `version` as integer PK enables fast lookup and
ordering. `name` for human-readable log output. `applied_at` for audit.
`CREATE TABLE IF NOT EXISTS` makes the runner idempotent (safe for legacy databases
with no prior migration history).

---

## Decision 4: File Naming / Versioning

**Decision**: Parse version number from filename prefix using regex `^(\d+)_`.
Files: `001_initial.sql`, `010_org_membership.sql` → versions 1, 10.

**Rationale**: Preserves existing naming convention. Sorted by parsed integer
(not lexicographic) to handle versions 001–099 correctly and beyond 100 without
zero-padding issues.

---

## Decision 5: embed.FS Pattern

**Decision**: New file `internal/db/migrate/migrate.go` with:
```go
//go:embed ../schema/*.sql
var migrations embed.FS
```

**Rationale**: Follows existing project pattern (pricing.go uses `//go:embed`,
assets uses `//go:embed all:css all:js`). Embed path is relative to the package file.
`embed.FS` is accessed via `fs.ReadDir` + `fs.ReadFile`.
