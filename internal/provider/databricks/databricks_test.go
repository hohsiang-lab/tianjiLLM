package databricks

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistered(t *testing.T) {
	p, err := provider.Get("databricks")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetSupportedParams(t *testing.T) {
	p, _ := provider.Get("databricks")
	dp, _ := p.(*Provider)
	params := dp.GetSupportedParams()
	assert.Contains(t, params, "response_format")
	assert.Contains(t, params, "tools")
}

func TestMapParams_MaxCompletionTokens(t *testing.T) {
	p, _ := provider.Get("databricks")
	dp, _ := p.(*Provider)
	result := dp.MapParams(map[string]any{
		"max_completion_tokens": 500,
		"temperature":           0.5,
	})
	assert.Equal(t, 500, result["max_tokens"])
	assert.Equal(t, 0.5, result["temperature"])
	assert.NotContains(t, result, "max_completion_tokens")
}
