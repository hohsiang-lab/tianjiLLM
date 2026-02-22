//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// US5 — Block/Unblock: hx-confirm dialog, status badge change, button toggle.
// Dialog handling is automatic — page.OnDialog is registered in setup().

func TestKeyBlock_BlockChangesStatus(t *testing.T) {
	f := setup(t)
	f.SeedKey(SeedOpts{Alias: "block-me"})
	f.NavigateToKeys()

	// Verify initial Active status
	assert.Contains(t, f.Text("#keys-table"), "Active")

	// Click Block — hx-confirm dialog auto-accepted by OnDialog handler
	f.ClickButton("Block")
	f.WaitStable()

	// Status should now be Blocked, button should say "Unblock"
	body := f.Text("#keys-table")
	assert.Contains(t, body, "Blocked")
	assert.Contains(t, body, "Unblock")
}

func TestKeyBlock_UnblockRestoresActive(t *testing.T) {
	f := setup(t)
	f.SeedKey(SeedOpts{Alias: "unblock-me", Blocked: true})
	f.NavigateToKeys()

	// Verify initial Blocked status
	assert.Contains(t, f.Text("#keys-table"), "Blocked")

	// Click Unblock — hx-confirm dialog auto-accepted by OnDialog handler
	f.ClickButton("Unblock")
	f.WaitStable()

	// Status should now be Active, button should say "Block" (not "Unblock")
	body := f.Text("#keys-table")
	assert.Contains(t, body, "Active")
	assert.Contains(t, body, "Block")
	assert.NotContains(t, body, "Unblock")
}
