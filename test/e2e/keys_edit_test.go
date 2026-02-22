//go:build e2e

package e2e

import (
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US4 — Edit Settings: form prefill, save, toast, cancel.

func TestKeyEdit_FormPrefilled(t *testing.T) {
	f := setup(t)
	budget := 200.0
	tpm := int64(8000)
	hash := f.SeedKey(SeedOpts{
		Alias:     "edit-prefill",
		MaxBudget: &budget,
		TPMLimit:  &tpm,
		Models:    []string{"gpt-4o"},
	})

	f.NavigateToKeyDetail(hash)

	// Switch to Settings tab
	require.NoError(t, f.Page.Locator("[data-tui-tabs-trigger]").Filter(playwright.LocatorFilterOptions{
		HasText: "Settings",
	}).Click())
	f.WaitStable()

	// Click Edit Settings
	f.ClickButton("Edit Settings")
	f.WaitStable()

	// Verify form fields are prefilled via InputValue (reads current input value)
	alias, err := f.Page.Locator("#edit_key_alias").InputValue()
	require.NoError(t, err)
	assert.Equal(t, "edit-prefill", alias)

	maxBudget, err := f.Page.Locator("#edit_max_budget").InputValue()
	require.NoError(t, err)
	assert.Equal(t, "200.00", maxBudget)

	tpmVal, err := f.Page.Locator("#edit_tpm_limit").InputValue()
	require.NoError(t, err)
	assert.Equal(t, "8000", tpmVal)

	models, err := f.Page.Locator("#edit_models").InputValue()
	require.NoError(t, err)
	assert.Equal(t, "gpt-4o", models)
}

func TestKeyEdit_SaveUpdatesAndShowsToast(t *testing.T) {
	f := setup(t)
	hash := f.SeedKey(SeedOpts{Alias: "edit-save"})

	f.NavigateToKeyDetail(hash)

	// Settings → Edit
	require.NoError(t, f.Page.Locator("[data-tui-tabs-trigger]").Filter(playwright.LocatorFilterOptions{
		HasText: "Settings",
	}).Click())
	f.WaitStable()
	f.ClickButton("Edit Settings")
	f.WaitStable()

	// Change alias
	f.InputByID("edit_key_alias", "edit-save-updated")
	f.InputByID("edit_max_budget", "999")

	// Submit
	require.NoError(t, f.Page.Locator("button[type=submit]").Filter(playwright.LocatorFilterOptions{
		HasText: "Save Changes",
	}).Click())
	f.WaitStable()

	// Toast should appear
	text := f.WaitToast()
	assert.Contains(t, text, "updated")

	// Settings view should show new values
	body := f.Text("#settings-content")
	assert.Contains(t, body, "edit-save-updated")
	assert.Contains(t, body, "$999.00")
}

func TestKeyEdit_CancelReturnsToView(t *testing.T) {
	f := setup(t)
	hash := f.SeedKey(SeedOpts{Alias: "edit-cancel"})

	f.NavigateToKeyDetail(hash)

	// Settings → Edit
	require.NoError(t, f.Page.Locator("[data-tui-tabs-trigger]").Filter(playwright.LocatorFilterOptions{
		HasText: "Settings",
	}).Click())
	f.WaitStable()
	f.ClickButton("Edit Settings")
	f.WaitStable()

	// Click Cancel
	require.NoError(t, f.Page.Locator("#settings-content button").Filter(playwright.LocatorFilterOptions{
		HasText: "Cancel",
	}).Click())
	f.WaitStable()

	// Should be back in view mode with Edit Settings button
	assert.Contains(t, f.Text("#settings-content"), "Edit Settings")
	assert.Contains(t, f.Text("#settings-content"), "edit-cancel")
}
