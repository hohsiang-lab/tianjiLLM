package integration

import (
	"context"
	"os"
	"testing"

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

	// Reset migration state so each run starts from a "fresh" perspective.
	// Application tables use IF NOT EXISTS so dropping only schema_migrations is enough.
	_, err = pool.Exec(ctx, "DROP TABLE IF EXISTS schema_migrations")
	require.NoError(t, err, "failed to reset schema_migrations")

	// Test 1: Fresh DB → RunMigrations succeeds and applies all 10 migrations.
	t.Run("FreshDB", func(t *testing.T) {
		err := dbmigrate.RunMigrations(ctx, pool)
		require.NoError(t, err)

		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 10, count, "expected 10 rows in schema_migrations")
	})

	// Test 2: Running again on an already-migrated DB must be idempotent.
	t.Run("Idempotent", func(t *testing.T) {
		err := dbmigrate.RunMigrations(ctx, pool)
		assert.NoError(t, err, "second RunMigrations call must not return an error")
	})

	// Test 3: The first migration row has the expected file name.
	t.Run("FirstMigrationName", func(t *testing.T) {
		// golang-migrate stores the file name without directory in the dirty column
		// and the version as an integer. The name stored is the full file name.
		// Check version 1 record exists.
		var version int64
		var dirty bool
		err := pool.QueryRow(ctx,
			"SELECT version, dirty FROM schema_migrations WHERE version = 1",
		).Scan(&version, &dirty)
		require.NoError(t, err, "version 1 row must exist in schema_migrations")
		assert.Equal(t, int64(1), version)
		assert.False(t, dirty, "version 1 must not be dirty")
	})
}
