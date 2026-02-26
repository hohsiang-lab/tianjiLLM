//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrgCreate_HappyPath(t *testing.T) {
	f := setup(t)
	f.NavigateToOrgs()

	f.ClickButton("New Organization")
	f.WaitDialogOpen("create-org-dialog")

	f.InputByID("org_alias", "new-org-e2e")
	f.SubmitDialog("create-org-dialog", "Create Organization")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	// Org should appear in the list (DOM check)
	assert.Contains(t, f.Text("#orgs-table"), "new-org-e2e")

	text := f.WaitToast()
	assert.Contains(t, text, "created")
}

func TestOrgCreate_RequiresAlias(t *testing.T) {
	f := setup(t)
	f.NavigateToOrgs()

	f.ClickButton("New Organization")
	f.WaitDialogOpen("create-org-dialog")

	val, err := f.Page.Locator("#org_alias").GetAttribute("required")
	require.NoError(t, err)
	assert.NotEmpty(t, val)
}

func TestOrgCreate_WithOptionalSettings(t *testing.T) {
	f := setup(t)
	f.NavigateToOrgs()

	f.ClickButton("New Organization")
	f.WaitDialogOpen("create-org-dialog")

	f.InputByID("org_alias", "full-opts-org")

	require.NoError(t, f.Page.Locator("#create-org-dialog details summary").First().Click())
	f.WaitStable()

	f.InputByID("org_max_budget", "1000")
	f.InputByID("org_tpm_limit", "50000")
	f.InputByID("org_rpm_limit", "500")

	f.SubmitDialog("create-org-dialog", "Create Organization")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	assert.Contains(t, f.Text("#orgs-table"), "full-opts-org")
}

func TestOrgCreate_CancelClosesDialog(t *testing.T) {
	f := setup(t)
	f.NavigateToOrgs()

	f.ClickButton("New Organization")
	f.WaitDialogOpen("create-org-dialog")

	f.ClickButtonIn("#create-org-dialog", "Cancel")
	f.WaitDialogClose("create-org-dialog")

	assert.Contains(t, f.Text("#orgs-table"), "No organizations found")
}
