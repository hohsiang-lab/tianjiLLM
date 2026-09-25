//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// US2 — Create New Model.

func TestModelCreate_FullLifecycle(t *testing.T) {
	f := setup(t)
	f.NavigateToModels()

	// 1. Open create dialog
	f.ClickButton("Add Model")
	f.WaitDialogOpen("create-model-dialog")
	require.Equal(t, 0, f.Count("#create-model-dialog textarea[name=allowed_orgs]"))
	require.Equal(t, 0, f.Count("#create-model-dialog textarea[name=allowed_teams]"))
	require.Equal(t, 0, f.Count("#create-model-dialog textarea[name=allowed_keys]"))
	require.NotContains(t, f.Text("#create-model-dialog"), "Access Control")
	f.InputByID("model_name", "e2e-new-model")
	f.InputByID("model", "anthropic/claude-sonnet-4-5-20250929")

	// 3. Submit — wait for dialog close as reliable success gate.
	f.SubmitDialog("create-model-dialog", "Create")
	f.WaitDialogClose("create-model-dialog")

	// 4. Verify model appears in table
	f.WaitForTextIn("#models-table", "e2e-new-model")
	body := f.Text("#models-table")
	assert.Contains(t, body, "e2e-new-model")
	assert.Contains(t, body, "anthropic")

	// 5. Verify via DB
	ctx := context.Background()
	dbModel := waitForProxyModelByName(t, ctx, "e2e-new-model")
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
	// Use a unique name per test run to avoid cross-run pollution.
	modelName := "full-options-" + generateTestKey()[20:28]
	ctx := context.Background()

	f := setup(t)
	f.NavigateToModels()

	f.ClickButton("Add Model")
	f.WaitDialogOpen("create-model-dialog")

	const dialog = "#create-model-dialog"
	f.InputByIDIn(dialog, "model_name", modelName)
	f.InputByIDIn(dialog, "model", "openaicompat/gpt-4o")
	f.InputByIDIn(dialog, "api_base", "https://custom.api.com")
	f.InputByIDIn(dialog, "api_key", "sk-test-1234")
	f.InputByIDIn(dialog, "tpm", "50000")
	f.InputByIDIn(dialog, "rpm", "500")

	require.Equal(t, modelName, f.InputValueIn(dialog, "model_name"))
	require.Equal(t, "openaicompat/gpt-4o", f.InputValueIn(dialog, "model"))
	require.Equal(t, "https://custom.api.com", f.InputValueIn(dialog, "api_base"))
	require.Equal(t, "sk-test-1234", f.InputValueIn(dialog, "api_key"))
	require.Equal(t, "50000", f.InputValueIn(dialog, "tpm"))
	require.Equal(t, "500", f.InputValueIn(dialog, "rpm"))

	f.SubmitDialog("create-model-dialog", "Create")
	f.WaitDialogClose("create-model-dialog")

	// Verify DB persistence first. It is the durable signal that the HTMX create
	// request succeeded; toast/table rendering can lag briefly on loaded CI.
	dbModel := waitForProxyModelByName(t, ctx, modelName)

	// Then verify visible feedback.
	toast := f.WaitToast()
	require.Contains(t, toast, "created", "expected success toast, got: %s", toast)
	f.WaitForTextIn("#models-table", modelName)

	var tp map[string]any
	require.NoError(t, json.Unmarshal(dbModel.TianjiParams, &tp))
	assert.Equal(t, "openaicompat/gpt-4o", tp["model"])
	assert.Equal(t, "https://custom.api.com", tp["api_base"])
	assert.Equal(t, "sk-test-1234", tp["api_key"])
	assert.Equal(t, float64(50000), tp["tpm"])
	assert.Equal(t, float64(500), tp["rpm"])
	assert.NotContains(t, tp, "openai_subscription_credential_ids")
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
	// Wait for dialog close as reliable success gate.
	f.SubmitDialog("create-model-dialog", "Create")
	f.WaitDialogClose("create-model-dialog")

	// Verify name displayed correctly (HTML-escaped, not XSS)
	body := f.Text("#models-table")
	assert.Contains(t, body, specialName)

	// Verify DB
	ctx := context.Background()
	dbModel := waitForProxyModelByName(t, ctx, specialName)
	assert.Equal(t, specialName, dbModel.ModelName)
}

func waitForProxyModelByName(t *testing.T, ctx context.Context, modelName string) db.ProxyModelTable {
	t.Helper()

	var dbModel db.ProxyModelTable
	require.Eventually(t, func() bool {
		// CI can keep /ui/models/table busy for a few settle cycles after the
		// dialog closes, so bound each DB attempt and keep retrying within a
		// larger overall budget.
		attemptCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		model, err := testDB.GetProxyModelByName(attemptCtx, modelName)
		if err != nil {
			return false
		}
		dbModel = model
		return true
	}, 15*time.Second, 100*time.Millisecond, "expected model %q to be persisted", modelName)

	return dbModel
}

func waitForProxyModelByID(t *testing.T, ctx context.Context, modelID string) db.ProxyModelTable {
	t.Helper()

	var dbModel db.ProxyModelTable
	require.Eventually(t, func() bool {
		attemptCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		model, err := testDB.GetProxyModel(attemptCtx, modelID)
		if err != nil {
			return false
		}
		dbModel = model
		return true
	}, 15*time.Second, 100*time.Millisecond, "expected model %q to be persisted", modelID)

	return dbModel
}
