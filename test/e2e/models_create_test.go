//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// US2 — Create New Model.

func TestModelCreate_FullLifecycle(t *testing.T) {
	f := setup(t)
	f.NavigateToModels()

	// 1. Open create dialog
	f.ClickButton("Add Model")
	f.WaitDialogOpen("create-model-dialog")

	// 2. Fill required fields
	f.InputByID("model_name", "e2e-new-model")
	f.InputByID("model", "anthropic/claude-sonnet-4-5-20250929")

	// 3. Submit
	f.SubmitDialog("create-model-dialog", "Create")
	f.WaitStable()

	// 4. Verify model appears in table
	body := f.Text("#models-table")
	assert.Contains(t, body, "e2e-new-model")
	assert.Contains(t, body, "anthropic")

	// 5. Verify via DB
	ctx := context.Background()
	dbModel, err := testDB.GetProxyModelByName(ctx, "e2e-new-model")
	require.NoError(t, err)
	assert.Equal(t, "e2e-new-model", dbModel.ModelName)
}

func TestModelCreate_RequiredFields(t *testing.T) {
	f := setup(t)
	f.NavigateToModels()

	f.ClickButton("Add Model")
	f.WaitDialogOpen("create-model-dialog")

	// model_name should have required attribute
	val, err := f.Page.Locator("#model_name").GetAttribute("required")
	require.NoError(t, err)
	assert.NotEmpty(t, val, "model_name should have required attribute")

	// model should have required attribute
	val, err = f.Page.Locator("#model").GetAttribute("required")
	require.NoError(t, err)
	assert.NotEmpty(t, val, "model should have required attribute")
}

func TestModelCreate_DuplicateName(t *testing.T) {
	f := setup(t)
	f.SeedModel(SeedModelOpts{ModelName: "existing-model"})
	f.NavigateToModels()

	f.ClickButton("Add Model")
	f.WaitDialogOpen("create-model-dialog")

	f.InputByID("model_name", "existing-model")
	f.InputByID("model", "openai/gpt-4o")
	f.SubmitDialog("create-model-dialog", "Create")

	// Should show error toast
	text := f.WaitToast()
	assert.Contains(t, text, "already exists")
}

func TestModelCreate_CancelClosesDialog(t *testing.T) {
	f := setup(t)
	f.NavigateToModels()

	f.ClickButton("Add Model")
	f.WaitDialogOpen("create-model-dialog")

	f.ClickButtonIn("#create-model-dialog", "Cancel")
	f.WaitDialogClose("create-model-dialog")

	// Table should still show empty state
	assert.Contains(t, f.Text("#models-table"), "No models configured")
}

func TestModelCreate_WithOptionalFields(t *testing.T) {
	f := setup(t)
	f.NavigateToModels()

	f.ClickButton("Add Model")
	f.WaitDialogOpen("create-model-dialog")

	f.InputByID("model_name", "full-options-model")
	f.InputByID("model", "openai/gpt-4o")
	f.InputByID("api_base", "https://custom.api.com")
	f.InputByID("api_key", "sk-test-1234")
	f.InputByID("tpm", "50000")
	f.InputByID("rpm", "500")

	f.SubmitDialog("create-model-dialog", "Create")
	f.WaitStable()

	// Verify DB has correct tianji_params
	ctx := context.Background()
	dbModel, err := testDB.GetProxyModelByName(ctx, "full-options-model")
	require.NoError(t, err)

	var tp map[string]any
	require.NoError(t, json.Unmarshal(dbModel.TianjiParams, &tp))
	assert.Equal(t, "openai/gpt-4o", tp["model"])
	assert.Equal(t, "https://custom.api.com", tp["api_base"])
	assert.Equal(t, "sk-test-1234", tp["api_key"])
	assert.Equal(t, float64(50000), tp["tpm"])
	assert.Equal(t, float64(500), tp["rpm"])
}

func TestModelCreate_SpecialCharsInName(t *testing.T) {
	f := setup(t)
	f.NavigateToModels()

	f.ClickButton("Add Model")
	f.WaitDialogOpen("create-model-dialog")

	// Name with special characters (potential XSS, unicode, spaces)
	specialName := `test <script>alert(1)</script> model / ñ`
	f.InputByID("model_name", specialName)
	f.InputByID("model", "openai/gpt-4o")
	f.SubmitDialog("create-model-dialog", "Create")
	f.WaitStable()

	// Verify name displayed correctly (HTML-escaped, not XSS)
	body := f.Text("#models-table")
	assert.Contains(t, body, specialName)

	// Verify DB
	ctx := context.Background()
	dbModel, err := testDB.GetProxyModelByName(ctx, specialName)
	require.NoError(t, err)
	assert.Equal(t, specialName, dbModel.ModelName)
}
