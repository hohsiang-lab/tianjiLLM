//go:build e2e

package e2e

import (
	"context"
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
)

// US4 — Delete Model.

func TestModelDelete_RemovesFromTable(t *testing.T) {
	f := setup(t)
	modelID := f.SeedModel(SeedModelOpts{ModelName: "delete-me-model"})
	f.NavigateToModels()

	// Verify model is visible
	assert.Contains(t, f.Text("#models-table"), "delete-me-model")

	// Click Delete (hx-confirm auto-accepted by page.OnDialog handler)
	f.Page.Locator("#models-table table tbody tr").First().GetByRole("button", playwright.LocatorGetByRoleOptions{
		Name: "Delete",
	}).Click()
	f.WaitStable()

	// Verify model removed from table
	assert.NotContains(t, f.Text("#models-table"), "delete-me-model")

	// Verify model removed from DB
	ctx := context.Background()
	_, err := testDB.GetProxyModel(ctx, modelID)
	assert.Error(t, err, "model should be deleted from DB")
}

func TestModelDelete_CountDecreases(t *testing.T) {
	f := setup(t)
	f.SeedModels(3)
	f.NavigateToModels()

	// Verify 3 rows
	assert.Equal(t, 3, f.Count("#models-table table tbody tr"))

	// Delete first model
	f.Page.Locator("#models-table table tbody tr").First().GetByRole("button", playwright.LocatorGetByRoleOptions{
		Name: "Delete",
	}).Click()
	f.WaitStable()

	// Verify 2 rows remain
	assert.Equal(t, 2, f.Count("#models-table table tbody tr"))
}
