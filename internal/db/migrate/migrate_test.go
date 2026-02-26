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

// TestSchemaFilesEmbed verifies that the embed.FS contains exactly 11 .up.sql files.
func TestSchemaFilesEmbed(t *testing.T) {
	entries, err := fs.ReadDir(db.SchemaFiles, "schema")
	require.NoError(t, err)

	var sqlFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			sqlFiles = append(sqlFiles, e.Name())
		}
	}

	assert.Len(t, sqlFiles, 11, "expected exactly 11 .up.sql files in embedded schema FS")
}

// TestSchemaFilesOrder verifies that the iofs source resolves versions 1-11 in order.
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

	assert.Len(t, versions, 11, "expected 11 migration versions")

	// Verify versions are sorted (ascending).
	assert.True(t, sort.SliceIsSorted(versions, func(i, j int) bool {
		return versions[i] < versions[j]
	}), "migration versions must be in ascending order")

	assert.Equal(t, uint(11), versions[len(versions)-1], "last migration version must be 11")
}

// TestRunMigrationsNilPool verifies that RunMigrations with a nil pool returns a
// descriptive error (not a panic), containing enough context for an LLM or developer
// to immediately locate the root cause without reading source code.
//
// T026: no migration attempt occurs when pool is nil.
// UT-01: error message must contain identifiable keywords.
func TestRunMigrationsNilPool(t *testing.T) {
	err := RunMigrations(t.Context(), nil)

	require.Error(t, err, "RunMigrations(nil) must return an error")

	// Error message must contain keywords that let an LLM/developer instantly identify:
	// 1. which function failed  → "RunMigrations"
	// 2. what was wrong        → "nil pool"
	// 3. what to do about it   → "DATABASE_URL"
	assert.Contains(t, err.Error(), "RunMigrations", "error must name the failing function")
	assert.Contains(t, err.Error(), "nil pool", "error must describe the bad argument")
	assert.Contains(t, err.Error(), "DATABASE_URL", "error must hint at the fix")
}
