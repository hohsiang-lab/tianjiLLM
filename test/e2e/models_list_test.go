//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// US1 — View Models with Search & Pagination.

func TestModelsList_EmptyState(t *testing.T) {
	f := setup(t)
	f.NavigateToModels()

	assert.Contains(t, f.Text("#models-table"), "No models configured")
}

func TestModelsList_ShowsModels(t *testing.T) {
	f := setup(t)
	f.SeedModels(3)
	f.NavigateToModels()

	rows := f.Count("#models-table table tbody tr")
	assert.Equal(t, 3, rows)

	body := f.Text("#models-table")
	assert.Contains(t, body, "e2e-model-1")
	assert.Contains(t, body, "e2e-model-2")
	assert.Contains(t, body, "e2e-model-3")
}

func TestModelsList_APIKeyMasked(t *testing.T) {
	f := setup(t)
	f.SeedModel(SeedModelOpts{
		ModelName: "masked-key-model",
		APIKey:    "sk-ant-secret-key-12345678",
	})
	f.NavigateToModels()

	body := f.Text("#models-table")
	// Full key must NOT be visible
	assert.NotContains(t, body, "sk-ant-secret-key-12345678")
	// Masked suffix should be visible
	assert.Contains(t, body, "5678")
}

func TestModelsList_FilterByName(t *testing.T) {
	f := setup(t)
	f.SeedModels(5)
	f.NavigateToModels()

	f.FilterModels("e2e-model-3")

	rows := f.Count("#models-table table tbody tr")
	assert.Equal(t, 1, rows)
	assert.Contains(t, f.Text("#models-table"), "e2e-model-3")
}

func TestModelsList_FilterNoMatch(t *testing.T) {
	f := setup(t)
	f.SeedModels(3)
	f.NavigateToModels()

	f.FilterModels("nonexistent-model")

	assert.Contains(t, f.Text("#models-table"), "No models match your search")
}

func TestModelsList_Pagination(t *testing.T) {
	f := setup(t)
	f.SeedModels(25)
	f.NavigateToModels()

	// First page: 20 rows
	rows := f.Count("#models-table table tbody tr")
	assert.Equal(t, 20, rows)

	// Click next page
	f.Page.Locator("[aria-label='Go to next page']").Click()
	time.Sleep(500 * time.Millisecond)
	f.WaitStable()

	// Second page: 5 rows
	rows = f.Count("#models-table table tbody tr")
	assert.Equal(t, 5, rows)
}

func TestModelsList_DBVerification(t *testing.T) {
	f := setup(t)
	ids := f.SeedModels(2)
	f.NavigateToModels()

	// Verify via DB that models exist
	ctx := context.Background()
	for _, id := range ids {
		m, err := testDB.GetProxyModel(ctx, id)
		assert.NoError(t, err)
		assert.NotEmpty(t, m.ModelName)
	}
}
