# Feature Specification: Auto-Run DB Migrations on Startup

**Feature Branch**: `002-auto-db-migration`
**Created**: 2026-02-25
**Status**: Draft
**Input**: User description: "Auto-run DB migrations on startup" (Issue #5)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Fresh Database Startup (Priority: P1)

An operator deploys TianjiLLM against a brand-new, empty PostgreSQL database.
On first boot, the service automatically applies all schema migrations in order
and becomes fully operational — no manual SQL steps required.

**Why this priority**: Eliminates the most common deployment friction point.
A fresh deploy should "just work" without manual DBA intervention.

**Independent Test**: Start the service with `DATABASE_URL` pointing to an
empty database. Verify the service starts successfully and the database contains
all expected tables and the migrations tracking table.

**Acceptance Scenarios**:

1. **Given** an empty PostgreSQL database, **When** the service starts,
   **Then** all migrations run in sequence (001 through latest) and the
   service is ready to handle requests.
2. **Given** an empty PostgreSQL database, **When** the service starts,
   **Then** a `schema_migrations` table records each applied migration with
   its version number and applied timestamp.
3. **Given** an empty PostgreSQL database, **When** the service starts,
   **Then** startup log includes a summary: "Applied N migrations successfully".

---

### User Story 2 - Incremental Upgrade (Priority: P1)

An operator upgrades TianjiLLM to a new version that adds new schema migrations.
On restart, only the new (unapplied) migrations run — existing data and schema
are untouched.

**Why this priority**: Equally critical to fresh install. Zero-downtime upgrades
require safe, incremental migration.

**Independent Test**: Start a service with migrations 001–005 applied. Upgrade
to a binary that knows about 006–010. Restart — verify only 006–010 are applied
and existing rows survive.

**Acceptance Scenarios**:

1. **Given** a database with migrations 001–005 applied, **When** a new binary
   with migrations 001–010 starts, **Then** only migrations 006–010 are executed.
2. **Given** a fully up-to-date database, **When** the service restarts,
   **Then** no migrations are executed and the service starts immediately.
3. **Given** an incremental upgrade, **When** migration succeeds,
   **Then** the startup log includes: "Applied 5 new migrations; schema is up-to-date".

---

### User Story 3 - Migration Failure Blocks Startup (Priority: P1)

A migration fails (e.g., conflicting schema, network error, permission denied).
The service refuses to start and logs a clear, actionable error message.

**Why this priority**: Fail-fast prevents a partially-migrated database from
serving corrupt or inconsistent data.

**Independent Test**: Introduce a deliberately broken migration SQL. Attempt
to start the service. Verify the process exits non-zero and logs which migration
failed with a human-readable reason.

**Acceptance Scenarios**:

1. **Given** a migration SQL with a syntax error, **When** the service starts,
   **Then** the service exits with a non-zero exit code and logs:
   "Migration 006_xxx failed: [reason]. Service will not start."
2. **Given** a failed migration, **When** the service exits,
   **Then** the database is left in a consistent state (no partial schema changes
   from the failed migration file).
3. **Given** a migration failure, **When** the operator inspects logs,
   **Then** the log clearly identifies the failing migration file name and
   the underlying database error.

---

### User Story 4 - No-Database Mode (Priority: P2)

TianjiLLM is started without a `DATABASE_URL` (config-file-only mode).
No migration is attempted; the service starts normally in stateless mode.

**Why this priority**: The service supports a no-DB deployment mode.
Migration logic must not block startup when no database is configured.

**Independent Test**: Start the service without `DATABASE_URL`. Verify the
service starts successfully with no migration-related log output.

**Acceptance Scenarios**:

1. **Given** no database URL is configured, **When** the service starts,
   **Then** no migration is attempted and the service starts in config-file mode.
2. **Given** no database URL, **When** the service starts,
   **Then** no migration-related errors appear in the startup log.

---

### Edge Cases

- What happens when the database is reachable but the target database does not exist?
  → Service exits with a clear "cannot connect to database" error before attempting migrations.
- What happens when two service instances start simultaneously against the same DB?
  → Migrations are executed under a distributed lock; only one instance runs migrations,
    the other waits and proceeds once migrations are complete.
- What happens when a migration file is removed from the binary after being applied?
  → Already-applied migrations are recorded by version number; removal of the file
    does not affect startup (no rollback is attempted).
- What happens when the `schema_migrations` tracking table itself is missing on a
  non-empty database (legacy upgrade)?
  → The tracking table is created first; all unapplied migrations are then identified
    by comparing file versions against the tracking table.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: On startup, the service MUST check whether the database connection
  is configured; if not, migration is skipped entirely.
- **FR-002**: On startup with a configured database, the service MUST execute all
  pending schema migrations in ascending version order before accepting any requests.
- **FR-003**: The service MUST track applied migrations in a `schema_migrations`
  table (version, applied_at) to prevent re-running already-applied migrations.
- **FR-004**: Migration MUST use a database-level distributed lock so that
  concurrent service instances do not execute migrations simultaneously.
- **FR-005**: If any migration fails, the service MUST exit immediately with a
  non-zero code and log the failing migration name and database error.
- **FR-006**: Individual migration files MUST be applied atomically so that
  a failure leaves no partial changes from that migration file.
- **FR-007**: The service MUST log a human-readable summary on startup:
  number of migrations applied, or "schema is up-to-date" if none were pending.
- **FR-008**: Migration source files are packaged with the service binary and
  require no external file system access at runtime.
- **FR-009**: Adding a new migration requires only placing a new sequentially
  numbered SQL file in the designated schema directory — no code changes needed.

### Key Entities

- **Migration File**: An ordered SQL file (e.g., `005_phase3.sql`) embedded
  in the binary. Identified by its numeric prefix as the version number.
- **schema_migrations table**: Persisted record of applied migrations.
  Fields: `version` (integer, PK), `name` (text), `applied_at` (timestamptz).
- **Migration Runner**: The startup component responsible for acquiring the
  advisory lock, comparing applied vs. available migrations, and executing
  pending ones in order.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A fresh deployment against an empty database becomes fully
  operational in under 30 seconds (including all migrations).
- **SC-002**: An incremental upgrade with new migrations completes startup
  in under 10 seconds per new migration file.
- **SC-003**: A migration failure causes the service to exit within 5 seconds
  with a non-zero exit code and a log message identifying the failing file.
- **SC-004**: Two simultaneous service instances starting against the same
  database apply each migration exactly once (no duplicate execution).
- **SC-005**: After a successful migration run, 100% of schema tables defined
  in the migration files are present in the database.

## Assumptions

- The database supports distributed locking primitives required for safe concurrent startup.
- Schema files in `internal/db/schema/` are already syntactically valid and
  ordered correctly (001 → 010 today); new files follow the same convention.
- No rollback (down migration) support is required in this iteration.
- The Python TianjiLLM reference does not have an equivalent migration-on-startup
  feature; this is a Go-specific operational improvement. Deviation is intentional
  and approved (Issue #5).
