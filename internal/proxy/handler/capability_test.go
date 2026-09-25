package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/guardrail"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/azure"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/codestral"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/databricks"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/github"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/groq"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/mistral"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openaicompat"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openrouter"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capabilityProviderWrapper struct {
	provider.Provider
	remapped map[string]bool
}

func (p capabilityProviderWrapper) UnderlyingProvider() provider.Provider {
	return p.Provider
}

func (p capabilityProviderWrapper) PreservesStandardParameter(param string) bool {
	return !p.remapped[param]
}

type capabilityMutatingGuardrail struct{}

func (capabilityMutatingGuardrail) Name() string { return "inject-temperature" }

func (capabilityMutatingGuardrail) SupportedHooks() []guardrail.Hook {
	return []guardrail.Hook{guardrail.HookPreCall}
}

func (capabilityMutatingGuardrail) Run(_ context.Context, _ guardrail.Hook, req *model.ChatCompletionRequest, _ *model.ModelResponse) (guardrail.Result, error) {
	modified := *req
	temperature := 0.2
	modified.Temperature = &temperature
	return guardrail.Result{Passed: true, ModifiedRequest: &modified}, nil
}

func TestCapabilityForRouteUsesExactInjectedBackendModelKey(t *testing.T) {
	h := &Handlers{Capabilities: model.CapabilityMatrix{
		{Backend: model.BackendDirectOpenAIHTTP, Model: "public-model"}: {
			SupportsStream: true,
		},
	}}
	route := resolvedProviderRoute{
		Provider:    openai.New(),
		PublicModel: "public-model",
		Backend:     model.BackendDirectOpenAIHTTP,
	}

	assert.True(t, h.capabilityForRoute(route).SupportsStream)
	assert.False(t, h.capabilityForRoute(route).SupportsNonStream)
	route.PublicModel = "other-model"
	assert.Equal(t, model.CapabilityRecord{}, h.capabilityForRoute(route))
	route.PublicModel = "public-model"
	route.Backend = model.BackendChatGPTCodex
	assert.Equal(t, model.CapabilityRecord{}, h.capabilityForRoute(route))
}

func TestCapabilityForRouteAppliesNonStreamOverrideToInjectedRecord(t *testing.T) {
	h := &Handlers{Capabilities: model.CapabilityMatrix{
		{Backend: model.BackendDirectOpenAIHTTP, Model: "public-model"}: {
			SupportsStream:    true,
			SupportsNonStream: true,
		},
	}}
	route := resolvedProviderRoute{
		PublicModel: "public-model",
		Backend:     model.BackendDirectOpenAIHTTP,
		TianjiParams: config.TianjiParams{
			Overflow: map[string]any{"supports_non_stream": false},
		},
	}

	record := h.capabilityForRoute(route)
	assert.True(t, record.SupportsStream)
	assert.False(t, record.SupportsNonStream)
}

func TestDefaultCodexCapabilitiesMatchPhaseZeroEvidence(t *testing.T) {
	h := &Handlers{}
	record := h.capabilityForRoute(resolvedProviderRoute{
		Backend:   model.BackendChatGPTCodex,
		ModelName: "openai/gpt-5.6-terra",
	})

	assert.True(t, record.SupportsStream)
	assert.False(t, record.SupportsNonStream)
	assert.True(t, record.SupportsResponseFormat)
	assert.True(t, record.SupportsJSONObject)
	assert.True(t, record.SupportsJSONSchema)
	assert.True(t, record.SupportsTools)
	assert.True(t, record.SupportsToolChoice)
	assert.True(t, record.SupportsStreamOptionsIncludeUsage)
	assert.False(t, record.SupportsTemperature)
	assert.False(t, record.SupportsTopP)
	assert.False(t, record.SupportsMaxTokens)
	assert.False(t, record.SupportsMaxCompletionTokens)
	assert.False(t, record.SupportsEmbeddings)
	assert.False(t, record.SupportsDimensions)
}

func TestCapabilityForRouteAppliesNonStreamOverrideToDefaultCodexRecord(t *testing.T) {
	h := &Handlers{}
	record := h.capabilityForRoute(resolvedProviderRoute{
		Backend:   model.BackendChatGPTCodex,
		ModelName: "openai/gpt-5.6-terra",
		TianjiParams: config.TianjiParams{
			Overflow: map[string]any{"supports_non_stream": true},
		},
	})

	assert.True(t, record.SupportsStream)
	assert.True(t, record.SupportsNonStream)
}

func TestDefaultCapabilitiesUseDirectStandardsAndCodexEvidence(t *testing.T) {
	h := &Handlers{}

	direct := h.capabilityForRoute(resolvedProviderRoute{
		Provider: openai.New(),
		Backend:  model.BackendDirectOpenAIHTTP,
	})
	assert.True(t, direct.SupportsResponseFormat)
	assert.True(t, direct.SupportsJSONObject)
	assert.True(t, direct.SupportsJSONSchema)
	assert.True(t, direct.SupportsNonStream)
	assert.True(t, direct.SupportsMaxCompletionTokens)
	assert.True(t, direct.SupportsStreamOptionsIncludeUsage)

	codex := h.capabilityForRoute(resolvedProviderRoute{
		Backend:   model.BackendChatGPTCodex,
		ModelName: "gpt-5.6-sol",
	})
	assert.True(t, codex.SupportsStream)
	assert.False(t, codex.SupportsResponseFormat)
	assert.False(t, codex.SupportsTools)
	assert.False(t, codex.SupportsStreamOptionsIncludeUsage)
}

func TestDefaultCodexCapabilitiesEnableGraphitiFieldsForVerifiedModels(t *testing.T) {
	h := &Handlers{}
	tests := []struct {
		name  string
		model string
	}{
		{name: "gpt-5.4-mini", model: "openai/gpt-5.4-mini"},
		{name: "gpt-5.6-luna", model: "gpt-5.6-luna"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := h.capabilityForRoute(resolvedProviderRoute{
				Backend:   model.BackendChatGPTCodex,
				ModelName: tt.model,
			})

			assert.True(t, record.SupportsStream)
			assert.False(t, record.SupportsNonStream)
			assert.True(t, record.SupportsResponseFormat)
			assert.True(t, record.SupportsJSONObject)
			assert.True(t, record.SupportsJSONSchema)
			assert.True(t, record.SupportsTemperature)
			assert.True(t, record.SupportsMaxTokens)
			assert.False(t, record.SupportsTopP)
			assert.False(t, record.SupportsTools)
			assert.False(t, record.SupportsToolChoice)
			assert.False(t, record.SupportsMaxCompletionTokens)
			assert.False(t, record.SupportsStreamOptionsIncludeUsage)
		})
	}
}

func TestDefaultCapabilitiesEnableDimensionsOnlyForVerifiedOpenAIEmbeddingModels(t *testing.T) {
	h := &Handlers{}
	tests := []struct {
		name     string
		provider provider.Provider
		model    string
		want     bool
	}{
		{
			name:     "first-party text embedding 3",
			provider: openai.New(),
			model:    "text-embedding-3-small",
			want:     true,
		},
		{
			name:     "older first-party embedding model",
			provider: openai.New(),
			model:    "text-embedding-ada-002",
		},
		{
			name:     "unverified custom endpoint",
			provider: openai.NewWithBaseURL("https://proxy.example.test/v1"),
			model:    "text-embedding-3-small",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := h.capabilityForRoute(resolvedProviderRoute{
				Provider:  tt.provider,
				Backend:   model.BackendDirectOpenAIHTTP,
				ModelName: tt.model,
			})
			assert.Equal(t, tt.want, record.SupportsDimensions)
		})
	}
}

func TestDefaultCapabilitiesDoNotInferDimensionsFromProviderParams(t *testing.T) {
	h := &Handlers{}
	configured := openaicompat.NewFromConfig(openaicompat.SimpleProviderConfig{
		BaseURL:         "https://embeddings.example.test/v1",
		SupportedParams: []string{"dimensions"},
		ParamMappings:   map[string]string{"dimensions": "truncate"},
	})

	record := h.capabilityForRoute(resolvedProviderRoute{
		Provider:  configured,
		Backend:   model.BackendDirectOpenAIHTTP,
		ModelName: "custom-embedding-model",
	})

	assert.False(t, record.SupportsDimensions)
}

func TestDefaultCapabilitiesUseAzureOpenAIStandards(t *testing.T) {
	h := &Handlers{}
	record := h.capabilityForRoute(resolvedProviderRoute{
		Provider: azure.New(),
		Backend:  model.BackendDirectOpenAIHTTP,
	})

	assert.True(t, record.SupportsResponseFormat)
	assert.True(t, record.SupportsJSONObject)
	assert.True(t, record.SupportsJSONSchema)
	assert.True(t, record.SupportsNonStream)
	assert.True(t, record.SupportsMaxCompletionTokens)
	assert.True(t, record.SupportsStreamOptionsIncludeUsage)
}

func TestDefaultCapabilitiesPreserveFirstClassOpenAIAdapters(t *testing.T) {
	h := &Handlers{}
	for name, tc := range map[string]struct {
		provider      provider.Provider
		streamOptions bool
	}{
		"databricks": {provider: &databricks.Provider{Provider: openai.New()}},
		"github": {
			provider:      &github.Provider{Provider: openai.NewWithBaseURL("https://models.inference.ai.azure.com")},
			streamOptions: true,
		},
		"groq": {
			provider:      &groq.Provider{Provider: openai.NewWithBaseURL("https://api.groq.com/openai/v1")},
			streamOptions: true,
		},
		"mistral": {
			provider: &mistral.Provider{
				Provider: openai.NewWithBaseURL("https://api.mistral.ai/v1"),
			},
		},
		"openrouter": {
			provider:      &openrouter.Provider{Provider: openai.NewWithBaseURL("https://openrouter.ai/api/v1")},
			streamOptions: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			record := h.capabilityForRoute(resolvedProviderRoute{
				Provider: tc.provider,
				Backend:  model.BackendDirectOpenAIHTTP,
			})

			assert.True(t, record.SupportsNonStream)
			assert.True(t, record.SupportsResponseFormat)
			assert.True(t, record.SupportsJSONObject)
			assert.True(t, record.SupportsJSONSchema)
			assert.Equal(t, tc.streamOptions, record.SupportsStreamOptionsIncludeUsage)
		})
	}
}

func TestDefaultCapabilitiesFailClosedForUnverifiedOpenAIAdapters(t *testing.T) {
	h := &Handlers{}
	record := h.capabilityForRoute(resolvedProviderRoute{
		Provider: &codestral.Provider{
			Provider: openai.NewWithBaseURL("https://codestral.mistral.ai/v1"),
		},
		Backend: model.BackendDirectOpenAIHTTP,
	})

	assert.False(t, record.SupportsNonStream)
	assert.False(t, record.SupportsJSONObject)
	assert.False(t, record.SupportsJSONSchema)
	assert.False(t, record.SupportsStreamOptionsIncludeUsage)
}

func TestDefaultCapabilitiesFailClosedForOlderAzureAPIVersion(t *testing.T) {
	h := &Handlers{}
	oldVersion := "2024-02-15-preview"
	record := h.capabilityForRoute(resolvedProviderRoute{
		Provider: azure.New(),
		Backend:  model.BackendDirectOpenAIHTTP,
		TianjiParams: config.TianjiParams{
			APIVersion: &oldVersion,
		},
	})

	assert.False(t, record.SupportsNonStream)
	assert.False(t, record.SupportsResponseFormat)
	assert.False(t, record.SupportsJSONObject)
	assert.False(t, record.SupportsJSONSchema)
	assert.False(t, record.SupportsMaxCompletionTokens)
	assert.False(t, record.SupportsStreamOptionsIncludeUsage)
}

func TestDefaultCapabilitiesPreserveWrappedOpenAIStandards(t *testing.T) {
	h := &Handlers{}
	record := h.capabilityForRoute(resolvedProviderRoute{
		Provider: capabilityProviderWrapper{Provider: openai.New()},
		Backend:  model.BackendDirectOpenAIHTTP,
	})

	assert.True(t, record.SupportsJSONObject)
	assert.True(t, record.SupportsJSONSchema)
	assert.True(t, record.SupportsStreamOptionsIncludeUsage)
}

func TestDefaultCapabilitiesFailClosedForRemappedOpenAIRequestShape(t *testing.T) {
	h := &Handlers{}
	record := h.capabilityForRoute(resolvedProviderRoute{
		Provider: capabilityProviderWrapper{
			Provider: openai.New(),
			remapped: map[string]bool{
				"response_format": true,
				"stream_options":  true,
				"tools":           true,
				"tool_choice":     true,
			},
		},
		Backend: model.BackendDirectOpenAIHTTP,
	})

	assert.False(t, record.SupportsJSONObject)
	assert.False(t, record.SupportsJSONSchema)
	assert.False(t, record.SupportsStreamOptionsIncludeUsage)
	assert.False(t, record.SupportsTools)
	assert.False(t, record.SupportsToolChoice)
}

func TestDefaultCapabilitiesFailClosedForCustomOpenAIEndpoints(t *testing.T) {
	h := &Handlers{}
	for name, configured := range map[string]provider.Provider{
		"direct":  openai.NewWithBaseURL("https://proxy.example.test/v1"),
		"wrapped": capabilityProviderWrapper{Provider: openai.NewWithBaseURL("https://proxy.example.test/v1")},
	} {
		t.Run(name, func(t *testing.T) {
			record := h.capabilityForRoute(resolvedProviderRoute{
				Provider: configured,
				Backend:  model.BackendDirectOpenAIHTTP,
			})

			assert.False(t, record.SupportsResponseFormat)
			assert.False(t, record.SupportsJSONObject)
			assert.False(t, record.SupportsJSONSchema)
			assert.False(t, record.SupportsNonStream)
			assert.False(t, record.SupportsMaxCompletionTokens)
			assert.False(t, record.SupportsStreamOptionsIncludeUsage)
		})
	}
}

func TestDefaultCapabilitiesFailClosedForGenericOpenAICompatibleNonStream(t *testing.T) {
	h := &Handlers{}
	route := resolvedProviderRoute{
		Provider: openaicompat.NewFromConfig(openaicompat.SimpleProviderConfig{
			BaseURL:         "https://example.test/v1",
			SupportedParams: []string{"stream"},
		}),
		Backend: model.BackendDirectOpenAIHTTP,
	}

	record := h.capabilityForRoute(route)
	assert.True(t, record.SupportsStream)
	assert.False(t, record.SupportsNonStream)
	assert.False(t, record.SupportsMaxCompletionTokens)

	route.TianjiParams.Overflow = map[string]any{"supports_non_stream": true}
	assert.True(t, h.capabilityForRoute(route).SupportsNonStream)
}

func TestCapabilityForRouteHonorsMappedMaxCompletionTokens(t *testing.T) {
	h := &Handlers{}
	record := h.capabilityForRoute(resolvedProviderRoute{
		Provider: openaicompat.NewFromConfig(openaicompat.SimpleProviderConfig{
			BaseURL:         "https://example.test/v1",
			SupportedParams: []string{"max_tokens"},
			ParamMappings:   map[string]string{"max_completion_tokens": "max_tokens"},
		}),
		Backend: model.BackendDirectOpenAIHTTP,
	})

	assert.True(t, record.SupportsMaxCompletionTokens)
}

func TestCapabilityForRouteKeepsMappedComplexFeaturesFailClosed(t *testing.T) {
	h := &Handlers{}
	record := h.capabilityForRoute(resolvedProviderRoute{
		Provider: openaicompat.NewFromConfig(openaicompat.SimpleProviderConfig{
			BaseURL: "https://example.test/v1",
			SupportedParams: []string{
				"format",
				"stream_config",
				"functions",
				"function_choice",
			},
			ParamMappings: map[string]string{
				"response_format": "format",
				"stream_options":  "stream_config",
				"tools":           "functions",
				"tool_choice":     "function_choice",
			},
		}),
		Backend: model.BackendDirectOpenAIHTTP,
	})

	assert.True(t, record.SupportsResponseFormat)
	assert.False(t, record.SupportsJSONObject)
	assert.False(t, record.SupportsJSONSchema)
	assert.False(t, record.SupportsStreamOptionsIncludeUsage)
	assert.False(t, record.SupportsTools)
	assert.False(t, record.SupportsToolChoice)
}

func TestCapabilityForRouteDoesNotInferStructuredModesFromResponseFormat(t *testing.T) {
	h := &Handlers{}
	record := h.capabilityForRoute(resolvedProviderRoute{
		Provider: openaicompat.NewFromConfig(openaicompat.SimpleProviderConfig{
			BaseURL:         "https://example.test/v1",
			SupportedParams: []string{"response_format"},
		}),
		Backend: model.BackendDirectOpenAIHTTP,
	})

	assert.True(t, record.SupportsResponseFormat)
	assert.False(t, record.SupportsJSONObject)
	assert.False(t, record.SupportsJSONSchema)
}

func TestCapabilityForRouteHonorsConfiguredNonStreamOverride(t *testing.T) {
	h := &Handlers{}
	record := h.capabilityForRoute(resolvedProviderRoute{
		Provider: openai.New(),
		Backend:  model.BackendDirectOpenAIHTTP,
		TianjiParams: config.TianjiParams{
			Overflow: map[string]any{"supports_non_stream": false},
		},
	})

	assert.False(t, record.SupportsNonStream)
	assert.True(t, record.SupportsStream)
}

func TestChatCompletionRevalidatesGuardrailModifiedRequest(t *testing.T) {
	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalled = true
		writeJSON(w, http.StatusOK, model.ModelResponse{
			ID:      "chatcmpl-guardrail",
			Object:  "chat.completion",
			Model:   "gpt-4o",
			Choices: []model.Choice{{Index: 0, Message: &model.Message{Role: "assistant", Content: "ok"}}},
		})
	}))
	defer upstream.Close()

	apiKey := "test-key"
	apiBase := upstream.URL
	registry := guardrail.NewRegistry()
	registry.Register(capabilityMutatingGuardrail{})
	h := &Handlers{
		Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
			ModelName: "guarded-model",
			TianjiParams: config.TianjiParams{
				Model:   "openaicompat/gpt-4o",
				APIKey:  &apiKey,
				APIBase: &apiBase,
			},
		}}},
		Guardrails: registry,
		Capabilities: model.CapabilityMatrix{
			{Backend: model.BackendDirectOpenAIHTTP, Model: "guarded-model"}: {
				SupportsNonStream: true,
			},
		},
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"guarded-model",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	request = request.WithContext(context.WithValue(request.Context(), middleware.ContextKeyGuardrails, []string{"inject-temperature"}))
	recorder := httptest.NewRecorder()

	h.ChatCompletion(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "temperature", response.Error.Param)
	assert.Equal(t, "unsupported_parameter", response.Error.Code)
	assert.False(t, upstreamCalled)
}

func TestValidateChatCapabilities(t *testing.T) {
	stream := true
	temperature := 0.2
	maxTokens := 100
	req := &model.ChatCompletionRequest{
		Stream:      &stream,
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
		ResponseFormat: map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "answer",
				"schema": map[string]any{"type": "object"},
				"strict": true,
			},
		},
		Tools:         []model.Tool{},
		ToolChoice:    "auto",
		StreamOptions: &model.StreamOptions{IncludeUsage: true},
	}
	capability := model.CapabilityRecord{
		SupportsStream:                    true,
		SupportsResponseFormat:            true,
		SupportsJSONSchema:                true,
		SupportsTemperature:               true,
		SupportsMaxTokens:                 true,
		SupportsTools:                     true,
		SupportsToolChoice:                true,
		SupportsStreamOptionsIncludeUsage: true,
	}

	require.Nil(t, validateChatCapabilities(req, capability))

	capability.SupportsTemperature = false
	err := validateChatCapabilities(req, capability)
	require.NotNil(t, err)
	assert.Equal(t, "temperature", err.Detail.Param)
	assert.Equal(t, "unsupported_parameter", err.Detail.Code)
}

func TestValidateChatCapabilitiesRejectsMalformedAndAmbiguousValues(t *testing.T) {
	badTopP := 1.1
	err := validateChatCapabilities(&model.ChatCompletionRequest{TopP: &badTopP}, model.CapabilityRecord{SupportsNonStream: true, SupportsTopP: true})
	require.NotNil(t, err)
	assert.Equal(t, "invalid_value", err.Detail.Code)

	maxTokens, maxCompletionTokens := 10, 20
	err = validateChatCapabilities(
		&model.ChatCompletionRequest{MaxTokens: &maxTokens, MaxCompletionTokens: &maxCompletionTokens},
		model.CapabilityRecord{
			SupportsNonStream:           true,
			SupportsMaxTokens:           true,
			SupportsMaxCompletionTokens: true,
		},
	)
	require.NotNil(t, err)
	assert.Equal(t, "invalid_request", err.Detail.Code)
}

func TestValidateChatCapabilitiesAllowsEmptyToolsWithoutToolSupport(t *testing.T) {
	err := validateChatCapabilities(
		&model.ChatCompletionRequest{Tools: []model.Tool{}},
		model.CapabilityRecord{SupportsNonStream: true},
	)

	require.Nil(t, err)
}

func TestValidateChatCapabilitiesAllowsExplicitTextFormat(t *testing.T) {
	req := &model.ChatCompletionRequest{ResponseFormat: map[string]any{"type": "text"}}
	err := validateChatCapabilities(req, model.CapabilityRecord{SupportsNonStream: true})

	require.Nil(t, err)
	assert.Nil(t, req.ResponseFormat)
}

func TestValidateChatCapabilitiesReportsMalformedResponseFormatBeforeUnsupported(t *testing.T) {
	tests := []struct {
		name   string
		format map[string]any
	}{
		{
			name:   "unknown type",
			format: map[string]any{"type": "xml"},
		},
		{
			name: "malformed json schema",
			format: map[string]any{
				"type":        "json_schema",
				"json_schema": map[string]any{"name": "answer"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateChatCapabilities(
				&model.ChatCompletionRequest{ResponseFormat: tt.format},
				model.CapabilityRecord{SupportsNonStream: true},
			)
			require.NotNil(t, err)
			assert.Equal(t, "response_format", err.Detail.Param)
			assert.Equal(t, "invalid_value", err.Detail.Code)
		})
	}
}

func TestValidateChatCapabilitiesReportsNestedStreamOptionParameter(t *testing.T) {
	stream := true
	err := validateChatCapabilities(
		&model.ChatCompletionRequest{
			Stream:        &stream,
			StreamOptions: &model.StreamOptions{IncludeUsage: true},
		},
		model.CapabilityRecord{SupportsStream: true},
	)

	require.NotNil(t, err)
	assert.Equal(t, "stream_options.include_usage", err.Detail.Param)
	assert.Equal(t, "unsupported_parameter", err.Detail.Code)
}
