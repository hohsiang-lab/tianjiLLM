//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamCreate_HappyPath(t *testing.T) {
	f := setup(t)
	f.NavigateToTeams()

	f.ClickButton("New Team")
	f.WaitDialogOpen("create-team-dialog")

	f.InputByID("team_alias", "new-team-e2e")
	f.SubmitDialog("create-team-dialog", "Create Team")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	// Team should appear in the list (DOM verification — not just HTTP 200)
	assert.Contains(t, f.Text("#teams-table"), "new-team-e2e")

	// Toast feedback
	text := f.WaitToast()
	assert.Contains(t, text, "created")
}

func TestTeamCreate_RequiresAlias(t *testing.T) {
	f := setup(t)
	f.NavigateToTeams()

	f.ClickButton("New Team")
	f.WaitDialogOpen("create-team-dialog")

	// team_alias has required attribute
	val, err := f.Page.Locator("#team_alias").GetAttribute("required")
	require.NoError(t, err)
	assert.NotEmpty(t, val)
}

func TestTeamCreate_DuplicateAlias(t *testing.T) {
	f := setup(t)
	f.SeedTeam(SeedTeamOpts{Alias: "dup-alias-team"})
	f.NavigateToTeams()

	f.ClickButton("New Team")
	f.WaitDialogOpen("create-team-dialog")

	f.InputByID("team_alias", "dup-alias-team")
	f.SubmitDialog("create-team-dialog", "Create Team")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	// Should show error toast about duplicate alias
	text := f.WaitToast()
	assert.Contains(t, text, "alias")
}

func TestTeamCreate_WithOptionalSettings(t *testing.T) {
	f := setup(t)
	f.NavigateToTeams()

	f.ClickButton("New Team")
	f.WaitDialogOpen("create-team-dialog")

	f.InputByID("team_alias", "full-opts-team")

	// Expand optional settings
	require.NoError(t, f.Page.Locator("#create-team-dialog details summary").First().Click())
	f.WaitStable()

	f.InputByID("team_max_budget", "500")
	f.InputByID("team_tpm_limit", "10000")
	f.InputByID("team_rpm_limit", "100")

	f.SubmitDialog("create-team-dialog", "Create Team")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	assert.Contains(t, f.Text("#teams-table"), "full-opts-team")
}

func TestTeamCreate_CancelClosesDialog(t *testing.T) {
	f := setup(t)
	f.NavigateToTeams()

	f.ClickButton("New Team")
	f.WaitDialogOpen("create-team-dialog")

	f.ClickButtonIn("#create-team-dialog", "Cancel")
	f.WaitDialogClose("create-team-dialog")

	assert.Contains(t, f.Text("#teams-table"), "No teams found")
}
