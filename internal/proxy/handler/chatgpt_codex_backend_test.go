package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatGPTCodexBackend_ChatCompletionsAreUnsupported(t *testing.T) {
	var backendCalls atomic.Int32
	var platformCalls atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		backendCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "resp_chat",
			"model":  "gpt-5.5",
			"output": []map[string]any{{"content": []map[string]any{{"type": "output_text", "text": "chat"}}}},
		})
	}))
	defer backend.Close()
	platform := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		platformCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer platform.Close()

	h := newChatGPTCodexBackendHarness(t, backend.URL, platform.URL, "acct_123")
	store, ok := h.DB.(*mockStore)
	require.True(t, ok)
	store.getCredentialFn = func(context.Context, string) (db.CredentialTable, error) {
		return db.CredentialTable{}, errors.New("credential resolution should not run")
	}
	fetcher := &fakeCodexUsageFetcher{}
	h.CodexUsageFetcher = fetcher
	h.CodexUsageCache = NewOpenAISubscriptionCodexUsageCache()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"chatgpt/gpt-5.5",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	w := httptest.NewRecorder()

	h.ChatCompletion(w, req)

	require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "not_supported", response.Error.Type)
	assert.Zero(t, backendCalls.Load())
	assert.Zero(t, platformCalls.Load())
	assert.Empty(t, fetcher.calls)
}

func TestChatGPTCodexBackend_EmbeddingsAreUnsupportedForOfficialAndResolvedBindings(t *testing.T) {
	var backendCalls atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		backendCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tests := []struct {
		name            string
		requestedModel  string
		configuredModel string
		resolvedModel   string
	}{
		{
			name:            "direct official binding",
			requestedModel:  "openai/gpt-5.5",
			configuredModel: "openai/gpt-5.5",
			resolvedModel:   "openai/gpt-5.5",
		},
		{
			name:            "configured alias",
			requestedModel:  "codex-gpt-5.5",
			configuredModel: "codex-gpt-5.5",
			resolvedModel:   "openai/gpt-5.5",
		},
		{
			name:            "configured wildcard",
			requestedModel:  "codex-gpt-5.5",
			configuredModel: "codex-*",
			resolvedModel:   "openai/*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
				"cred-a": {AccessToken: "access-secret", AccountID: "acct_123"},
			})
			h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = backend.URL
			h.Config.ModelList = []config.ModelConfig{{
				ModelName: tt.configuredModel,
				TianjiParams: config.TianjiParams{
					Model:                       tt.resolvedModel,
					OpenAISubscriptionTransport: config.OpenAISubscriptionTransportChatGPTCodexBackend,
				},
			}}
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(`{"model":"`+tt.requestedModel+`","input":"hello"}`))

			h.Embedding(w, req)

			require.Equal(t, http.StatusNotImplemented, w.Code, w.Body.String())
			var response model.ErrorResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			assert.Equal(t, "not_supported", response.Error.Type)
		})
	}

	assert.Zero(t, backendCalls.Load())
}

func TestChatGPTCodexBackend_ChatUnsupportedDoesNotLeakCredentialMaterial(t *testing.T) {
	apiKey := "sk-fallback"
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-disabled": {Status: "disabled"},
	})
	h.Config.ModelList = []config.ModelConfig{{
		ModelName: "chatgpt/gpt-5.5",
		TianjiParams: config.TianjiParams{
			Model:                           "openai/gpt-5.5",
			APIKey:                          &apiKey,
			OpenAISubscriptionCredentialIDs: []string{"missing", "cred-disabled"},
			OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
		},
	}}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"chatgpt/gpt-5.5",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	w := httptest.NewRecorder()

	h.ChatCompletion(w, req)

	require.Equal(t, http.StatusNotImplemented, w.Code)
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "not_supported", response.Error.Type)
	assert.NotContains(t, w.Body.String(), apiKey)
	assert.NotContains(t, w.Body.String(), "credential disabled")
}

func newChatGPTCodexBackendHarness(t *testing.T, backendBaseURL, platformBaseURL, accountID string) *Handlers {
	t.Helper()
	h := newOpenAISubscriptionRoutingHarness(t, map[string]openAISubscriptionTestCredential{
		"cred-a": {AccessToken: "access-secret", AccountID: accountID},
	})
	h.CodexUsageFetcher = &fakeCodexUsageFetcher{}
	h.Config.GeneralSettings.OpenAIOAuth.CodexBackendBaseURL = backendBaseURL
	params := config.TianjiParams{
		Model:                           "openai/gpt-5.5",
		OpenAISubscriptionCredentialIDs: []string{"cred-a"},
		OpenAISubscriptionTransport:     config.OpenAISubscriptionTransportChatGPTCodexBackend,
	}
	if platformBaseURL != "" {
		params.APIBase = &platformBaseURL
	}
	h.Config.ModelList = []config.ModelConfig{{ModelName: "chatgpt/gpt-5.5", TianjiParams: params}}
	enableTestCodexNonStream(h, "chatgpt/gpt-5.5")
	return h
}

func enableTestCodexNonStream(h *Handlers, publicModel string) {
	if h.Capabilities == nil {
		h.Capabilities = make(model.CapabilityMatrix)
	}
	h.Capabilities[model.CapabilityKey{
		Backend: model.BackendChatGPTCodex,
		Model:   publicModel,
	}] = model.CapabilityRecord{
		SupportsStream:                    true,
		SupportsNonStream:                 true,
		SupportsStreamOptionsIncludeUsage: true,
	}
}
