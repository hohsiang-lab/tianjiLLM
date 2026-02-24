# Data Model: Auto-Run DB Migrations

## schema_migrations

Tracks which migration files have been applied. Created automatically by the runner
on first startup (idempotent).

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER      PRIMARY KEY,
    name       TEXT         NOT NULL,
    applied_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
```

| Column | Type | Description |
|--------|------|-------------|
| `version` | INTEGER (PK) | Numeric prefix of migration file, e.g. `1` for `001_initial.sql` |
| `name` | TEXT | Full filename, e.g. `001_initial.sql` |
| `applied_at` | TIMESTAMPTZ | When the migration was applied (UTC) |

**Notes**:
- Managed exclusively by `internal/db/migrate` — not a sqlc-generated table
- No foreign keys (bootstrap infrastructure)
- No DOWN migration support in this iteration

## In-Memory Types (Go)

```go
// migration represents a single parsed migration file.
type migration struct {
    version int
    name    string // full filename e.g. "001_initial.sql"
    sql     string // file contents
}
```
