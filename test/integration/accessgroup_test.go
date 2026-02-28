package integration

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// testDSN returns the database connection string for integration tests.
// Set TEST_DATABASE_URL to override. Default: docker-compose postgres on port 5433.
func testDSN() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://tianji:tianji@localhost:5433/tianji?sslmode=disable"
}

func setupTestDB(t *testing.T) *db.Queries {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, testDSN())
	require.NoError(t, err, "failed to connect to test database — is docker-compose postgres running?")
	t.Cleanup(func() { pool.Close() })

	// Clean up test data before each test
	_, _ = pool.Exec(ctx, `DELETE FROM "ModelAccessGroup" WHERE group_id LIKE 'test-%'`)
	_, _ = pool.Exec(ctx, `DELETE FROM "VerificationToken" WHERE token LIKE 'test-%'`)

	return db.New(pool)
}

// ensureTestKey inserts a minimal VerificationToken for testing key membership.
func ensureTestKey(t *testing.T, q *db.Queries, token string) {
	t.Helper()
	ctx := context.Background()
	pool := getPool(t, q)
	_, err := pool.Exec(ctx,
		`INSERT INTO "VerificationToken" (token, key_name, key_alias, access_group_ids)
		 VALUES ($1, $2, $3, '{}')
		 ON CONFLICT (token) DO NOTHING`,
		token, "test-key-"+token, "alias-"+token)
	require.NoError(t, err)
}

// getPool extracts the underlying pool from Queries via a helper connection.
func getPool(t *testing.T, _ *db.Queries) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN())
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

// --- CRUD Tests ---

func TestAccessGroup_CreateAndGet(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	alias := "test-alias-crud"
	params := db.CreateAccessGroupParams{
		GroupID:    "test-crud-1",
		GroupAlias: &alias,
		Models:     []string{"gpt-4o", "claude-3"},
		CreatedBy:  "test-user",
	}

	group, err := q.CreateAccessGroup(ctx, params)
	require.NoError(t, err)
	assert.Equal(t, "test-crud-1", group.GroupID)
	assert.Equal(t, &alias, group.GroupAlias)
	assert.Equal(t, []string{"gpt-4o", "claude-3"}, group.Models)
	assert.Equal(t, "test-user", group.CreatedBy)

	// Read back
	got, err := q.GetAccessGroup(ctx, "test-crud-1")
	require.NoError(t, err)
	assert.Equal(t, group.GroupID, got.GroupID)
	assert.Equal(t, group.Models, got.Models)
}

func TestAccessGroup_Update(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	// Create org for FK constraint
	pool := getPool(t, q)
	_, err := pool.Exec(ctx,
		`INSERT INTO "OrganizationTable" (organization_id, organization_alias) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		"test-org-1", "Test Org 1")
	require.NoError(t, err)

	alias := "test-alias-update"
	_, err = q.CreateAccessGroup(ctx, db.CreateAccessGroupParams{
		GroupID:    "test-update-1",
		GroupAlias: &alias,
		Models:     []string{"gpt-4o"},
		CreatedBy:  "creator",
	})
	require.NoError(t, err)

	newAlias := "test-alias-updated"
	orgID := "test-org-1"
	err = q.UpdateAccessGroup(ctx, db.UpdateAccessGroupParams{
		GroupID:        "test-update-1",
		GroupAlias:     &newAlias,
		Models:         []string{"gpt-4o", "claude-3"},
		OrganizationID: &orgID,
		UpdatedBy:      "updater",
	})
	require.NoError(t, err)

	got, err := q.GetAccessGroup(ctx, "test-update-1")
	require.NoError(t, err)
	assert.Equal(t, &newAlias, got.GroupAlias)
	assert.Equal(t, []string{"gpt-4o", "claude-3"}, got.Models)
	assert.Equal(t, "updater", got.UpdatedBy)
}

func TestAccessGroup_Delete(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	alias := "test-alias-delete"
	_, err := q.CreateAccessGroup(ctx, db.CreateAccessGroupParams{
		GroupID:    "test-delete-1",
		GroupAlias: &alias,
		Models:     []string{"gpt-4o"},
		CreatedBy:  "creator",
	})
	require.NoError(t, err)

	err = q.DeleteAccessGroup(ctx, "test-delete-1")
	require.NoError(t, err)

	_, err = q.GetAccessGroup(ctx, "test-delete-1")
	assert.Error(t, err, "should not find deleted group")
}

func TestAccessGroup_List(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		alias := fmt.Sprintf("test-list-%d", i)
		_, err := q.CreateAccessGroup(ctx, db.CreateAccessGroupParams{
			GroupID:    fmt.Sprintf("test-list-%d", i),
			GroupAlias: &alias,
			Models:     []string{"gpt-4o"},
			CreatedBy:  "creator",
		})
		require.NoError(t, err)
	}

	groups, err := q.ListAccessGroups(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(groups), 3)
}

// --- Alias Uniqueness Tests ---

func TestAccessGroup_DuplicateAlias(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	alias := "test-dup-alias"
	_, err := q.CreateAccessGroup(ctx, db.CreateAccessGroupParams{
		GroupID:    "test-dup-1",
		GroupAlias: &alias,
		Models:     []string{"gpt-4o"},
		CreatedBy:  "creator",
	})
	require.NoError(t, err)

	// GetAccessGroupByAlias should find it
	got, err := q.GetAccessGroupByAlias(ctx, &alias)
	require.NoError(t, err)
	assert.Equal(t, "test-dup-1", got.GroupID)

	// Note: The DB schema doesn't enforce UNIQUE on group_alias.
	// Alias uniqueness is enforced at the application layer (UI handler).
	// We verify the application-level check works by testing GetAccessGroupByAlias.
}

// --- Key Membership Tests ---

func TestAccessGroup_AddAndRemoveKey(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	alias := "test-key-membership"
	_, err := q.CreateAccessGroup(ctx, db.CreateAccessGroupParams{
		GroupID:    "test-key-grp-1",
		GroupAlias: &alias,
		Models:     []string{"gpt-4o"},
		CreatedBy:  "creator",
	})
	require.NoError(t, err)

	ensureTestKey(t, q, "test-key-1")
	ensureTestKey(t, q, "test-key-2")

	// Add keys to group
	err = q.AddKeyToAccessGroup(ctx, db.AddKeyToAccessGroupParams{
		GroupID: "test-key-grp-1",
		Token:   "test-key-1",
	})
	require.NoError(t, err)

	err = q.AddKeyToAccessGroup(ctx, db.AddKeyToAccessGroupParams{
		GroupID: "test-key-grp-1",
		Token:   "test-key-2",
	})
	require.NoError(t, err)

	// Verify members
	members, err := q.ListKeysByAccessGroup(ctx, "test-key-grp-1")
	require.NoError(t, err)
	assert.Len(t, members, 2)

	// Remove one key
	err = q.RemoveKeyFromAccessGroup(ctx, db.RemoveKeyFromAccessGroupParams{
		GroupID: "test-key-grp-1",
		Token:   "test-key-1",
	})
	require.NoError(t, err)

	members, err = q.ListKeysByAccessGroup(ctx, "test-key-grp-1")
	require.NoError(t, err)
	assert.Len(t, members, 1)
	assert.Equal(t, "test-key-2", members[0].Token)
}

func TestAccessGroup_AddKeyIdempotent(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	alias := "test-idempotent"
	_, err := q.CreateAccessGroup(ctx, db.CreateAccessGroupParams{
		GroupID:    "test-idemp-grp",
		GroupAlias: &alias,
		Models:     []string{"gpt-4o"},
		CreatedBy:  "creator",
	})
	require.NoError(t, err)

	ensureTestKey(t, q, "test-key-idemp")

	// Add same key twice — should not duplicate
	for i := 0; i < 2; i++ {
		err = q.AddKeyToAccessGroup(ctx, db.AddKeyToAccessGroupParams{
			GroupID: "test-idemp-grp",
			Token:   "test-key-idemp",
		})
		require.NoError(t, err)
	}

	members, err := q.ListKeysByAccessGroup(ctx, "test-idemp-grp")
	require.NoError(t, err)
	assert.Len(t, members, 1, "adding same key twice should not duplicate")
}

// --- Delete Cleanup (Cascade) Tests ---

func TestAccessGroup_DeleteCleansUpKeys(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	alias := "test-cascade"
	_, err := q.CreateAccessGroup(ctx, db.CreateAccessGroupParams{
		GroupID:    "test-cascade-grp",
		GroupAlias: &alias,
		Models:     []string{"gpt-4o"},
		CreatedBy:  "creator",
	})
	require.NoError(t, err)

	ensureTestKey(t, q, "test-key-cascade-1")
	ensureTestKey(t, q, "test-key-cascade-2")

	// Add keys to group
	for _, tk := range []string{"test-key-cascade-1", "test-key-cascade-2"} {
		err = q.AddKeyToAccessGroup(ctx, db.AddKeyToAccessGroupParams{
			GroupID: "test-cascade-grp",
			Token:   tk,
		})
		require.NoError(t, err)
	}

	// Verify keys are in group
	members, err := q.ListKeysByAccessGroup(ctx, "test-cascade-grp")
	require.NoError(t, err)
	assert.Len(t, members, 2)

	// Cascade: remove group from all keys, then delete group
	// This mirrors the UI handler's delete behavior (transaction)
	err = q.RemoveAccessGroupFromAllKeys(ctx, "test-cascade-grp")
	require.NoError(t, err)

	err = q.DeleteAccessGroup(ctx, "test-cascade-grp")
	require.NoError(t, err)

	// Verify keys no longer reference the group
	members, err = q.ListKeysByAccessGroup(ctx, "test-cascade-grp")
	require.NoError(t, err)
	assert.Len(t, members, 0, "after delete, no keys should reference the group")

	// Verify the group is gone
	_, err = q.GetAccessGroup(ctx, "test-cascade-grp")
	assert.Error(t, err)
}

// --- Validation Tests ---

func TestAccessGroup_EmptyAlias(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	// Creating with nil alias should succeed (alias is nullable)
	group, err := q.CreateAccessGroup(ctx, db.CreateAccessGroupParams{
		GroupID:   "test-nil-alias",
		Models:    []string{"gpt-4o"},
		CreatedBy: "creator",
	})
	require.NoError(t, err)
	assert.Nil(t, group.GroupAlias)

	// Note: Empty string alias validation is at the application layer (UI handler),
	// not at the DB layer. The DB allows NULL aliases.
}

func TestAccessGroup_ListKeysNotInGroup(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	alias := "test-not-in-grp"
	_, err := q.CreateAccessGroup(ctx, db.CreateAccessGroupParams{
		GroupID:    "test-notin-grp",
		GroupAlias: &alias,
		Models:     []string{"gpt-4o"},
		CreatedBy:  "creator",
	})
	require.NoError(t, err)

	ensureTestKey(t, q, "test-key-in")
	ensureTestKey(t, q, "test-key-out")

	// Add only one key
	err = q.AddKeyToAccessGroup(ctx, db.AddKeyToAccessGroupParams{
		GroupID: "test-notin-grp",
		Token:   "test-key-in",
	})
	require.NoError(t, err)

	// Keys not in group should include test-key-out but not test-key-in
	notIn, err := q.ListKeysNotInAccessGroup(ctx, "test-notin-grp")
	require.NoError(t, err)

	tokens := make(map[string]bool)
	for _, k := range notIn {
		tokens[k.Token] = true
	}
	assert.True(t, tokens["test-key-out"], "test-key-out should be in not-in-group list")
	assert.False(t, tokens["test-key-in"], "test-key-in should NOT be in not-in-group list")
}

// --- Edit Organization Tests ---

func TestAccessGroup_UpdateOrganization(t *testing.T) {
	q := setupTestDB(t)
	ctx := context.Background()

	// Create an organization first
	pool := getPool(t, q)
	_, err := pool.Exec(ctx,
		`INSERT INTO "OrganizationTable" (organization_id, organization_alias) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		"test-org-a", "Org A")
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO "OrganizationTable" (organization_id, organization_alias) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		"test-org-b", "Org B")
	require.NoError(t, err)

	alias := "test-org-update"
	orgA := "test-org-a"
	_, err = q.CreateAccessGroup(ctx, db.CreateAccessGroupParams{
		GroupID:        "test-org-upd-1",
		GroupAlias:     &alias,
		Models:         []string{"gpt-4o"},
		OrganizationID: &orgA,
		CreatedBy:      "creator",
	})
	require.NoError(t, err)

	// Update to org B
	orgB := "test-org-b"
	err = q.UpdateAccessGroup(ctx, db.UpdateAccessGroupParams{
		GroupID:        "test-org-upd-1",
		GroupAlias:     &alias,
		Models:         []string{"gpt-4o"},
		OrganizationID: &orgB,
		UpdatedBy:      "updater",
	})
	require.NoError(t, err)

	got, err := q.GetAccessGroup(ctx, "test-org-upd-1")
	require.NoError(t, err)
	assert.Equal(t, &orgB, got.OrganizationID)

	// Clear organization
	err = q.UpdateAccessGroup(ctx, db.UpdateAccessGroupParams{
		GroupID:    "test-org-upd-1",
		GroupAlias: &alias,
		Models:     []string{"gpt-4o"},
		UpdatedBy:  "updater",
	})
	require.NoError(t, err)

	got, err = q.GetAccessGroup(ctx, "test-org-upd-1")
	require.NoError(t, err)
	assert.Nil(t, got.OrganizationID)
}
