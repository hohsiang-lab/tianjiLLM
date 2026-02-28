//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AC-4 — UI: Add/Edit Model access control fields & badge display.

func TestModelAccessControl_CreateWithRestriction(t *testing.T) {
	f := setup(t)
	f.NavigateToModels()

	// Open create dialog
	f.ClickButton("Add Model")
	f.WaitDialogOpen("create-model-dialog")

	// Fill required fields
	f.InputByID("model_name", "restricted-model")
	f.InputByID("model", "openai/gpt-4o")

	// Fill access control fields
	require.NoError(t, f.Page.Locator("textarea[name=allowed_orgs]").Fill("org_acme\norg_bigcorp"))
	require.NoError(t, f.Page.Locator("textarea[name=allowed_teams]").Fill("team_ml"))

	// Submit
	f.SubmitDialog("create-model-dialog", "Create")
	f.WaitForTextIn("#models-table", "restricted-model")

	// Verify 🔓 Restricted badge appears in the table
	row := f.Page.Locator("#models-table table tbody tr").First()
	rowText, err := row.TextContent()
	require.NoError(t, err)
	assert.Contains(t, rowText, "Restricted")

	// Verify DB has access_control in model_info
	ctx := context.Background()
	dbModel, err := testDB.GetProxyModelByName(ctx, "restricted-model")
	require.NoError(t, err)

	var info map[string]any
	require.NoError(t, json.Unmarshal(dbModel.ModelInfo, &info))
	ac, ok := info["access_control"].(map[string]any)
	require.True(t, ok, "model_info should contain access_control")

	orgs := toStringSliceFromAny(ac["allowed_orgs"])
	teams := toStringSliceFromAny(ac["allowed_teams"])
	assert.Equal(t, []string{"org_acme", "org_bigcorp"}, orgs)
	assert.Equal(t, []string{"team_ml"}, teams)
}

func TestModelAccessControl_CreatePublic(t *testing.T) {
	f := setup(t)
	f.NavigateToModels()

	f.ClickButton("Add Model")
	f.WaitDialogOpen("create-model-dialog")

	f.InputByID("model_name", "public-model")
	f.InputByID("model", "openai/gpt-4o")
	// Leave access control fields empty

	f.SubmitDialog("create-model-dialog", "Create")
	f.WaitForTextIn("#models-table", "public-model")

	// Should NOT show Restricted badge
	row := f.Page.Locator("#models-table table tbody tr").First()
	rowText, err := row.TextContent()
	require.NoError(t, err)
	assert.NotContains(t, rowText, "Restricted")
}

func TestModelAccessControl_EditAddRestriction(t *testing.T) {
	f := setup(t)
	// Seed a public model
	modelID := f.SeedModel(SeedModelOpts{ModelName: "soon-restricted"})
	f.NavigateToModels()

	// Click Edit
	f.Page.Locator("#models-table table tbody tr").First().GetByRole("button", playwright.LocatorGetByRoleOptions{
		Name: "Edit",
	}).Click()
	f.WaitDialogOpen("edit-model-dialog")

	// Add access control
	require.NoError(t, f.Page.Locator("#edit_allowed_orgs").Fill("org_exclusive"))

	f.SubmitDialog("edit-model-dialog", "Save Changes")
	f.WaitStable()

	// Verify badge in table
	row := f.Page.Locator("#models-table table tbody tr").First()
	rowText, err := row.TextContent()
	require.NoError(t, err)
	assert.Contains(t, rowText, "Restricted")

	// Verify DB
	ctx := context.Background()
	dbModel, err := testDB.GetProxyModel(ctx, modelID)
	require.NoError(t, err)

	var info map[string]any
	require.NoError(t, json.Unmarshal(dbModel.ModelInfo, &info))
	ac, ok := info["access_control"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"org_exclusive"}, toStringSliceFromAny(ac["allowed_orgs"]))
}

func TestModelAccessControl_EditRemoveRestriction(t *testing.T) {
	f := setup(t)
	// Seed a restricted model
	f.SeedModel(SeedModelOpts{
		ModelName: "unrestrict-me",
		Extra:     map[string]any{},
	})
	// Manually set model_info with access_control
	ctx := context.Background()
	dbModel, err := testDB.GetProxyModelByName(ctx, "unrestrict-me")
	require.NoError(t, err)
	info := map[string]any{"access_control": map[string]any{"allowed_orgs": []string{"org_a"}}}
	infoJSON, _ := json.Marshal(info)
	_, err = testPool.Exec(ctx, `UPDATE "ProxyModelTable" SET model_info = $1 WHERE model_id = $2`, infoJSON, dbModel.ModelID)
	require.NoError(t, err)

	f.NavigateToModels()

	// Verify Restricted badge before edit
	row := f.Page.Locator("#models-table table tbody tr").First()
	rowText, err := row.TextContent()
	require.NoError(t, err)
	assert.Contains(t, rowText, "Restricted")

	// Click Edit
	row.GetByRole("button", playwright.LocatorGetByRoleOptions{Name: "Edit"}).Click()
	f.WaitDialogOpen("edit-model-dialog")

	// Verify prefilled value
	val, err := f.Page.Locator("#edit_allowed_orgs").InputValue()
	require.NoError(t, err)
	assert.Contains(t, val, "org_a")

	// Clear all access control
	require.NoError(t, f.Page.Locator("#edit_allowed_orgs").Fill(""))
	require.NoError(t, f.Page.Locator("#edit_allowed_teams").Fill(""))
	require.NoError(t, f.Page.Locator("#edit_allowed_keys").Fill(""))

	f.SubmitDialog("edit-model-dialog", "Save Changes")
	f.WaitStable()

	// Badge should be gone
	row = f.Page.Locator("#models-table table tbody tr").First()
	rowText, err = row.TextContent()
	require.NoError(t, err)
	assert.NotContains(t, rowText, "Restricted")
}

func TestModelAccessControl_EditPreservesAccessControl(t *testing.T) {
	f := setup(t)
	f.SeedModel(SeedModelOpts{ModelName: "keep-ac"})
	ctx := context.Background()
	dbModel, _ := testDB.GetProxyModelByName(ctx, "keep-ac")
	info := map[string]any{"access_control": map[string]any{
		"allowed_orgs":  []string{"org_a", "org_b"},
		"allowed_teams": []string{"team_x"},
		"allowed_keys":  []string{"sk-hash-123"},
	}}
	infoJSON, _ := json.Marshal(info)
	testPool.Exec(ctx, `UPDATE "ProxyModelTable" SET model_info = $1 WHERE model_id = $2`, infoJSON, dbModel.ModelID)

	f.NavigateToModels()
	f.Page.Locator("#models-table table tbody tr").First().GetByRole("button", playwright.LocatorGetByRoleOptions{Name: "Edit"}).Click()
	f.WaitDialogOpen("edit-model-dialog")

	// Verify all three fields are prefilled
	orgsVal, _ := f.Page.Locator("#edit_allowed_orgs").InputValue()
	teamsVal, _ := f.Page.Locator("#edit_allowed_teams").InputValue()
	keysVal, _ := f.Page.Locator("#edit_allowed_keys").InputValue()
	assert.Contains(t, orgsVal, "org_a")
	assert.Contains(t, orgsVal, "org_b")
	assert.Contains(t, teamsVal, "team_x")
	// Keys are masked in the edit form (P1 security fix) — textarea should be empty
	assert.Empty(t, keysVal, "allowed_keys textarea should be empty (masked)")
	// Placeholder should indicate keys are configured
	keysPlaceholder, _ := f.Page.Locator("#edit_allowed_keys").GetAttribute("placeholder")
	assert.Contains(t, keysPlaceholder, "1 key(s) configured")

	// Edit only the model name, leave AC unchanged
	require.NoError(t, f.Page.Locator("#edit_model_name").Clear())
	f.InputByID("edit_model_name", "renamed-keep-ac")
	f.SubmitDialog("edit-model-dialog", "Save Changes")
	f.WaitStable()

	// Verify AC preserved in DB
	dbModel2, _ := testDB.GetProxyModelByName(ctx, "renamed-keep-ac")
	var info2 map[string]any
	json.Unmarshal(dbModel2.ModelInfo, &info2)
	ac := info2["access_control"].(map[string]any)
	assert.Equal(t, []string{"org_a", "org_b"}, toStringSliceFromAny(ac["allowed_orgs"]))
	assert.Equal(t, []string{"team_x"}, toStringSliceFromAny(ac["allowed_teams"]))
	assert.Equal(t, []string{"sk-hash-123"}, toStringSliceFromAny(ac["allowed_keys"]))
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func toStringSliceFromAny(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		// Try direct []string (from Go json unmarshal with typed struct)
		if ss, ok := v.([]string); ok {
			return ss
		}
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// Unused import guard
var _ = strings.Join
