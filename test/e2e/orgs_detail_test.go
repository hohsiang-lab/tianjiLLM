//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrgDetail_NavigateFromList(t *testing.T) {
	f := setup(t)
	f.SeedOrg(SeedOrgOpts{Alias: "detail-nav-org"})
	f.NavigateToOrgs()

	require.NoError(t, f.Page.Locator("#orgs-table table tbody tr a").First().Click())
	require.NoError(t, f.Page.WaitForURL("**/ui/orgs/**"))
	f.WaitStable()

	assert.Contains(t, f.Text("body"), "detail-nav-org")
}

func TestOrgDetail_AllFieldsDisplayed(t *testing.T) {
	f := setup(t)
	budget := 500.0
	orgID := f.SeedOrg(SeedOrgOpts{
		Alias:     "full-detail-org",
		MaxBudget: &budget,
		Spend:     50.0,
		Models:    []string{"gpt-4o"},
	})
	// Add members
	f.SeedUser("org-detail-user")
	f.SeedOrgMember(orgID, "org-detail-user", "admin")
	// Add a team under this org
	f.SeedTeam(SeedTeamOpts{Alias: "org-child-team", OrgID: orgID})

	f.NavigateToOrgDetail(orgID)

	body := f.Text("body")
	assert.Contains(t, body, "full-detail-org")
	assert.Contains(t, body, "$50.00")
	assert.Contains(t, body, "$500.00")
	assert.Contains(t, body, "gpt-4o")
	assert.Contains(t, body, "org-detail-user")
	assert.Contains(t, body, "admin")
	assert.Contains(t, body, "org-child-team")
}

func TestOrgDetail_EditAlias(t *testing.T) {
	f := setup(t)
	orgID := f.SeedOrg(SeedOrgOpts{Alias: "edit-org-alias"})
	f.NavigateToOrgDetail(orgID)

	f.ClickButton("Edit")
	f.WaitDialogOpen("edit-org-dialog")

	f.InputByID("edit_org_alias", "renamed-org")
	f.SubmitDialog("edit-org-dialog", "Save Changes")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	assert.Contains(t, f.Text("#org-detail-content"), "renamed-org")
}

func TestOrgDetail_EditBudget(t *testing.T) {
	f := setup(t)
	budget := 100.0
	orgID := f.SeedOrg(SeedOrgOpts{Alias: "budget-edit-org", MaxBudget: &budget})
	f.NavigateToOrgDetail(orgID)

	f.ClickButton("Edit")
	f.WaitDialogOpen("edit-org-dialog")

	f.InputByID("edit_org_max_budget", "999")
	f.SubmitDialog("edit-org-dialog", "Save Changes")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	assert.Contains(t, f.Text("#org-detail-content"), "$999.00")
}

func TestOrgDetail_BackToOrgs(t *testing.T) {
	f := setup(t)
	orgID := f.SeedOrg(SeedOrgOpts{Alias: "back-test-org"})
	f.NavigateToOrgDetail(orgID)

	f.ClickButton("← Back to Organizations")
	require.NoError(t, f.Page.WaitForURL("**/ui/orgs"))
	f.WaitStable()

	assert.Contains(t, f.URL(), "/ui/orgs")
}
