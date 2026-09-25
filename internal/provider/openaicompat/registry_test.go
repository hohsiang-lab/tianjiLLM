package openaicompat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/ollama"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
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

func TestLoadProviders_PreservesBaseURLConfigurableProvider(t *testing.T) {
	original, err := provider.Get("ollama")
	require.NoError(t, err)
	t.Cleanup(func() {
		provider.Register("ollama", original)
	})

	tmpDir := t.TempDir()
	providersJSON := `{
		"providers": {
			"ollama": {
				"base_url": "http://ollama-from-json.test:11434/v1",
				"auth_header": "X-API-Key",
				"auth_prefix": "Token ",
				"headers": {"X-Custom": "preserved"},
				"supported_params": ["input_type", "dimensions"],
				"param_mappings": {"dimensions": "truncate"},
				"constraints": [{"param": "truncate", "max": 512}]
			}
		}
	}`

	path := filepath.Join(tmpDir, "providers.json")
	require.NoError(t, os.WriteFile(path, []byte(providersJSON), 0644))
	require.NoError(t, LoadProviders(path))

	loaded, err := provider.Get("ollama")
	require.NoError(t, err)
	unwrapper, ok := loaded.(interface{ UnderlyingProvider() provider.Provider })
	require.True(t, ok)
	assert.IsType(t, original, unwrapper.UnderlyingProvider())

	embeddingProvider, ok := loaded.(provider.EmbeddingProvider)
	require.True(t, ok)
	dimensions := 1024
	httpReq, err := embeddingProvider.TransformEmbeddingRequest(context.Background(), &model.EmbeddingRequest{
		Model:      "qwen3-embedding:0.6b",
		Input:      "query text",
		Dimensions: &dimensions,
		ExtraParams: map[string]any{
			"input_type": "query",
		},
	}, "secret")
	require.NoError(t, err)
	assert.Equal(t, "http://ollama-from-json.test:11434/v1/embeddings", httpReq.URL.String())
	assert.Equal(t, "Token secret", httpReq.Header.Get("X-API-Key"))
	assert.Equal(t, "preserved", httpReq.Header.Get("X-Custom"))
	assert.Empty(t, httpReq.Header.Get("Authorization"))
	assert.Equal(t, []string{"input_type", "dimensions"}, loaded.GetSupportedParams())

	body := decodeJSONBody(t, httpReq)
	assert.Equal(t, "Instruct: Given a web search query, retrieve relevant passages that answer the query\nQuery: query text", body["input"])
	assert.Equal(t, float64(512), body["truncate"])
	assert.NotContains(t, body, "dimensions")
	assert.NotContains(t, body, "input_type")
}

func TestConfiguredOpenAIProviderPreservesMaxCompletionTokens(t *testing.T) {
	temperatureMax := 2.0
	configured := newConfiguredProvider(openai.New(), SimpleProviderConfig{
		Constraints: []ParamConstraint{{Param: "temperature", Max: &temperatureMax}},
	})
	maxCompletionTokens := 75

	httpReq, err := configured.TransformRequest(context.Background(), &model.ChatCompletionRequest{
		Model:               "gpt-5",
		Messages:            []model.Message{{Role: "user", Content: "hello"}},
		MaxCompletionTokens: &maxCompletionTokens,
	}, "secret")
	require.NoError(t, err)

	body := decodeJSONBody(t, httpReq)
	assert.Equal(t, float64(75), body["max_completion_tokens"])
	assert.NotContains(t, body, "max_tokens")
}

func TestConfiguredOpenAIProviderReportsRemappedStandardParameters(t *testing.T) {
	configured := newConfiguredProvider(openai.New(), SimpleProviderConfig{
		ParamMappings: map[string]string{
			"response_format": "format",
			"stream_options":  "stream_config",
			"tools":           "functions",
			"tool_choice":     "function_choice",
		},
	})

	preserver, ok := configured.(interface {
		PreservesStandardParameter(string) bool
	})
	require.True(t, ok)
	assert.False(t, preserver.PreservesStandardParameter("response_format"))
	assert.False(t, preserver.PreservesStandardParameter("stream_options"))
	assert.False(t, preserver.PreservesStandardParameter("tools"))
	assert.False(t, preserver.PreservesStandardParameter("tool_choice"))
	assert.True(t, preserver.PreservesStandardParameter("temperature"))
}

func decodeJSONBody(t *testing.T, req *http.Request) map[string]any {
	t.Helper()
	defer req.Body.Close()

	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	return decoded
}
