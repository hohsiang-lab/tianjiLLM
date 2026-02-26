//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTeamBlock_BlockChangesStatus(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "list-block-team"})
	_ = teamID
	f.NavigateToTeams()

	assert.Contains(t, f.Text("#teams-table"), "Active")

	f.ClickButtonIn("#teams-table", "Block")
	f.WaitStable()

	assert.Contains(t, f.Text("#teams-table"), "Blocked")
}

func TestTeamBlock_UnblockRestoresActive(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "list-unblock-team", Blocked: true})
	_ = teamID
	f.NavigateToTeams()

	assert.Contains(t, f.Text("#teams-table"), "Blocked")

	f.ClickButtonIn("#teams-table", "Unblock")
	f.WaitStable()

	assert.Contains(t, f.Text("#teams-table"), "Active")
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
