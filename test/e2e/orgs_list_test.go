//go:build e2e

package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOrgsList_EmptyState(t *testing.T) {
	f := setup(t)
	f.NavigateToOrgs()

	assert.Contains(t, f.Text("#orgs-table"), "No organizations found")
}

func TestOrgsList_ShowsOrgs(t *testing.T) {
	f := setup(t)
	f.SeedOrgs(3)
	f.NavigateToOrgs()

	rows := f.Count("#orgs-table table tbody tr")
	assert.Equal(t, 3, rows)

	body := f.Text("#orgs-table")
	assert.Contains(t, body, "test-org-1")
	assert.Contains(t, body, "test-org-2")
	assert.Contains(t, body, "test-org-3")
}

func TestOrgsList_ColumnsWithTeamAndMemberCount(t *testing.T) {
	f := setup(t)
	orgID := f.SeedOrg(SeedOrgOpts{Alias: "count-org"})
	// Create teams under the org
	f.SeedTeam(SeedTeamOpts{Alias: "org-team-1", OrgID: orgID})
	f.SeedTeam(SeedTeamOpts{Alias: "org-team-2", OrgID: orgID})
	// Add org members
	f.SeedUser("org-user-1")
	f.SeedUser("org-user-2")
	f.SeedUser("org-user-3")
	f.SeedOrgMember(orgID, "org-user-1", "admin")
	f.SeedOrgMember(orgID, "org-user-2", "member")
	f.SeedOrgMember(orgID, "org-user-3", "member")

	f.NavigateToOrgs()

	body := f.Text("#orgs-table")
	assert.Contains(t, body, "count-org")
	// Team count = 2
	assert.Contains(t, body, "2")
	// Member count = 3
	assert.Contains(t, body, "3")
}

func TestOrgsList_FilterByAlias(t *testing.T) {
	f := setup(t)
	f.SeedOrgs(5)
	f.NavigateToOrgs()

	f.FilterOrgs("test-org-3")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	rows := f.Count("#orgs-table table tbody tr")
	assert.Equal(t, 1, rows)
	assert.Contains(t, f.Text("#orgs-table"), "test-org-3")
}

func TestOrgsList_BudgetUnlimited(t *testing.T) {
	f := setup(t)
	f.SeedOrg(SeedOrgOpts{Alias: "no-budget-org"})
	f.NavigateToOrgs()

	assert.Contains(t, f.Text("#orgs-table"), "∞")
}

func TestOrgsList_Performance200Orgs(t *testing.T) {
	f := setup(t)
	for i := range 205 {
		f.SeedOrg(SeedOrgOpts{Alias: fmt.Sprintf("perf-org-%d", i+1)})
	}

	start := time.Now()
	f.NavigateToOrgs()
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 3*time.Second, "Orgs list should load within 3 seconds with 200+ orgs")
	rows := f.Count("#orgs-table table tbody tr")
	assert.Equal(t, 50, rows, "Should show first page of 50")
}
