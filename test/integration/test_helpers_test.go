package integration

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// testDSN returns the database connection string for integration tests.
// Set TEST_DATABASE_URL to override. Default: docker-compose postgres on port 5433.
func testDSN() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://tianji:***@localhost:5433/tianji?sslmode=disable"
}

func setupTestDB(t *testing.T) *db.Queries {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, testDSN())
	require.NoError(t, err, "failed to connect to test database — is docker-compose postgres running?")
	t.Cleanup(func() { pool.Close() })

	return db.New(pool)
}

// getPool returns a test database pool for integration fixtures.
func getPool(t *testing.T, _ *db.Queries) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN())
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}
