//go:build integration

package contract

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

func newTestDBPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("E2E_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://tianji:tianji@localhost:5433/tianji_e2e"
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Skipf("cannot connect to test DB (%s): %v", dbURL, err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB not reachable (%s): %v", dbURL, err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestListTeamsByOrganization verifies that the query returns teams for the
// given org and returns an empty slice (not an error) when no teams exist.
func TestListTeamsByOrganization(t *testing.T) {
	pool := newTestDBPool(t)
	ctx := context.Background()
	q := db.New(pool)

	// Seed an organization and two teams that belong to it.
	orgID := fmt.Sprintf("test-org-%d", time.Now().UnixNano())
	_, err := pool.Exec(ctx,
		`INSERT INTO "OrganizationTable" (organization_id, organization_alias) VALUES ($1, $2)`,
		orgID, "Test Org for ListTeamsByOrganization",
	)
	require.NoError(t, err)

	team1ID := fmt.Sprintf("test-team1-%d", time.Now().UnixNano())
	team2ID := fmt.Sprintf("test-team2-%d", time.Now().UnixNano())
	_, err = pool.Exec(ctx,
		`INSERT INTO "TeamTable" (team_id, team_alias, organization_id, spend) VALUES ($1, $2, $3, 0), ($4, $5, $6, 0)`,
		team1ID, "Team Alpha", orgID, team2ID, "Team Beta", orgID,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM "TeamTable" WHERE organization_id = $1`, orgID)
		_, _ = pool.Exec(ctx, `DELETE FROM "OrganizationTable" WHERE organization_id = $1`, orgID)
	})

	t.Run("returns teams for org", func(t *testing.T) {
		teams, err := q.ListTeamsByOrganization(ctx, &orgID)
		require.NoError(t, err)
		assert.Len(t, teams, 2, "expected 2 teams for the org")
	})

	t.Run("empty slice for org with no teams", func(t *testing.T) {
		emptyOrgID := fmt.Sprintf("empty-org-%d", time.Now().UnixNano())
		_, err := pool.Exec(ctx,
			`INSERT INTO "OrganizationTable" (organization_id, organization_alias) VALUES ($1, $2)`,
			emptyOrgID, "Empty Org",
		)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM "OrganizationTable" WHERE organization_id = $1`, emptyOrgID)
		})

		teams, err := q.ListTeamsByOrganization(ctx, &emptyOrgID)
		require.NoError(t, err)
		assert.Empty(t, teams, "expected empty slice for org with no teams")
	})
}

// TestCountTeamsPerOrganization verifies that the query returns a row for each
// org that has teams, and returns zero rows for an org with no teams.
func TestCountTeamsPerOrganization(t *testing.T) {
	pool := newTestDBPool(t)
	ctx := context.Background()
	q := db.New(pool)

	orgID := fmt.Sprintf("cnt-org-%d", time.Now().UnixNano())
	_, err := pool.Exec(ctx,
		`INSERT INTO "OrganizationTable" (organization_id, organization_alias) VALUES ($1, $2)`,
		orgID, "Count Test Org",
	)
	require.NoError(t, err)

	teamID := fmt.Sprintf("cnt-team-%d", time.Now().UnixNano())
	_, err = pool.Exec(ctx,
		`INSERT INTO "TeamTable" (team_id, team_alias, organization_id, spend) VALUES ($1, $2, $3, 0)`,
		teamID, "Count Team", orgID,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM "TeamTable" WHERE team_id = $1`, teamID)
		_, _ = pool.Exec(ctx, `DELETE FROM "OrganizationTable" WHERE organization_id = $1`, orgID)
	})

	t.Run("org with teams has count > 0", func(t *testing.T) {
		rows, err := q.CountTeamsPerOrganization(ctx)
		require.NoError(t, err)

		found := false
		for _, row := range rows {
			if row.OrganizationID != nil && *row.OrganizationID == orgID {
				assert.Greater(t, row.TeamCount, int64(0))
				found = true
			}
		}
		assert.True(t, found, "expected a count row for the seeded org")
	})

	t.Run("org with no teams absent from results", func(t *testing.T) {
		emptyOrgID := fmt.Sprintf("no-team-org-%d", time.Now().UnixNano())
		_, err := pool.Exec(ctx,
			`INSERT INTO "OrganizationTable" (organization_id, organization_alias) VALUES ($1, $2)`,
			emptyOrgID, "No Team Org",
		)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM "OrganizationTable" WHERE organization_id = $1`, emptyOrgID)
		})

		rows, err := q.CountTeamsPerOrganization(ctx)
		require.NoError(t, err)

		for _, row := range rows {
			if row.OrganizationID != nil {
				assert.NotEqual(t, emptyOrgID, *row.OrganizationID,
					"org with no teams must not appear in count results")
			}
		}
	})
}

// TestCountMembersPerOrganization verifies membership counts.
func TestCountMembersPerOrganization(t *testing.T) {
	pool := newTestDBPool(t)
	ctx := context.Background()
	q := db.New(pool)

	orgID := fmt.Sprintf("mem-org-%d", time.Now().UnixNano())
	_, err := pool.Exec(ctx,
		`INSERT INTO "OrganizationTable" (organization_id, organization_alias) VALUES ($1, $2)`,
		orgID, "Member Count Test Org",
	)
	require.NoError(t, err)

	userID := fmt.Sprintf("mem-user-%d", time.Now().UnixNano())
	_, err = pool.Exec(ctx,
		`INSERT INTO "OrganizationMembership" (user_id, organization_id, user_role) VALUES ($1, $2, $3)`,
		userID, orgID, "member",
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM "OrganizationMembership" WHERE organization_id = $1`, orgID)
		_, _ = pool.Exec(ctx, `DELETE FROM "OrganizationTable" WHERE organization_id = $1`, orgID)
	})

	t.Run("org with members has count > 0", func(t *testing.T) {
		rows, err := q.CountMembersPerOrganization(ctx)
		require.NoError(t, err)

		found := false
		for _, row := range rows {
			if row.OrganizationID == orgID {
				assert.Greater(t, row.MemberCount, int64(0))
				found = true
			}
		}
		assert.True(t, found, "expected a count row for the seeded org")
	})

	t.Run("org with no members absent from results", func(t *testing.T) {
		emptyOrgID := fmt.Sprintf("no-mem-org-%d", time.Now().UnixNano())
		_, err := pool.Exec(ctx,
			`INSERT INTO "OrganizationTable" (organization_id, organization_alias) VALUES ($1, $2)`,
			emptyOrgID, "No Member Org",
		)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM "OrganizationTable" WHERE organization_id = $1`, emptyOrgID)
		})

		rows, err := q.CountMembersPerOrganization(ctx)
		require.NoError(t, err)

		for _, row := range rows {
			assert.NotEqual(t, emptyOrgID, row.OrganizationID,
				"org with no members must not appear in count results")
		}
	})
}
