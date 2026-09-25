//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US2 — Key Detail: Overview and Settings tabs.

func TestKeyDetail_NavigateFromList(t *testing.T) {
	f := setup(t)
	hashes := f.SeedKeys(1)
	f.NavigateToKeys()

	// Click the key ID link in the first row
	require.NoError(t, f.Page.Locator("table tbody tr a").First().Click())
	require.NoError(t, f.Page.WaitForURL("**/ui/keys/**"))
	f.WaitStable()

	// Should be on detail page
	body := f.Text("body")
	assert.Contains(t, body, "test-key-1")
	assert.Contains(t, body, hashes[0][:16])
}

func TestKeyDetail_OverviewTab(t *testing.T) {
	f := setup(t)
	budget := 100.0
	tpm := int64(5000)
	rpm := int64(200)
	hash := f.SeedKey(SeedOpts{
		Alias:     "detail-key",
		Spend:     25.50,
		MaxBudget: &budget,
		Models:    []string{"gpt-4o", "claude-sonnet"},
		TPMLimit:  &tpm,
		RPMLimit:  &rpm,
	})

	f.NavigateToKeyDetail(hash)

	body := f.Text("body")

	// Spend / Budget card
	assert.Contains(t, body, "$25.50")
	assert.Contains(t, body, "$100.00")

	// Rate Limits card
	assert.Contains(t, body, "5000")
	assert.Contains(t, body, "200")

	// Models card
	assert.Contains(t, body, "gpt-4o")
	assert.Contains(t, body, "claude-sonnet")
}

func TestKeyDetail_OverviewUnlimited(t *testing.T) {
	f := setup(t)
	hash := f.SeedKey(SeedOpts{Alias: "unlimited-key"})

	f.NavigateToKeyDetail(hash)

	body := f.Text("body")
	assert.Contains(t, body, "Unlimited")
	assert.Contains(t, body, "All models allowed")
}

func TestKeyDetail_SettingsTab(t *testing.T) {
	f := setup(t)
	hash := f.SeedKey(SeedOpts{Alias: "settings-key"})

	f.NavigateToSettings(hash)

	body := f.Text("#settings-content")
	assert.Contains(t, body, "Key ID")
	assert.Contains(t, body, hash)
	assert.Contains(t, body, "settings-key")
}

func TestKeyDetail_BackToKeys(t *testing.T) {
	f := setup(t)
	hash := f.SeedKey(SeedOpts{Alias: "back-test"})

	f.NavigateToKeyDetail(hash)

	f.ClickButton("← Back to Keys")
	require.NoError(t, f.Page.WaitForURL("**/ui/keys"))
	f.WaitStable()

	// Should be back on list page
	assert.Contains(t, f.Text("body"), "API Keys")
}

func TestKeyDetail_StatusBadge(t *testing.T) {
	f := setup(t)
	hash := f.SeedKey(SeedOpts{Alias: "blocked-detail", Blocked: true})

	f.NavigateToKeyDetail(hash)

	assert.Contains(t, f.Text("body"), "Blocked")
}
