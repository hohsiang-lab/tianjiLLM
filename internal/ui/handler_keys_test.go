package ui

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
)

// TestLoadAvailableModelNames covers the config-only (DB nil) path.
func TestLoadAvailableModelNames_ConfigOnly(t *testing.T) {
	h := &UIHandler{
		DB: nil,
		Config: &config.ProxyConfig{
			ModelList: []config.ModelConfig{
				{ModelName: "gpt-4"},
				{ModelName: "claude-3"},
				{ModelName: "gemini-pro"},
			},
		},
	}

	names := h.loadAvailableModelNames(context.Background())

	assert.Equal(t, []string{"gpt-4", "claude-3", "gemini-pro"}, names)
}

// TestLoadAvailableModelNames_DBNil_ConfigNil verifies that nil config + nil DB
// returns an empty (non-nil) slice.
func TestLoadAvailableModelNames_DBNil_ConfigNil(t *testing.T) {
	h := &UIHandler{
		DB:     nil,
		Config: nil,
	}

	names := h.loadAvailableModelNames(context.Background())

	assert.NotNil(t, names, "should return non-nil slice")
	assert.Equal(t, []string{}, names)
}

// TestLoadAvailableModelNames_ConfigDeduplication ensures duplicate ModelName
// entries in config are deduplicated (only first occurrence kept).
func TestLoadAvailableModelNames_ConfigDeduplication(t *testing.T) {
	h := &UIHandler{
		DB: nil,
		Config: &config.ProxyConfig{
			ModelList: []config.ModelConfig{
				{ModelName: "gpt-4"},
				{ModelName: "claude-3"},
				{ModelName: "gpt-4"}, // duplicate
			},
		},
	}

	names := h.loadAvailableModelNames(context.Background())

	// Should have exactly 2 unique entries, gpt-4 deduplicated
	assert.Equal(t, []string{"gpt-4", "claude-3"}, names)
}

// TestLoadAvailableModelNamesEmpty verifies that both DB nil and empty config
// returns []string{} (not nil).
func TestLoadAvailableModelNamesEmpty(t *testing.T) {
	h := &UIHandler{
		DB: nil,
		Config: &config.ProxyConfig{
			ModelList: []config.ModelConfig{},
		},
	}

	names := h.loadAvailableModelNames(context.Background())

	assert.NotNil(t, names, "should return non-nil slice even when empty")
	assert.Equal(t, []string{}, names)
	assert.Len(t, names, 0)
}

// TestLoadAvailableModelNames_EmptyModelName ensures zero-value model names
// are filtered out and not included in the output.
func TestLoadAvailableModelNames_EmptyModelName(t *testing.T) {
	h := &UIHandler{
		DB: nil,
		Config: &config.ProxyConfig{
			ModelList: []config.ModelConfig{
				{ModelName: "gpt-4"},
				{ModelName: ""}, // empty name — must be filtered out
				{ModelName: ""}, // empty name — must be filtered out
				{ModelName: "claude-3"},
			},
		},
	}

	names := h.loadAvailableModelNames(context.Background())

	// Empty string must be filtered out → only 2 names remain.
	assert.Len(t, names, 2)
	assert.Contains(t, names, "gpt-4")
	assert.Contains(t, names, "claude-3")
	assert.NotContains(t, names, "")
}

// TestParseModelSelection covers the pure model-selection parsing logic.
func TestParseModelSelection(t *testing.T) {
	tests := []struct {
		name       string
		allModels  string
		formModels []string
		want       []string
	}{
		{
			name:       "all_models=1 returns empty (unrestricted)",
			allModels:  "1",
			formModels: []string{"gpt-4", "claude-3"},
			want:       []string{},
		},
		{
			name:       "specific models selected",
			allModels:  "0",
			formModels: []string{"gpt-4", "claude-3"},
			want:       []string{"gpt-4", "claude-3"},
		},
		{
			name:       "nil models fallback treats as unrestricted",
			allModels:  "0",
			formModels: nil,
			want:       []string{},
		},
		{
			name:       "all_models=0 with empty slice treats as unrestricted",
			allModels:  "0",
			formModels: []string{},
			want:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseModelSelection(tt.allModels, tt.formModels)
			assert.Equal(t, tt.want, got)
		})
	}
}
