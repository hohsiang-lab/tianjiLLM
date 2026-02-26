//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamDelete_FromList(t *testing.T) {
	f := setup(t)
	f.SeedTeam(SeedTeamOpts{Alias: "delete-list-team"})
	f.NavigateToTeams()

	assert.Contains(t, f.Text("#teams-table"), "delete-list-team")

	// Click the delete (trash) button — opens dialog
	require.NoError(t, f.Page.Locator(`#teams-table [data-tui-dialog-trigger] button`).Last().Click())
	f.WaitStable()

	// Click Delete in dialog — the form targets body with push-url
	require.NoError(t, f.Page.Locator(`form[hx-post*="/delete"] button[type="submit"]`).Click())
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	// Team should disappear from the list
	require.NoError(t, f.Page.WaitForURL("**/ui/teams"))
	assert.NotContains(t, f.Text("#teams-table"), "delete-list-team")
}

func TestTeamDelete_FromDetail(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "delete-detail-team"})
	f.NavigateToTeamDetail(teamID)

	// Open delete dialog
	f.ClickButton("Delete")
	f.WaitDialogOpen("delete-team-detail-dialog")

	// Confirm delete
	require.NoError(t, f.Page.Locator(`#delete-team-detail-dialog form[hx-post*="/delete"] button[type="submit"]`).Click())
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	// Should redirect to teams list
	require.NoError(t, f.Page.WaitForURL("**/ui/teams"))
	assert.NotContains(t, f.Text("#teams-table"), "delete-detail-team")
}

func TestTeamDelete_CancelKeepsTeam(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "keep-team"})
	f.NavigateToTeamDetail(teamID)

	f.ClickButton("Delete")
	f.WaitDialogOpen("delete-team-detail-dialog")

	f.ClickButtonIn("#delete-team-detail-dialog", "Cancel")
	f.WaitDialogClose("delete-team-detail-dialog")

	// Still on detail page
	assert.Contains(t, f.Text("body"), "keep-team")
}
