//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US3 — Create Key: dialog lifecycle, form→submit→close→reveal→copy→close→list update.

func TestKeyCreate_FullLifecycle(t *testing.T) {
	f := setup(t)
	f.NavigateToKeys()

	// 1. Open create dialog
	f.ClickButton("Create New Key")
	f.WaitDialogOpen("create-key-dialog")

	// 2. Fill required alias
	f.InputByID("key_alias", "e2e-created-key")

	// 3. Submit
	f.SubmitDialog("create-key-dialog", "Create")

	// 4. Create dialog should close, reveal dialog should open
	f.WaitKeyReveal()

	// 5. Raw key (sk-...) should be visible
	rawKey, err := f.Page.Locator(".select-all").TextContent()
	require.NoError(t, err)
	assert.Contains(t, rawKey, "sk-", "raw key should start with sk-")

	// 6. Copy button exists
	assert.True(t, f.Has("#key-reveal-dialog button"), "copy button should exist")

	// 7. Close reveal dialog
	f.CloseKeyReveal()

	// 8. Key should now appear in the table
	assert.Contains(t, f.Text("#keys-table"), "e2e-created-key")
}

func TestKeyCreate_RequiresAlias(t *testing.T) {
	f := setup(t)
	f.NavigateToKeys()

	f.ClickButton("Create New Key")
	f.WaitDialogOpen("create-key-dialog")

	// Browser native validation — key_alias has required attribute
	val, err := f.Page.Locator("#key_alias").GetAttribute("required")
	require.NoError(t, err)
	assert.NotEmpty(t, val, "key_alias should have required attribute")
}

func TestKeyCreate_DuplicateAlias(t *testing.T) {
	f := setup(t)
	f.SeedKey(SeedOpts{Alias: "existing-alias"})
	f.NavigateToKeys()

	f.ClickButton("Create New Key")
	f.WaitDialogOpen("create-key-dialog")

	f.InputByID("key_alias", "existing-alias")
	f.SubmitDialog("create-key-dialog", "Create")

	// Should show error toast
	text := f.WaitToast()
	assert.Contains(t, text, "already exists")
}

func TestKeyCreate_WithOptionalSettings(t *testing.T) {
	f := setup(t)
	f.NavigateToKeys()

	f.ClickButton("Create New Key")
	f.WaitDialogOpen("create-key-dialog")

	f.InputByID("key_alias", "full-options-key")

	// Expand optional settings (use HasText to avoid ambiguity with model selector summary)
	require.NoError(t, f.Page.GetByText("Optional Settings").Click())
	f.WaitStable()

	f.InputByID("max_budget", "500")
	f.InputByID("tpm_limit", "10000")
	f.InputByID("rpm_limit", "100")

	f.SubmitDialog("create-key-dialog", "Create")
	f.WaitKeyReveal()
	f.CloseKeyReveal()

	// Verify key appears in list
	assert.Contains(t, f.Text("#keys-table"), "full-options-key")
}

func TestKeyCreate_CancelClosesDialog(t *testing.T) {
	f := setup(t)
	f.NavigateToKeys()

	f.ClickButton("Create New Key")
	f.WaitDialogOpen("create-key-dialog")

	// Click Cancel
	f.ClickButtonIn("#create-key-dialog", "Cancel")
	f.WaitDialogClose("create-key-dialog")

	// Table should still show empty state
	assert.Contains(t, f.Text("#keys-table"), "No keys found")
}
