package integration

import (
	"context"
	"os"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbmigrate "github.com/praxisllmlab/tianjiLLM/internal/db/migrate"
)

// TestRunMigrations requires a real PostgreSQL instance.
// Set E2E_DATABASE_URL to run (e.g. postgres://tianji:tianji@localhost:5433/tianji_e2e).
func TestRunMigrations(t *testing.T) {
	dbURL := os.Getenv("E2E_DATABASE_URL")
	if dbURL == "" {
		t.Skip("E2E_DATABASE_URL not set; skipping migration integration tests")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()
	require.NoError(t, pool.Ping(ctx))

	// resetMigrations drops the golang-migrate tracking table so the next call to
	// RunMigrations starts from a clean slate. Business tables use IF NOT EXISTS
	// so they are unaffected.
	resetMigrations := func(t *testing.T) {
		t.Helper()
		_, err := pool.Exec(ctx, "DROP TABLE IF EXISTS schema_migrations")
		require.NoError(t, err, "failed to reset schema_migrations")
	}

	// ── Existing tests ────────────────────────────────────────────────────────

	// Test 1: Fresh DB → RunMigrations succeeds and applies all 10 migrations.
	t.Run("FreshDB", func(t *testing.T) {
		resetMigrations(t)

		err := dbmigrate.RunMigrations(ctx, pool)
		require.NoError(t, err)

		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count))
		assert.Equal(t, 10, count, "expected 10 rows in schema_migrations after fresh apply")
	})

	// Test 2: Running again on an already-migrated DB must be idempotent.
	t.Run("Idempotent", func(t *testing.T) {
		// Depends on FreshDB having run first (no reset).
		err := dbmigrate.RunMigrations(ctx, pool)
		assert.NoError(t, err, "second RunMigrations call must not return an error")

		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count))
		assert.Equal(t, 10, count, "row count must remain 10 after idempotent run")
	})

	// Test 3: The first migration row records version=1, dirty=false.
	t.Run("FirstMigrationName", func(t *testing.T) {
		var version int64
		var dirty bool
		err := pool.QueryRow(ctx,
			"SELECT version, dirty FROM schema_migrations WHERE version = 1",
		).Scan(&version, &dirty)
		require.NoError(t, err, "version 1 row must exist in schema_migrations")
		assert.Equal(t, int64(1), version)
		assert.False(t, dirty, "version 1 must not be dirty")
	})

	// ── IT-01: IncrementalUpgrade ─────────────────────────────────────────────

	// IT-01: A DB where only versions 1-5 were applied receives only versions 6-10
	// on the next RunMigrations call. Existing data and schema are untouched.
	t.Run("IT01_IncrementalUpgrade", func(t *testing.T) {
		resetMigrations(t)

		// Simulate a DB that has had migrations 1-5 applied by a previous binary.
		// golang-migrate tracks applied versions in schema_migrations(version, dirty).
		// Business tables already exist (IF NOT EXISTS in SQL files) so inserting
		// fake rows is safe — the runner will only execute versions not in this table.
		for v := 1; v <= 5; v++ {
			_, err := pool.Exec(ctx,
				"INSERT INTO schema_migrations (version, dirty) VALUES ($1, false)", v)
			require.NoError(t, err, "failed to seed schema_migrations for version %d", v)
		}

		// Confirm only 5 rows exist before upgrade.
		var before int
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&before))
		require.Equal(t, 5, before, "precondition: 5 rows before upgrade")

		// Run the full migration suite — should apply 6-10 only.
		err := dbmigrate.RunMigrations(ctx, pool)
		require.NoError(t, err, "RunMigrations must succeed on incremental upgrade")

		var after int
		require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&after))
		assert.Equal(t, 10, after, "expected 10 rows after incremental upgrade (5 existing + 5 new)")

		// Versions 1-5 must still be there (not replaced).
		var countEarly int
		require.NoError(t, pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM schema_migrations WHERE version <= 5").Scan(&countEarly))
		assert.Equal(t, 5, countEarly, "versions 1-5 must be preserved after incremental upgrade")
	})

	// ── IT-02: MigrationFailFast ──────────────────────────────────────────────

	// IT-02: A migration SQL file with a syntax error causes RunMigrations to
	// return a non-nil error identifying the failing migration. The DB is left
	// in a consistent state (golang-migrate wraps failing migrations in a tx).
	t.Run("IT02_MigrationFailFast", func(t *testing.T) {
		resetMigrations(t)

		// Build an in-memory FS with one valid and one broken migration.
		brokenFS := fstest.MapFS{
			"001_good.up.sql": {
				Data: []byte("CREATE TABLE IF NOT EXISTS _it02_test (id serial PRIMARY KEY);"),
			},
			"002_broken.up.sql": {
				Data: []byte("CREATA TABEL _this_is_invalid_syntax (id serial);"),
			},
		}

		err := dbmigrate.RunMigrationsFromFS(ctx, pool, brokenFS, ".")
		require.Error(t, err, "RunMigrations must return an error when migration SQL is invalid")

		// Error message must identify the failure context.
		assert.Contains(t, err.Error(), "migrate:", "error must be wrapped with migrate: prefix")

		// Version 1 (good) was applied; version 2 (broken) should be marked dirty.
		var dirty bool
		dbErr := pool.QueryRow(ctx,
			"SELECT dirty FROM schema_migrations WHERE version = 2",
		).Scan(&dirty)
		// golang-migrate marks the failing version as dirty=true in schema_migrations.
		if dbErr == nil {
			assert.True(t, dirty, "failing migration version must be marked dirty=true")
		}
		// If the row doesn't exist at all, the transaction was fully rolled back —
		// also acceptable behaviour.

		// Cleanup: drop test table if it was created.
		_, _ = pool.Exec(ctx, "DROP TABLE IF EXISTS _it02_test")
	})

	// ── IT-03: DirtyStateRecovery ─────────────────────────────────────────────

	// IT-03: When schema_migrations contains a dirty=true row from a previously
	// failed migration, RunMigrations returns an error (golang-migrate ErrDirty)
	// instead of silently continuing. Operators must manually resolve dirty state.
	t.Run("IT03_DirtyStateRecovery", func(t *testing.T) {
		resetMigrations(t)

		// Manually force version 3 into a dirty state, as if a previous migration
		// run crashed mid-execution.
		_, err := pool.Exec(ctx,
			"INSERT INTO schema_migrations (version, dirty) VALUES (3, true)")
		require.NoError(t, err, "failed to seed dirty migration state")

		err = dbmigrate.RunMigrations(ctx, pool)
		require.Error(t, err, "RunMigrations must return an error when schema has a dirty version")

		// The error must indicate a dirty migration — look for "dirty" in the message.
		// golang-migrate wraps this as "migrate: <N> is dirty" or similar.
		assert.Contains(t, err.Error(), "dirty",
			"error message must mention 'dirty' so operators know to resolve it manually")

		// Confirm RunMigrations did NOT auto-fix dirty state (operator must intervene).
		var dirty bool
		require.NoError(t, pool.QueryRow(ctx,
			"SELECT dirty FROM schema_migrations WHERE version = 3",
		).Scan(&dirty))
		assert.True(t, dirty, "dirty flag must remain true — RunMigrations must not auto-repair dirty state")
	})
}
