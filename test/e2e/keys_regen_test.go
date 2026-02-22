//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// US7 — Regenerate Key: dialog, new raw key displayed.

func TestKeyRegenerate_ShowsNewKey(t *testing.T) {
	f := setup(t)
	hash := f.SeedKey(SeedOpts{Alias: "regen-key"})

	f.NavigateToKeyDetail(hash)

	// Open regenerate dialog
	f.ClickButton("Regenerate Key")
	f.WaitDialogOpen("regenerate-dialog")

	// Submit regeneration
	f.SubmitDialog("regenerate-dialog", "Regenerate")

	// New raw key should appear in #regenerate-result
	result := f.Text("#regenerate-result")
	assert.Contains(t, result, "sk-", "regenerated key should start with sk-")
	assert.Contains(t, result, "only be shown once")
}

func TestKeyRegenerate_CopyButtonExists(t *testing.T) {
	f := setup(t)
	hash := f.SeedKey(SeedOpts{Alias: "regen-copy"})

	f.NavigateToKeyDetail(hash)

	f.ClickButton("Regenerate Key")
	f.WaitDialogOpen("regenerate-dialog")

	f.SubmitDialog("regenerate-dialog", "Regenerate")

	// Copy button should exist
	assert.True(t, f.Has("#regenerate-result button"), "copy button should exist")
}

func TestKeyRegenerate_Cancel(t *testing.T) {
	f := setup(t)
	hash := f.SeedKey(SeedOpts{Alias: "regen-cancel"})

	f.NavigateToKeyDetail(hash)

	f.ClickButton("Regenerate Key")
	f.WaitDialogOpen("regenerate-dialog")

	// Cancel
	f.ClickButtonIn("#regenerate-dialog", "Cancel")
	f.WaitDialogClose("regenerate-dialog")

	// Still on detail page
	assert.Contains(t, f.Text("body"), "regen-cancel")
}

func TestKeyRegenerate_WithUpdatedLimits(t *testing.T) {
	f := setup(t)
	budget := 100.0
	hash := f.SeedKey(SeedOpts{
		Alias:     "regen-limits",
		MaxBudget: &budget,
	})

	f.NavigateToKeyDetail(hash)

	f.ClickButton("Regenerate Key")
	f.WaitDialogOpen("regenerate-dialog")

	// Update budget during regeneration
	f.InputByID("regen_max_budget", "999")
	f.InputByID("regen_tpm_limit", "50000")

	f.SubmitDialog("regenerate-dialog", "Regenerate")

	// New key should appear
	result := f.Text("#regenerate-result")
	assert.Contains(t, result, "sk-")
}
