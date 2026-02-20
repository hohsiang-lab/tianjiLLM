package openaicompat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadProviders(t *testing.T) {
	// Create a temporary providers.json
	tmpDir := t.TempDir()
	providersJSON := `{
		"providers": {
			"groq": {
				"base_url": "https://api.groq.com/openai/v1",
				"supported_params": ["model", "messages", "temperature", "max_tokens", "stream"],
				"param_mappings": {"max_completion_tokens": "max_tokens"},
				"constraints": [{"param": "temperature", "min": 0, "max": 2}]
			},
			"deepseek": {
				"base_url": "https://api.deepseek.com/v1",
				"headers": {"X-Custom": "value"}
			}
		}
	}`

	path := filepath.Join(tmpDir, "providers.json")
	require.NoError(t, os.WriteFile(path, []byte(providersJSON), 0644))

	err := LoadProviders(path)
	require.NoError(t, err)

	// Verify groq registered
	groqProvider, err := provider.Get("groq")
	require.NoError(t, err)
	assert.NotNil(t, groqProvider)
	assert.Contains(t, groqProvider.GetRequestURL("llama-3"), "groq.com")

	// Verify deepseek registered
	deepseekProvider, err := provider.Get("deepseek")
	require.NoError(t, err)
	assert.NotNil(t, deepseekProvider)
	assert.Contains(t, deepseekProvider.GetRequestURL("deepseek-chat"), "deepseek.com")
}

func TestLoadProviders_CustomHeaders(t *testing.T) {
	tmpDir := t.TempDir()
	providersJSON := `{
		"providers": {
			"test_provider": {
				"base_url": "https://api.test.com",
				"auth_header": "X-API-Key",
				"auth_prefix": "",
				"headers": {"X-Custom": "test-value"}
			}
		}
	}`

	path := filepath.Join(tmpDir, "providers.json")
	require.NoError(t, os.WriteFile(path, []byte(providersJSON), 0644))

	err := LoadProviders(path)
	require.NoError(t, err)

	p, err := provider.Get("test_provider")
	require.NoError(t, err)
	assert.NotNil(t, p)
}
