//go:build e2e

package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTeamsList_EmptyState(t *testing.T) {
	f := setup(t)
	f.NavigateToTeams()

	assert.Contains(t, f.Text("#teams-table"), "No teams found")
}

func TestTeamsList_ShowsTeams(t *testing.T) {
	f := setup(t)
	f.SeedTeams(3)
	f.NavigateToTeams()

	rows := f.Count("#teams-table table tbody tr")
	assert.Equal(t, 3, rows)

	body := f.Text("#teams-table")
	assert.Contains(t, body, "test-team-1")
	assert.Contains(t, body, "test-team-2")
	assert.Contains(t, body, "test-team-3")
}

func TestTeamsList_ColumnsComplete(t *testing.T) {
	f := setup(t)
	budget := 100.0
	orgID := f.SeedOrg(SeedOrgOpts{Alias: "acme-corp"})
	f.SeedTeam(SeedTeamOpts{
		Alias:     "col-test-team",
		OrgID:     orgID,
		Members:   []string{"user-a", "user-b"},
		Spend:     25.50,
		MaxBudget: &budget,
		Models:    []string{"gpt-4o", "claude-3"},
	})
	f.NavigateToTeams()

	body := f.Text("#teams-table")
	// Alias
	assert.Contains(t, body, "col-test-team")
	// Organization
	assert.Contains(t, body, "acme-corp")
	// Member count
	assert.Contains(t, body, "2")
	// Spend / Budget
	assert.Contains(t, body, "$25.50")
	assert.Contains(t, body, "$100.00")
	// Model count
	assert.Contains(t, body, "2")
	// Status
	assert.Contains(t, body, "Active")
}

func TestTeamsList_FilterByAlias(t *testing.T) {
	f := setup(t)
	f.SeedTeams(5)
	f.NavigateToTeams()

	f.FilterTeams("search", "test-team-3")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	rows := f.Count("#teams-table table tbody tr")
	assert.Equal(t, 1, rows)
	assert.Contains(t, f.Text("#teams-table"), "test-team-3")
}

func TestTeamsList_FilterNoMatch(t *testing.T) {
	f := setup(t)
	f.SeedTeams(3)
	f.NavigateToTeams()

	f.FilterTeams("search", "nonexistent-team")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	assert.Contains(t, f.Text("#teams-table"), "No teams")
}

func TestTeamsList_Pagination(t *testing.T) {
	f := setup(t)
	f.SeedTeams(55)
	f.NavigateToTeams()

	// Page 1 should show 50 rows
	rows := f.Count("#teams-table table tbody tr")
	assert.Equal(t, 50, rows)

	// Navigate to page 2
	f.Page.Locator(`[hx-get*="page=2"]`).First().Click()
	f.WaitStable()

	// Page 2 should show 5 rows
	rows = f.Count("#teams-table table tbody tr")
	assert.Equal(t, 5, rows)
}

func TestTeamsList_StatusBadges(t *testing.T) {
	f := setup(t)
	f.SeedTeam(SeedTeamOpts{Alias: "active-team"})
	f.SeedTeam(SeedTeamOpts{Alias: "blocked-team", Blocked: true})
	f.NavigateToTeams()

	body := f.Text("#teams-table")
	assert.Contains(t, body, "Active")
	assert.Contains(t, body, "Blocked")
}

func TestTeamsList_BudgetUnlimited(t *testing.T) {
	f := setup(t)
	f.SeedTeam(SeedTeamOpts{Alias: "no-budget-team"})
	f.NavigateToTeams()

	// No budget → shows infinity symbol
	assert.Contains(t, f.Text("#teams-table"), "∞")
}

func TestTeamsList_NoOrganization(t *testing.T) {
	f := setup(t)
	f.SeedTeam(SeedTeamOpts{Alias: "orphan-team"})
	f.NavigateToTeams()

	assert.Contains(t, f.Text("#teams-table"), "No Organization")
}

func TestTeamsList_EmptyModelsShowsAll(t *testing.T) {
	f := setup(t)
	f.SeedTeam(SeedTeamOpts{Alias: "all-models-team"})
	f.NavigateToTeams()

	assert.Contains(t, f.Text("#teams-table"), "All")
}

func TestTeamsList_Performance200Teams(t *testing.T) {
	f := setup(t)
	// Seed 200+ teams
	for i := range 205 {
		f.SeedTeam(SeedTeamOpts{Alias: fmt.Sprintf("perf-team-%d", i+1)})
	}

	start := time.Now()
	f.NavigateToTeams()
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 3*time.Second, "Teams list should load within 3 seconds with 200+ teams")
	// Verify data loaded
	rows := f.Count("#teams-table table tbody tr")
	assert.Equal(t, 50, rows, "Should show first page of 50")
}
