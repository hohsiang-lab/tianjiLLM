//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// US1 — Virtual Keys List with server-side filtering and pagination.

func TestKeysList_EmptyState(t *testing.T) {
	f := setup(t)
	f.NavigateToKeys()

	assert.Contains(t, f.Text("#keys-table"), "No keys found")
}

func TestKeysList_ShowsKeys(t *testing.T) {
	f := setup(t)
	f.SeedKeys(3)
	f.NavigateToKeys()

	// 3 data rows (exclude header row)
	rows := f.Count("table tbody tr")
	assert.Equal(t, 3, rows)

	// Verify aliases are visible
	body := f.Text("#keys-table")
	assert.Contains(t, body, "test-key-1")
	assert.Contains(t, body, "test-key-2")
	assert.Contains(t, body, "test-key-3")
}

func TestKeysList_ExpiresNever(t *testing.T) {
	f := setup(t)
	f.SeedKey(SeedOpts{Alias: "never-expires"})
	f.NavigateToKeys()

	assert.Contains(t, f.Text("#keys-table"), "Never")
}

func TestKeysList_BudgetUnlimited(t *testing.T) {
	f := setup(t)
	f.SeedKey(SeedOpts{Alias: "no-budget"})
	f.NavigateToKeys()

	assert.Contains(t, f.Text("#keys-table"), "Unlimited")
}

func TestKeysList_FilterByAlias(t *testing.T) {
	f := setup(t)
	f.SeedKeys(5)
	f.NavigateToKeys()

	// Fill triggers real keyboard input → HTMX "input changed" fires correctly.
	// Wait for debounce (300ms) + HTMX round-trip.
	f.FilterByName("key_alias", "test-key-3")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	rows := f.Count("table tbody tr")
	assert.Equal(t, 1, rows)
	assert.Contains(t, f.Text("#keys-table"), "test-key-3")
}

func TestKeysList_FilterNoMatch(t *testing.T) {
	f := setup(t)
	f.SeedKeys(3)
	f.NavigateToKeys()

	f.FilterByName("key_alias", "nonexistent-key")
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	assert.Contains(t, f.Text("#keys-table"), "No keys match your filters")
}

func TestKeysList_StatusBadges(t *testing.T) {
	f := setup(t)

	// Active key
	f.SeedKey(SeedOpts{Alias: "active-key"})
	// Blocked key
	f.SeedKey(SeedOpts{Alias: "blocked-key", Blocked: true})
	// Expired key
	past := time.Now().Add(-24 * time.Hour)
	f.SeedKey(SeedOpts{Alias: "expired-key", Expires: &past})

	f.NavigateToKeys()

	body := f.Text("#keys-table")
	assert.Contains(t, body, "Active")
	assert.Contains(t, body, "Blocked")
	assert.Contains(t, body, "Expired")
}
