//go:build e2e

package e2e

import (
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrgMembers_AddMember(t *testing.T) {
	f := setup(t)
	orgID := f.SeedOrg(SeedOrgOpts{Alias: "member-add-org"})
	f.SeedUser("new-org-user")
	f.NavigateToOrgDetail(orgID)

	require.NoError(t, f.Page.Locator(`#org-members-table input[name="user_id"]`).Fill("new-org-user"))
	f.ClickButtonIn("#org-members-table", "Add Member")
	f.WaitStable()

	// Member row should appear
	assert.Contains(t, f.Text("#org-members-table"), "new-org-user")
}

func TestOrgMembers_AddMemberWithRole(t *testing.T) {
	f := setup(t)
	orgID := f.SeedOrg(SeedOrgOpts{Alias: "role-org"})
	f.SeedUser("org-admin-user")
	f.NavigateToOrgDetail(orgID)

	require.NoError(t, f.Page.Locator(`#org-members-table input[name="user_id"]`).Fill("org-admin-user"))
	_, err := f.Page.Locator(`#org-members-table form select[name="user_role"]`).First().SelectOption(playwright.SelectOptionValues{
		Values: &[]string{"org_admin"},
	})
	require.NoError(t, err)
	f.ClickButtonIn("#org-members-table", "Add Member")
	f.WaitStable()

	body := f.Text("#org-members-table")
	assert.Contains(t, body, "org-admin-user")
	assert.Contains(t, body, "org_admin")
}

func TestOrgMembers_DuplicateError(t *testing.T) {
	f := setup(t)
	orgID := f.SeedOrg(SeedOrgOpts{Alias: "dup-member-org"})
	f.SeedUser("dup-org-user")
	f.SeedOrgMember(orgID, "dup-org-user", "member")
	f.NavigateToOrgDetail(orgID)

	// Try adding the same user again — use the add form's visible input
	require.NoError(t, f.Page.Locator(`#org-members-table form[hx-post*="members/add"] input[name="user_id"]`).Fill("dup-org-user"))
	f.ClickButtonIn("#org-members-table", "Add Member")
	f.WaitStable()

	text := f.WaitToast()
	assert.Contains(t, text, "Failed")
}

func TestOrgMembers_ChangeRole(t *testing.T) {
	f := setup(t)
	orgID := f.SeedOrg(SeedOrgOpts{Alias: "role-change-org"})
	f.SeedUser("role-user")
	f.SeedOrgMember(orgID, "role-user", "member")
	f.NavigateToOrgDetail(orgID)

	// The inline <select> with hx-trigger="change" for role update
	// Find the select in the member's row (the one that's NOT in the add form)
	roleSelect := f.Page.Locator(`#org-members-table select[hx-post*="members/update"]`)
	_, err := roleSelect.SelectOption(playwright.SelectOptionValues{
		Values: &[]string{"org_admin"},
	})
	require.NoError(t, err)
	f.WaitStable()

	// DOM should reflect new role
	assert.Contains(t, f.Text("#org-members-table"), "org_admin")
}

func TestOrgMembers_RemoveMember(t *testing.T) {
	f := setup(t)
	orgID := f.SeedOrg(SeedOrgOpts{Alias: "remove-member-org"})
	f.SeedUser("remove-org-user")
	f.SeedOrgMember(orgID, "remove-org-user", "member")
	f.NavigateToOrgDetail(orgID)

	assert.Contains(t, f.Text("#org-members-table"), "remove-org-user")

	require.NoError(t, f.Page.Locator(`#org-members-table form[hx-post*="members/remove"] button`).Click())
	f.WaitStable()

	assert.NotContains(t, f.Text("#org-members-table"), "remove-org-user")
}

func TestOrgMembers_EmptyState(t *testing.T) {
	f := setup(t)
	orgID := f.SeedOrg(SeedOrgOpts{Alias: "empty-members-org"})
	f.NavigateToOrgDetail(orgID)

	assert.Contains(t, f.Text("#org-members-table"), "No members yet")
}
