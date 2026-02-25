//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePricingServer returns a httptest.Server that serves a LiteLLM model_prices JSON.
// Automatically pads to 50+ models to pass the sanity check in SyncFromUpstream.
func fakePricingServer(t *testing.T, models map[string]map[string]any) *httptest.Server {
	t.Helper()
	// SyncFromUpstream requires at least 50 models — pad with dummy entries
	padded := make(map[string]map[string]any, len(models)+60)
	for k, v := range models {
		padded[k] = v
	}
	for i := len(padded); i < 60; i++ {
		padded[fmt.Sprintf("pad-model-%d", i)] = map[string]any{
			"input_cost_per_token":  0.000001,
			"output_cost_per_token": 0.000002,
			"max_tokens":            4096,
			"mode":                  "chat",
			"litellm_provider":      "openai",
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(padded)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestModelsSync_HappyPath(t *testing.T) {
	f := setup(t)

	// Spin up fake upstream with 3 models
	upstream := fakePricingServer(t, map[string]map[string]any{
		"gpt-4o": {
			"input_cost_per_token":  0.000005,
			"output_cost_per_token": 0.000015,
			"max_input_tokens":      128000,
			"max_output_tokens":     16384,
			"max_tokens":            128000,
			"mode":                  "chat",
			"litellm_provider":      "openai",
			"source":                "https://openai.com/pricing",
		},
		"claude-3-opus": {
			"input_cost_per_token":  0.000015,
			"output_cost_per_token": 0.000075,
			"max_input_tokens":      200000,
			"max_output_tokens":     4096,
			"max_tokens":            200000,
			"mode":                  "chat",
			"litellm_provider":      "anthropic",
			"source":                "https://anthropic.com/pricing",
		},
		"dall-e-3": {
			"input_cost_per_token":  0,
			"output_cost_per_token": 0,
			"mode":                  "image_generation",
			"litellm_provider":      "openai",
		},
	})

	// Set env for upstream URL
	t.Setenv("PRICING_UPSTREAM_URL", upstream.URL)

	f.NavigateToModels()

	// Click Sync Pricing button
	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Sync Pricing",
	}).Click())

	// Wait for toast
	toastText := f.WaitToast()
	assert.Contains(t, toastText, "Synced")
	assert.Contains(t, toastText, "models successfully")

	// Verify DB has records
	rows, err := testDB.ListModelPricing(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(rows), 3, "expected at least 3 pricing entries")

	// Check specific model
	found := false
	for _, r := range rows {
		if r.ModelName == "gpt-4o" {
			found = true
			assert.InDelta(t, 0.000005, r.InputCostPerToken, 1e-9)
			assert.InDelta(t, 0.000015, r.OutputCostPerToken, 1e-9)
			assert.Equal(t, "openai", r.Provider)
			break
		}
	}
	assert.True(t, found, "gpt-4o pricing not found in DB")
}

func TestModelsSync_Idempotent(t *testing.T) {
	f := setup(t)

	upstream := fakePricingServer(t, map[string]map[string]any{
		"gpt-4o-mini": {
			"input_cost_per_token":  0.00000015,
			"output_cost_per_token": 0.0000006,
			"max_tokens":            128000,
			"mode":                  "chat",
			"litellm_provider":      "openai",
		},
	})
	t.Setenv("PRICING_UPSTREAM_URL", upstream.URL)

	f.NavigateToModels()

	// Sync twice
	for i := 0; i < 2; i++ {
		require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{
			Name: "Sync Pricing",
		}).Click())
		toast := f.Page.Locator("[data-tui-toast]").First()
		require.NoError(t, toast.WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(10000),
		}))
		// Dismiss toast before next iteration
		_, _ = f.Page.Evaluate(`() => document.querySelectorAll('[data-tui-toast]').forEach(el => el.remove())`)
		f.WaitStable()
	}

	// Should still have exactly 1 entry (upsert, not duplicate)
	rows, err := testDB.ListModelPricing(context.Background())
	require.NoError(t, err)

	count := 0
	for _, r := range rows {
		if r.ModelName == "gpt-4o-mini" {
			count++
		}
	}
	assert.Equal(t, 1, count, "expected exactly 1 gpt-4o-mini entry after double sync")
}

func TestModelsSync_UpstreamError(t *testing.T) {
	f := setup(t)

	// Point to a server that returns 500
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "internal error")
	}))
	t.Cleanup(errServer.Close)
	t.Setenv("PRICING_UPSTREAM_URL", errServer.URL)

	f.NavigateToModels()

	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Sync Pricing",
	}).Click())

	toastText := f.WaitToast()
	assert.Contains(t, toastText, "Sync failed")
}

func TestModelsSync_UnreachableUpstream(t *testing.T) {
	f := setup(t)

	// Point to a port that's not listening
	t.Setenv("PRICING_UPSTREAM_URL", "http://127.0.0.1:1")

	f.NavigateToModels()

	require.NoError(t, f.Page.GetByRole("button", playwright.PageGetByRoleOptions{
		Name: "Sync Pricing",
	}).Click())

	toastText := f.WaitToast()
	assert.Contains(t, toastText, "Sync failed")
}
