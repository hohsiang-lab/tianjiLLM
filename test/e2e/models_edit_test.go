//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US3 — Edit Existing Model.

func TestModelEdit_PrefilledForm(t *testing.T) {
	f := setup(t)
	f.SeedModel(SeedModelOpts{
		ModelName: "edit-test-model",
		Model:     "anthropic/claude-sonnet-4-5-20250929",
		APIBase:   "https://custom.api.com",
	})
	f.NavigateToModels()

	// Click Edit on the row
	f.Page.Locator("#models-table table tbody tr").First().GetByRole("button", playwright.LocatorGetByRoleOptions{
		Name: "Edit",
	}).Click()
	f.WaitDialogOpen("edit-model-dialog")

	// Verify pre-filled values
	assert.Equal(t, "edit-test-model", f.InputValue("edit_model_name"))
	assert.Equal(t, "anthropic/claude-sonnet-4-5-20250929", f.InputValue("edit_model"))
	assert.Equal(t, "https://custom.api.com", f.InputValue("edit_api_base"))
}

func TestModelEdit_UpdateName(t *testing.T) {
	f := setup(t)
	modelID := f.SeedModel(SeedModelOpts{ModelName: "old-name"})
	f.NavigateToModels()

	// Click Edit
	f.Page.Locator("#models-table table tbody tr").First().GetByRole("button", playwright.LocatorGetByRoleOptions{
		Name: "Edit",
	}).Click()
	f.WaitDialogOpen("edit-model-dialog")

	// Update name
	require.NoError(t, f.Page.Locator("#edit_model_name").Clear())
	f.InputByID("edit_model_name", "new-name")

	f.SubmitDialog("edit-model-dialog", "Save Changes")
	f.WaitStable()

	// Verify in table
	body := f.Text("#models-table")
	assert.Contains(t, body, "new-name")
	assert.NotContains(t, body, "old-name")

	// Verify in DB
	ctx := context.Background()
	dbModel, err := testDB.GetProxyModel(ctx, modelID)
	require.NoError(t, err)
	assert.Equal(t, "new-name", dbModel.ModelName)
}

func TestModelEdit_APIKeyPreservation(t *testing.T) {
	f := setup(t)
	modelID := f.SeedModel(SeedModelOpts{
		ModelName: "key-preserve-model",
		APIKey:    "sk-original-secret-key",
	})
	f.NavigateToModels()

	// Click Edit
	f.Page.Locator("#models-table table tbody tr").First().GetByRole("button", playwright.LocatorGetByRoleOptions{
		Name: "Edit",
	}).Click()
	f.WaitDialogOpen("edit-model-dialog")

	// API key input should be empty (not showing the actual key)
	assert.Equal(t, "", f.InputValue("edit_api_key"))

	// Submit without entering a key
	f.SubmitDialog("edit-model-dialog", "Save Changes")
	f.WaitStable()

	// Verify DB still has original key
	ctx := context.Background()
	dbModel, err := testDB.GetProxyModel(ctx, modelID)
	require.NoError(t, err)

	var tp map[string]any
	require.NoError(t, json.Unmarshal(dbModel.TianjiParams, &tp))
	assert.Equal(t, "sk-original-secret-key", tp["api_key"])
}

func TestModelEdit_PreservesUnknownFields(t *testing.T) {
	f := setup(t)
	modelID := f.SeedModel(SeedModelOpts{
		ModelName: "unknown-fields-model",
		Extra: map[string]any{
			"timeout": 30,
			"region":  "us-east-1",
		},
	})
	f.NavigateToModels()

	// Click Edit
	f.Page.Locator("#models-table table tbody tr").First().GetByRole("button", playwright.LocatorGetByRoleOptions{
		Name: "Edit",
	}).Click()
	f.WaitDialogOpen("edit-model-dialog")

	// Just change the model name
	require.NoError(t, f.Page.Locator("#edit_model_name").Clear())
	f.InputByID("edit_model_name", "renamed-model")
	f.SubmitDialog("edit-model-dialog", "Save Changes")
	f.WaitStable()

	// Verify DB still has timeout and region
	ctx := context.Background()
	dbModel, err := testDB.GetProxyModel(ctx, modelID)
	require.NoError(t, err)

	var tp map[string]any
	require.NoError(t, json.Unmarshal(dbModel.TianjiParams, &tp))
	assert.Equal(t, float64(30), tp["timeout"])
	assert.Equal(t, "us-east-1", tp["region"])
	assert.Equal(t, "renamed-model", dbModel.ModelName)
}

func TestModelEdit_CancelClosesDialog(t *testing.T) {
	f := setup(t)
	f.SeedModel(SeedModelOpts{ModelName: "dont-edit-me"})
	f.NavigateToModels()

	// Click Edit
	f.Page.Locator("#models-table table tbody tr").First().GetByRole("button", playwright.LocatorGetByRoleOptions{
		Name: "Edit",
	}).Click()
	f.WaitDialogOpen("edit-model-dialog")

	// Click Cancel
	f.ClickButtonIn("#edit-model-dialog", "Cancel")
	f.WaitDialogClose("edit-model-dialog")

	// Model still in table unchanged
	assert.Contains(t, f.Text("#models-table"), "dont-edit-me")
}
