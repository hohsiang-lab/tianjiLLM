//go:build e2e

package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamDetail_NavigateFromList(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "detail-nav-team"})
	f.NavigateToTeams()

	require.NoError(t, f.Page.Locator("#teams-table table tbody tr a").First().Click())
	require.NoError(t, f.Page.WaitForURL("**/ui/teams/**"))
	f.WaitStable()

	body := f.Text("body")
	assert.Contains(t, body, "detail-nav-team")
	assert.Contains(t, body, teamID)
}

func TestTeamDetail_AllFieldsDisplayed(t *testing.T) {
	f := setup(t)
	budget := 200.0
	tpm := int64(5000)
	rpm := int64(200)
	orgID := f.SeedOrg(SeedOrgOpts{Alias: "detail-org"})
	teamID := f.SeedTeam(SeedTeamOpts{
		Alias:     "full-detail-team",
		OrgID:     orgID,
		Spend:     45.00,
		MaxBudget: &budget,
		Models:    []string{"gpt-4o", "claude-3"},
		TPMLimit:  &tpm,
		RPMLimit:  &rpm,
		MembersWithRoles: []TeamMemberSeed{
			{UserID: "user-001", Role: "admin"},
			{UserID: "user-002", Role: "member"},
		},
	})
	f.NavigateToTeamDetail(teamID)

	body := f.Text("body")
	// Alias and status
	assert.Contains(t, body, "full-detail-team")
	assert.Contains(t, body, "Active")
	// Spend / Budget
	assert.Contains(t, body, "$45.00")
	assert.Contains(t, body, "$200.00")
	// Rate limits
	assert.Contains(t, body, "5000")
	assert.Contains(t, body, "200")
	// Models
	assert.Contains(t, body, "gpt-4o")
	assert.Contains(t, body, "claude-3")
	// Members
	assert.Contains(t, body, "user-001")
	assert.Contains(t, body, "user-002")
	assert.Contains(t, body, "admin")
	assert.Contains(t, body, "member")
}

func TestTeamDetail_EditAlias(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "edit-alias-team"})
	f.NavigateToTeamDetail(teamID)

	// Open edit dialog
	f.ClickButton("Edit")
	f.WaitDialogOpen("edit-team-dialog")

	// Change alias
	f.InputByID("edit_team_alias", "renamed-team")
	f.SubmitDialog("edit-team-dialog", "Save Changes")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	// DOM should reflect new alias
	assert.Contains(t, f.Text("#team-detail-content"), "renamed-team")
}

func TestTeamDetail_EditBudget(t *testing.T) {
	f := setup(t)
	budget := 100.0
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "budget-edit-team", MaxBudget: &budget})
	f.NavigateToTeamDetail(teamID)

	f.ClickButton("Edit")
	f.WaitDialogOpen("edit-team-dialog")

	f.InputByID("edit_max_budget", "999")
	f.SubmitDialog("edit-team-dialog", "Save Changes")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	assert.Contains(t, f.Text("#team-detail-content"), "$999.00")
}

func TestTeamDetail_EditBudgetBelowSpend(t *testing.T) {
	f := setup(t)
	budget := 100.0
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "budget-warn-team", MaxBudget: &budget, Spend: 80.0})
	f.NavigateToTeamDetail(teamID)

	f.ClickButton("Edit")
	f.WaitDialogOpen("edit-team-dialog")

	// Set budget below current spend
	f.InputByID("edit_max_budget", "50")
	f.SubmitDialog("edit-team-dialog", "Save Changes")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	// Should still save (spec says warn but allow)
	assert.Contains(t, f.Text("#team-detail-content"), "$50.00")
}

func TestTeamDetail_BackToTeams(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "back-test-team"})
	f.NavigateToTeamDetail(teamID)

	f.ClickButton("← Back to Teams")
	require.NoError(t, f.Page.WaitForURL("**/ui/teams"))
	f.WaitStable()

	assert.Contains(t, f.URL(), "/ui/teams")
}

func TestTeamDetail_BlockedStatus(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "blocked-detail-team", Blocked: true})
	f.NavigateToTeamDetail(teamID)

	assert.Contains(t, f.Text("body"), "Blocked")
}

func TestTeamDetail_EmptyModelsShowsInherited(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "no-models-team"})
	f.NavigateToTeamDetail(teamID)

	body := f.Text("body")
	assert.True(t, strings.Contains(body, "All models") || strings.Contains(body, "Inherited"),
		"Empty models should show 'All models' or 'Inherited'")
}
