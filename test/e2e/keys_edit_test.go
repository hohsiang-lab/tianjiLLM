//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

	f.NavigateToSettingsEdit(hash)

	assert.Equal(t, "edit-prefill", f.InputValue("edit_key_alias"))
	assert.Equal(t, "200.00", f.InputValue("edit_max_budget"))
	assert.Equal(t, "8000", f.InputValue("edit_tpm_limit"))
}

func TestKeyEdit_SaveUpdatesAndShowsToast(t *testing.T) {
	f := setup(t)
	hash := f.SeedKey(SeedOpts{Alias: "edit-save"})

	f.NavigateToSettingsEdit(hash)

	// Change alias
	f.InputByID("edit_key_alias", "edit-save-updated")
	f.InputByID("edit_max_budget", "999")

	// Submit
	f.ClickButton("Save Changes")
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

	f.NavigateToSettingsEdit(hash)

	// Click Cancel
	f.ClickButtonIn("#settings-content", "Cancel")
	f.WaitStable()

	// Should be back in view mode with Edit Settings button
	assert.Contains(t, f.Text("#settings-content"), "Edit Settings")
	assert.Contains(t, f.Text("#settings-content"), "edit-cancel")
}
