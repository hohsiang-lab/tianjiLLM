//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamModels_AddModel(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "model-add-team"})
	f.NavigateToTeamDetail(teamID)

	// Select a model from dropdown and add
	_, err := f.Page.Locator(`#team-models-section select[name="model_name"]`).SelectOption(playwright.SelectOptionValues{
		Values: &[]string{"gpt-4o"},
	})
	require.NoError(t, err)
	f.ClickButtonIn("#team-models-section", "Add")
	f.WaitStable()

	// Model should appear in the models section
	assert.Contains(t, f.Text("#team-models-section"), "gpt-4o")
}

func TestTeamModels_RemoveModel(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{
		Alias:  "model-remove-team",
		Models: []string{"gpt-4o"},
	})
	f.NavigateToTeamDetail(teamID)

	// Verify model is present
	assert.Contains(t, f.Text("#team-models-section"), "gpt-4o")

	// Click remove on the model (the × button next to the model badge)
	require.NoError(t, f.Page.Locator(`#team-models-section form[hx-post*="models/remove"] button`).Click())
	f.WaitStable()

	// Model badge should be gone — check that it shows "Inherited / All models" now
	body := f.Text("#team-models-section")
	assert.True(t, strings.Contains(body, "Inherited") || strings.Contains(body, "All models"),
		"After removing last model, should show inherited/all models")
}

func TestTeamModels_EmptyShowsInherited(t *testing.T) {
	f := setup(t)
	teamID := f.SeedTeam(SeedTeamOpts{Alias: "no-model-team"})
	f.NavigateToTeamDetail(teamID)

	body := f.Text("#team-models-section")
	assert.True(t, strings.Contains(body, "All models") || strings.Contains(body, "Inherited"),
		"Empty models should show inherited label")
}
