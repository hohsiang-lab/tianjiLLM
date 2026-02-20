package openai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapOpenAIParams_MaxCompletionTokens(t *testing.T) {
	params := map[string]any{
		"max_completion_tokens": 1000,
		"temperature":           0.7,
	}

	result := MapOpenAIParams(params)
	assert.Equal(t, 1000, result["max_tokens"])
	assert.Equal(t, 0.7, result["temperature"])
	assert.NotContains(t, result, "max_completion_tokens")
}

func TestMapOpenAIParams_NoMapping(t *testing.T) {
	params := map[string]any{
		"temperature": 0.5,
		"top_p":       0.9,
	}

	result := MapOpenAIParams(params)
	assert.Equal(t, 0.5, result["temperature"])
	assert.Equal(t, 0.9, result["top_p"])
}

func TestFilterUnsupportedParams(t *testing.T) {
	params := map[string]any{
		"model":       "gpt-4o",
		"messages":    []any{},
		"temperature": 0.7,
		"unsupported": "value",
		"custom_flag": true,
	}

	result := FilterUnsupportedParams(params, SupportedParams)
	assert.Contains(t, result, "model")
	assert.Contains(t, result, "messages")
	assert.Contains(t, result, "temperature")
	assert.NotContains(t, result, "unsupported")
	assert.NotContains(t, result, "custom_flag")
}
