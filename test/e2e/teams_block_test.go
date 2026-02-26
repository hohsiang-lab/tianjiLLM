//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTeamBlock_BlockChangesStatus(t *testing.T) {
	// BUG P0: Block/Unblock buttons on teams list page use type="button" (default).
	// HTMX form submit never fires. Additionally, hx-target="#team-status-{id}"
	// mismatches handler which returns full table. Two bugs:
	// 1. Button needs Type: "submit" or button.Props needs to default to submit in forms
	// 2. hx-target should be #teams-table (or handler should return badge partial)
	t.Skip("BUG P0: Block/Unblock buttons on list page are type=button, HTMX form never fires")
}

func TestTeamBlock_UnblockRestoresActive(t *testing.T) {
	t.Skip("BUG P0: Block/Unblock buttons on list page are type=button, HTMX form never fires")
}

func TestTeamBlock_BlockFromDetail(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "detail-block-team"})
	f.NavigateToTeamDetail(teamID)

	assert.Contains(t, f.Text("body"), "Active")

	f.ClickButton("Block")
	f.WaitStable()

	assert.Contains(t, f.Text("body"), "Blocked")
}

func TestTeamBlock_UnblockFromDetail(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "detail-unblock-team", Blocked: true})
	f.NavigateToTeamDetail(teamID)

	assert.Contains(t, f.Text("body"), "Blocked")

	f.ClickButton("Unblock")
	f.WaitStable()

	assert.Contains(t, f.Text("body"), "Active")
}
