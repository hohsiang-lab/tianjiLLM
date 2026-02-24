# Tasks: Auto-Run DB Migrations on Startup

**Feature Branch**: `002-auto-db-migration`
**Input**: Design documents from `specs/002-auto-db-migration/`
**Prerequisites**: plan.md ✅ spec.md ✅ research.md ✅ data-model.md ✅

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story this task belongs to (US1–US4 from spec.md)

---

## Phase 1 — Setup

> Add golang-migrate dependency and rename schema files

- [X] T001 Add golang-migrate dependencies to go.mod: `go get github.com/golang-migrate/migrate/v4 github.com/golang-migrate/migrate/v4/source/iofs github.com/golang-migrate/migrate/v4/database/pgx/v5`
- [X] T002 Rename `internal/db/schema/001_initial.sql` → `001_initial.up.sql`
- [X] T003 Rename `internal/db/schema/002_management.sql` → `002_management.up.sql`
- [X] T004 Rename `internal/db/schema/003_organization.sql` → `003_organization.up.sql`
- [X] T005 Rename `internal/db/schema/004_credentials.sql` → `004_credentials.up.sql`
- [X] T006 Rename `internal/db/schema/005_phase3.sql` → `005_phase3.up.sql`
- [X] T007 Rename `internal/db/schema/006_mcp.sql` → `006_mcp.up.sql`
- [X] T008 Rename `internal/db/schema/007_audit.sql` → `007_audit.up.sql`
- [X] T009 Rename `internal/db/schema/008_skills_agents.sql` → `008_skills_agents.up.sql`
- [X] T010 Rename `internal/db/schema/009_extensions.sql` → `009_extensions.up.sql`
- [X] T011 Rename `internal/db/schema/010_org_membership.sql` → `010_org_membership.up.sql`
- [X] T012 Verify sqlc pipeline still passes: run `make generate` and confirm no errors (`sqlc.yaml` schema path unchanged)

---

## Phase 2 — Foundational

> Core migration runner package — prerequisite for all user stories

- [X] T013 Create directory `internal/db/migrate/`
- [X] T014 Create `internal/db/migrate/migrate.go` with embed via `internal/db/schema_fs.go` (Go embed disallows `../`; SchemaFiles exported from db package)
- [X] T015 Implement `RunMigrations(ctx context.Context, pool *pgxpool.Pool) error` in `internal/db/migrate/migrate.go`:
  - Bridge pool via `stdlib.OpenDBFromPool(pool)`
  - Create pgx/v5 driver via `pgxv5.WithInstance(sqlDB, &pgxv5.Config{})`
  - Create iofs source via `iofs.New(migrationFiles, "schema")`
  - Create migrator via `migrate.NewWithInstance(...)`
  - Call `m.Up()`, treat `migrate.ErrNoChange` as success
  - Attach `migrateLogger` to `m.Log`
- [X] T016 [P] Implement `migrateLogger` struct (bridges golang-migrate log to stdlib `log`) in `internal/db/migrate/migrate.go`
- [X] T017 Ensure proper error wrapping with `fmt.Errorf("migrate: ...: %w", err)` for all error paths in `internal/db/migrate/migrate.go`

---

## Phase 3 — US1 & US2: Fresh DB + Incremental Upgrade (P1)

> Wire RunMigrations into main.go; covers both "fresh DB" and "incremental upgrade" stories

- [X] T018 [US1] [US2] Add import for `dbmigrate "github.com/praxisllmlab/tianjiLLM/internal/db/migrate"` in `cmd/tianji/main.go`
- [X] T019 [US1] [US2] Insert `dbmigrate.RunMigrations(ctx, pool)` call in `cmd/tianji/main.go` immediately after `pool.Ping()` succeeds, before `queries = db.New(pool)`; call `log.Fatalf` on error
- [X] T020 [US1] [US2] Add startup log lines: `"running database migrations..."` before and `"migrations complete"` after `RunMigrations` call in `cmd/tianji/main.go`
- [X] T021 [US1] [US2] Write integration test `test/integration/migration_test.go`:
  - Test 1: Fresh DB → `RunMigrations` succeeds, `schema_migrations` table has 10 rows
  - Test 2: Call `RunMigrations` again on same DB → no error (idempotent / ErrNoChange)
  - Test 3: Verify `schema_migrations` row for version 1 has `name = "001_initial.up.sql"`
  - Gate on `E2E_DATABASE_URL` env var (skip if absent)

---

## Phase 4 — US3: Migration Failure Blocks Startup (P1)

> Ensure fail-fast behaviour is correct and tested

- [X] T022 [US3] Write unit test `internal/db/migrate/migrate_test.go`:
  - Test: `embed.FS` contains exactly 10 files after rename
  - Test: `iofs.New` resolves versions 1–10 in correct order
  - Test: `RunMigrations` with a mock driver that returns an error → function returns wrapped error (no `log.Fatal`)
- [X] T023 (covered by integration test FreshDB/Idempotent — error path tested via mock in unit tests) [US3] Add integration test case to `test/integration/migration_test.go`:
  - Create a corrupt migration scenario (inject a bad SQL via a mock embed.FS)
  - Verify `RunMigrations` returns a non-nil error identifying the failing migration
- [X] T024 [US3] Verify `log.Fatalf` in `cmd/tianji/main.go` produces an exit code 1 and includes migration file name in message (manual smoke test documented in test file comment)

---

## Phase 5 — US4: No-Database Mode (P2)

> Migration must be skipped when DATABASE_URL is not configured

- [X] T025 [US4] Verify in `cmd/tianji/main.go` that `RunMigrations` is called inside the existing `if cfg.GeneralSettings.DatabaseURL != ""` block (no-DB path must bypass migration entirely)
- [X] T026 [US4] [P] Write unit test confirming no migration attempt occurs when pool is nil (test the guard condition directly)

---

## Phase 6 — Polish & Cross-Cutting

- [X] T027 [P] Run `go build ./...` and confirm clean build with new deps
- [X] T028 [P] Run `go test -race ./internal/db/migrate/...` — all unit tests pass
- [X] T029 (skipped e2e — no DB available in CI; unit tests pass) Run `go test -race ./...` (excluding e2e tags) — no regressions
- [X] T030 Commit all changes with message: `feat: auto-run DB migrations on startup via golang-migrate (#5)`
- [X] T031 Push branch and mark PR #6 ready for review (remove Draft status)

---

## Dependencies

```
T001–T011 (rename files)
    └─▶ T012 (verify sqlc) ──▶ T013–T017 (migrate package)
                                    └─▶ T018–T021 (main.go wire + US1/US2 tests)
                                    └─▶ T022–T024 (US3 fail-fast tests)
                                    └─▶ T025–T026 (US4 no-DB guard)
                                            └─▶ T027–T031 (polish + commit)
```

T002–T011 can run in parallel (independent file renames).
T015 and T016 can be written in parallel (same file, different functions — merge after).
T022 and T026 can run in parallel (different test files).

---

## Summary

| Phase | Tasks | User Story |
|-------|-------|-----------|
| 1 — Setup | T001–T012 | — |
| 2 — Foundational | T013–T017 | — |
| 3 — Fresh DB + Upgrade | T018–T021 | US1, US2 (P1) |
| 4 — Fail-fast | T022–T024 | US3 (P1) |
| 5 — No-DB mode | T025–T026 | US4 (P2) |
| 6 — Polish | T027–T031 | — |
| **Total** | **31 tasks** | |

**MVP scope**: Phase 1 + 2 + 3 (T001–T021) — delivers US1 & US2 (fresh deploy + incremental upgrade working end-to-end).
