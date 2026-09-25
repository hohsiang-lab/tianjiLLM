package guardrail

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFromConfig_AllModes(t *testing.T) {
	modes := []string{
		"openai_moderation", "presidio", "prompt_injection",
		"lakera_guard", "azure_prompt_shield", "azure_text_moderation",
		"content_filter", "tool_permission", "generic",
		"aim", "aporia", "custom_code", "dynamoai", "enkryptai",
		"grayswan", "guardrails_ai", "hiddenlayer", "ibm_guardrails",
		"javelin", "lakera_v2", "lasso", "model_armor", "noma",
		"onyx", "pangea", "panw_prisma_airs", "pillar",
		"prompt_security", "qualifire", "unified_guardrail",
		"zscaler_ai_guard",
	}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			gc := config.GuardrailConfig{
				GuardrailName: "test-" + mode,
				TianjiParams: map[string]any{
					"mode":     mode,
					"api_key":  "test-key",
					"api_base": "https://example.com",
				},
			}
			g, err := NewFromConfig(gc)
			require.NoError(t, err, "mode: %s", mode)
			assert.NotNil(t, g)
		})
	}
}

func TestNewFromConfig_UnknownMode(t *testing.T) {
	gc := config.GuardrailConfig{
		GuardrailName: "test",
		TianjiParams:  map[string]any{"mode": "nonexistent"},
	}
	_, err := NewFromConfig(gc)
	assert.Error(t, err)
}

func TestNewFromConfig_MissingMode(t *testing.T) {
	gc := config.GuardrailConfig{
		GuardrailName: "test",
		TianjiParams:  map[string]any{},
	}
	_, err := NewFromConfig(gc)
	assert.Error(t, err)
}
