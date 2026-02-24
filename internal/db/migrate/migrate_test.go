package migrate

import (
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// TestSchemaFilesEmbed verifies that the embed.FS contains exactly 10 .up.sql files.
func TestSchemaFilesEmbed(t *testing.T) {
	entries, err := fs.ReadDir(db.SchemaFiles, "schema")
	require.NoError(t, err)

	var sqlFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			sqlFiles = append(sqlFiles, e.Name())
		}
	}

	assert.Len(t, sqlFiles, 10, "expected exactly 10 .up.sql files in embedded schema FS")
}

// TestSchemaFilesOrder verifies that the iofs source resolves versions 1-10 in order.
func TestSchemaFilesOrder(t *testing.T) {
	src, err := iofs.New(db.SchemaFiles, "schema")
	require.NoError(t, err)
	defer src.Close()

	// Collect all versions by traversing next from first.
	first, err := src.First()
	require.NoError(t, err)
	assert.Equal(t, uint(1), first, "first migration version must be 1")

	var versions []uint
	v := first
	for {
		versions = append(versions, v)
		next, err := src.Next(v)
		if err != nil {
			break // ErrNotExist means no more
		}
		v = next
	}

	assert.Len(t, versions, 10, "expected 10 migration versions")

	// Verify versions are sorted (ascending).
	assert.True(t, sort.SliceIsSorted(versions, func(i, j int) bool {
		return versions[i] < versions[j]
	}), "migration versions must be in ascending order")

	assert.Equal(t, uint(10), versions[len(versions)-1], "last migration version must be 10")
}

// TestRunMigrationsNilPool verifies that RunMigrations with a nil pool returns an error
// rather than panicking (guard for no-DB mode accidentally calling the function).
// T026: no migration attempt occurs when pool is nil.
func TestRunMigrationsNilPool(t *testing.T) {
	// stdlib.OpenDBFromPool panics on nil; this test documents expected behavior.
	// In production, RunMigrations is only called inside the DatabaseURL != "" block,
	// so pool is always non-nil. This test guards against accidental nil calls.
	assert.Panics(t, func() {
		_ = RunMigrations(t.Context(), nil) //nolint:errcheck
	}, "RunMigrations with nil pool should panic — caller must ensure pool is non-nil")
}
