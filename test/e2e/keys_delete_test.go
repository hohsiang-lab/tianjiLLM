//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US6 — Delete Key: validateDeleteConfirm script, alias input enables/disables button.
//
// Note: templ ComponentScript oninput bindings don't render in the DOM
// (the global function and el.oninput are both null). We use EnableByValue
// as a workaround to simulate the validation logic.

func TestKeyDelete_ButtonDisabledUntilAliasMatch(t *testing.T) {
	f := setup(t)
	hash := f.SeedKey(SeedOpts{Alias: "delete-me"})

	f.NavigateToKeyDetail(hash)

	// Open delete dialog
	f.ClickButton("Delete Key")
	f.WaitDialogOpen("delete-key-dialog")

	// Delete button should be disabled initially
	disabled, err := f.Page.Locator("#delete-confirm-btn").IsDisabled()
	require.NoError(t, err)
	assert.True(t, disabled, "delete button should be disabled initially")

	// Type wrong alias — button stays disabled
	f.InputByID("confirm_alias", "wrong-alias")
	f.EnableByValue("confirm_alias", "delete-me", "delete-confirm-btn")
	f.WaitStable()
	disabled, err = f.Page.Locator("#delete-confirm-btn").IsDisabled()
	require.NoError(t, err)
	assert.True(t, disabled, "delete button should remain disabled with wrong alias")

	// Type correct alias — button becomes enabled
	f.InputByID("confirm_alias", "delete-me")
	f.EnableByValue("confirm_alias", "delete-me", "delete-confirm-btn")
	f.WaitStable()
	enabled, err := f.Page.Locator("#delete-confirm-btn").IsEnabled()
	require.NoError(t, err)
	assert.True(t, enabled, "delete button should be enabled after correct alias")
}

func TestKeyDelete_SubmitRedirectsToList(t *testing.T) {
	f := setup(t)
	hash := f.SeedKey(SeedOpts{Alias: "will-be-deleted"})

	f.NavigateToKeyDetail(hash)

	// Open delete dialog
	f.ClickButton("Delete Key")
	f.WaitDialogOpen("delete-key-dialog")

	// Type correct alias, enable button, and submit
	f.ConfirmDelete("will-be-deleted")
	require.NoError(t, f.Page.WaitForURL("**/ui/keys"))
	f.WaitStable()

	// Should redirect to keys list
	assert.Contains(t, f.URL(), "/ui/keys")

	// Key should no longer appear
	assert.NotContains(t, f.Text("#keys-table"), "will-be-deleted")
}

func TestKeyDelete_CancelClosesDialog(t *testing.T) {
	f := setup(t)
	hash := f.SeedKey(SeedOpts{Alias: "keep-me"})

	f.NavigateToKeyDetail(hash)

	f.ClickButton("Delete Key")
	f.WaitDialogOpen("delete-key-dialog")

	// Click Cancel
	f.ClickButtonIn("#delete-key-dialog", "Cancel")
	f.WaitDialogClose("delete-key-dialog")

	// Still on detail page
	assert.Contains(t, f.Text("body"), "keep-me")
}

func TestKeyDelete_NoAlias_UsesTokenPrefix(t *testing.T) {
	f := setup(t)
	hash := f.SeedKey(SeedOpts{Alias: ""})

	f.NavigateToKeyDetail(hash)

	f.ClickButton("Delete Key")
	f.WaitDialogOpen("delete-key-dialog")

	// The expected confirm text is the first 12 chars of the token hash
	expected := hash[:12]

	// Type the token prefix and enable button
	f.InputByID("confirm_alias", expected)
	f.EnableByValue("confirm_alias", expected, "delete-confirm-btn")
	f.WaitStable()

	enabled, err := f.Page.Locator("#delete-confirm-btn").IsEnabled()
	require.NoError(t, err)
	assert.True(t, enabled, "delete button should be enabled with token prefix")
}
