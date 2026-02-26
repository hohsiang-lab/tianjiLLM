//go:build e2e

package e2e

import (
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamMembers_AddMember(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "member-add-team"})
	f.NavigateToTeamDetail(teamID)

	// Fill user_id and submit
	require.NoError(t, f.Page.Locator(`#team-members-section input[name="user_id"]`).Fill("new-user-123"))
	f.ClickButtonIn("#team-members-section", "Add Member")
	f.WaitStable()

	// Member row should appear in DOM
	assert.Contains(t, f.Text("#team-members-section"), "new-user-123")
}

func TestTeamMembers_AddMemberWithRole(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "member-role-team"})
	f.NavigateToTeamDetail(teamID)

	require.NoError(t, f.Page.Locator(`#team-members-section input[name="user_id"]`).Fill("admin-user"))
	_, err := f.Page.Locator(`#team-members-section select[name="role"]`).SelectOption(playwright.SelectOptionValues{
		Values: &[]string{"admin"},
	})
	require.NoError(t, err)
	f.ClickButtonIn("#team-members-section", "Add Member")
	f.WaitStable()

	body := f.Text("#team-members-section")
	assert.Contains(t, body, "admin-user")
	assert.Contains(t, body, "admin")
}

func TestTeamMembers_DuplicateError(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{
		Alias:   "dup-member-team",
		Members: []string{"existing-user"},
		MembersWithRoles: []TeamMemberSeed{
			{UserID: "existing-user", Role: "member"},
		},
	})
	f.NavigateToTeamDetail(teamID)

	// Try adding the same user again — use the form's visible input (not hidden)
	require.NoError(t, f.Page.Locator(`#team-members-section form[hx-post*="members/add"] input[name="user_id"]`).Fill("existing-user"))
	f.ClickButtonIn("#team-members-section", "Add Member")
	f.WaitStable()

	// Should show error
	text := f.WaitToast()
	assert.Contains(t, text, "already")
}

func TestTeamMembers_RemoveMember(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{
		Alias:   "remove-member-team",
		Members: []string{"remove-me"},
		MembersWithRoles: []TeamMemberSeed{
			{UserID: "remove-me", Role: "member"},
		},
	})
	f.NavigateToTeamDetail(teamID)

	// Verify member is present
	assert.Contains(t, f.Text("#team-members-section"), "remove-me")

	// Click remove button (trash icon in the member row)
	require.NoError(t, f.Page.Locator(`#team-members-section form[hx-post*="members/remove"] button`).Click())
	f.WaitStable()

	// Member should be gone from DOM
	assert.NotContains(t, f.Text("#team-members-section"), "remove-me")
}

func TestTeamMembers_EmptyState(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "empty-members-team"})
	f.NavigateToTeamDetail(teamID)

	assert.Contains(t, f.Text("#team-members-section"), "No members yet")
}
