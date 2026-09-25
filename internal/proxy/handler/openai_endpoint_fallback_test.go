package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSupportsOpenAICompatibleProvider(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		want     bool
	}{
		{name: "generic compatibility provider", provider: "openaicompat", want: true},
		{name: "registered standard adapter", provider: "openrouter", want: true},
		{name: "registered Ollama adapter", provider: "ollama", want: true},
		{name: "native provider", provider: "anthropic", want: false},
		{name: "unknown provider", provider: "does-not-exist", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, supportsOpenAICompatibleProvider(tc.provider, config.TianjiParams{}))
		})
	}
}

func TestOpenAIEndpointFallback_UsesFirstCompatibleModel(t *testing.T) {
	restrictedBase := "http://restricted.example"
	publicBase := "http://public.example"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{
		{
			ModelName: "restricted",
			TianjiParams: config.TianjiParams{
				Model:   "openaicompat/restricted",
				APIKey:  stringPtr("[REDACTED]"),
				APIBase: &restrictedBase,
			},
		},
		{
			ModelName: "public",
			TianjiParams: config.TianjiParams{
				Model:   "openaicompat/public",
				APIKey:  stringPtr("[REDACTED]"),
				APIBase: &publicBase,
			},
		},
	}}}
	r := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	route, err := h.resolveOpenAIEndpointRouteForRequestWithOpenRouter(r, false)

	require.NoError(t, err)
	require.Equal(t, restrictedBase, route.Upstream)
}

func TestOpenAIEndpointFallback_DisablesOpenRouter(t *testing.T) {
	openRouterBase := "http://openrouter.example"
	compatBase := "http://compat.example"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{
		{ModelName: "router", TianjiParams: config.TianjiParams{
			Model: "openrouter/route-model", APIKey: stringPtr("[REDACTED]"), APIBase: &openRouterBase,
		}},
		{ModelName: "compat", TianjiParams: config.TianjiParams{
			Model: "openaicompat/compat-model", APIKey: stringPtr("[REDACTED]"), APIBase: &compatBase,
		}},
	}}}
	r := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	route, err := h.resolveOpenAIEndpointRouteForRequestWithOpenRouter(r, false)

	require.NoError(t, err)
	require.Equal(t, compatBase, route.Upstream)
}

func TestAssistantsFallback_DoesNotUseNativeProviderDefaultOpenAIURL(t *testing.T) {
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName: "claude",
		TianjiParams: config.TianjiParams{
			Model:  "anthropic/claude-sonnet",
			APIKey: stringPtr("[REDACTED]"),
		},
	}}}}

	route, err := h.resolveOpenAIEndpointFallbackRoute(context.Background(), false)

	require.NoError(t, err)
	require.Empty(t, route.Upstream)
	require.Empty(t, route.APIKey)
}

func TestOpenAIEndpointRouteForModel_RewritesGenericExplicitBaseModel(t *testing.T) {
	base := "http://compat.example/v1"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName: "public-alias",
		TianjiParams: config.TianjiParams{
			Model:   "openaicompat/upstream-model",
			APIKey:  stringPtr("[REDACTED]"),
			APIBase: &base,
		},
	}}}}

	route, err := h.resolveOpenAIEndpointRouteForModel(context.Background(), "public-alias", false)

	require.NoError(t, err)
	require.Equal(t, "upstream-model", route.ModelName)
}

func TestOpenAIEndpointFallback_DoesNotUseAssistantSettingsWhenOfficialModelExists(t *testing.T) {
	h := newOpenAISubscriptionEndpointHandlers(t)
	legacyBase := "http://legacy-assistant.example"
	h.Config.AssistantSettings = &config.AssistantSettings{APIBase: legacyBase, APIKey: "[REDACTED]"}

	route, err := h.resolveOpenAIEndpointFallbackRoute(context.Background(), false)

	require.NoError(t, err)
	require.NotEqual(t, legacyBase, route.Upstream)
	require.Equal(t, openAISubscriptionTransportChatGPTCodexBackend, route.Transport)
}

func TestFilesList_OfficialOnlyFailsClosed(t *testing.T) {
	h := newOpenAISubscriptionEndpointHandlers(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/files", nil)
	w := httptest.NewRecorder()

	h.FilesList(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "no OpenAI-compatible provider configured")
}

func TestForwardFallback_UsesFirstCompatibleModel(t *testing.T) {
	restrictedBase := "http://restricted.example"
	publicBase := "http://public.example"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{
		{ModelName: "restricted", TianjiParams: config.TianjiParams{
			Model: "openaicompat/restricted", APIKey: stringPtr("[REDACTED]"), APIBase: &restrictedBase,
		}},
		{ModelName: "public", TianjiParams: config.TianjiParams{
			Model: "openaicompat/public", APIKey: stringPtr("[REDACTED]"), APIBase: &publicBase,
		}},
	}}}
	baseURL, _, err := h.resolveProviderBaseURLWithContext(context.Background(), "")

	require.NoError(t, err)
	require.Equal(t, restrictedBase, baseURL)
}

func TestRequestUsesOfficialOpenAIModel_IgnoresLegacyAccessContext(t *testing.T) {
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName:    "restricted-openai",
		TianjiParams: config.TianjiParams{Model: "openai/gpt-4o"},
	}}}}
	require.True(t, h.requestUsesOfficialOpenAIModel("restricted-openai"))
}

func TestFilesList_FallbackUsesConfiguredModel(t *testing.T) {
	var upstreamCalled atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled.Store(true)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer upstream.Close()

	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName:    "restricted-compat",
		TianjiParams: config.TianjiParams{Model: "openaicompat/gpt-4o", APIKey: stringPtr("[REDACTED]"), APIBase: &upstream.URL},
	}}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/files", nil)
	w := httptest.NewRecorder()

	h.FilesList(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, upstreamCalled.Load())
}

func TestForwardFallback_DoesNotUseNativeProviderDefaultOpenAIURL(t *testing.T) {
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName:    "claude",
		TianjiParams: config.TianjiParams{Model: "anthropic/claude-sonnet", APIKey: stringPtr("[REDACTED]")},
	}}}}

	baseURL, _, err := h.resolveProviderBaseURLWithContext(context.Background(), "")

	require.Error(t, err)
	require.Empty(t, baseURL)
}

func TestOpenAIEndpointRouteForModel_RejectsNativeProviderCustomAPIBase(t *testing.T) {
	base := "http://native.example/v1"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName: "claude",
		TianjiParams: config.TianjiParams{
			Model:   "anthropic/claude-sonnet",
			APIKey:  stringPtr("[REDACTED]"),
			APIBase: &base,
		},
	}}}}

	route, err := h.resolveOpenAIEndpointRouteForModel(context.Background(), "claude", false)

	require.NoError(t, err)
	require.Empty(t, route.Upstream)
	require.Empty(t, route.APIKey)
}

func TestOpenAIEndpointFallback_RejectsNativeProviderCustomAPIBase(t *testing.T) {
	base := "http://native.example/v1"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName: "claude",
		TianjiParams: config.TianjiParams{
			Model:   "anthropic/claude-sonnet",
			APIKey:  stringPtr("[REDACTED]"),
			APIBase: &base,
		},
	}}}}

	route, err := h.resolveOpenAIEndpointFallbackRoute(context.Background(), false)

	require.NoError(t, err)
	require.Empty(t, route.Upstream)
	require.Empty(t, route.APIKey)
}

func TestForwardExplicitModel_RejectsNativeProviderCustomAPIBase(t *testing.T) {
	base := "http://native.example/v1"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName: "claude",
		TianjiParams: config.TianjiParams{
			Model:   "anthropic/claude-sonnet",
			APIKey:  stringPtr("[REDACTED]"),
			APIBase: &base,
		},
	}}}}

	baseURL, apiKey, err := h.resolveProviderBaseURLWithContext(context.Background(), "claude")

	require.ErrorContains(t, err, "does not support OpenAI-compatible passthrough")
	require.Empty(t, baseURL)
	require.Empty(t, apiKey)
}

func TestForwardFallback_RejectsNativeProviderCustomAPIBase(t *testing.T) {
	base := "http://native.example/v1"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName: "claude",
		TianjiParams: config.TianjiParams{
			Model:   "anthropic/claude-sonnet",
			APIKey:  stringPtr("[REDACTED]"),
			APIBase: &base,
		},
	}}}}

	baseURL, apiKey, err := h.resolveProviderBaseURLWithContext(context.Background(), "")

	require.Error(t, err)
	require.Empty(t, baseURL)
	require.Empty(t, apiKey)
}

func TestForwardExplicitModel_IgnoresLegacyAccessContext(t *testing.T) {
	base := "http://restricted.example"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName:    "restricted",
		TianjiParams: config.TianjiParams{Model: "openaicompat/restricted", APIKey: stringPtr("[REDACTED]"), APIBase: &base},
	}}}}

	baseURL, _, err := h.resolveProviderBaseURLWithContext(context.Background(), "restricted")

	require.NoError(t, err)
	require.Equal(t, base, baseURL)
}

func TestResolveProviderRoute_IgnoresLegacyAccessContext(t *testing.T) {
	base := "http://restricted.example"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName:    "restricted",
		TianjiParams: config.TianjiParams{Model: "openaicompat/restricted", APIKey: stringPtr("[REDACTED]"), APIBase: &base},
	}}}}

	_, err := h.resolveProviderFromConfigRouteWithContext(context.Background(), "restricted")

	require.NoError(t, err)
}

func TestGenericEndpointRoutesRewriteConfiguredUpstreamModel(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		body      string
		handler   func(*Handlers, http.ResponseWriter, *http.Request)
		wantModel string
	}{
		{name: "responses compact", path: "/v1/responses/compact", body: `{"model":"public-alias","input":"hello"}`, handler: (*Handlers).CompactResponse, wantModel: "upstream-model"},
		{name: "assistants", path: "/v1/assistants", body: `{"model":"public-alias","input":"hello"}`, handler: (*Handlers).AssistantCreate, wantModel: "upstream-model"},
		{name: "thread", path: "/v1/threads", body: `{"metadata":{}}`, handler: (*Handlers).ThreadCreate},
		{name: "message", path: "/v1/threads/thread/messages", body: `{"role":"user","content":"hello"}`, handler: (*Handlers).MessageCreate},
		{name: "run", path: "/v1/threads/thread/runs", body: `{"assistant_id":"assistant"}`, handler: (*Handlers).RunCreate},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var receivedBody string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				receivedBody = string(body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer upstream.Close()

			key := "[REDACTED]"
			modelCfg := config.ModelConfig{
				ModelName: "public-alias",
				TianjiParams: config.TianjiParams{
					Model:   "openaicompat/upstream-model",
					APIKey:  &key,
					APIBase: &upstream.URL,
				},
			}
			h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{modelCfg}}}
			body := tc.body
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			tc.handler(h, w, req)

			require.Less(t, w.Code, http.StatusBadRequest, w.Body.String())
			if tc.wantModel != "" {
				require.Contains(t, receivedBody, `"model":"`+tc.wantModel+`"`)
			} else {
				require.NotContains(t, receivedBody, `"model"`)
			}
		})
	}
}

func TestProxyPassthroughUsesConfiguredModel(t *testing.T) {
	var received atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Store(true)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	key := "[REDACTED]"
	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName:    "restricted",
		TianjiParams: config.TianjiParams{Model: "openaicompat/restricted", APIKey: &key, APIBase: &upstream.URL},
	}}}}
	r := httptest.NewRequest(http.MethodGet, "/v1/files?model=restricted", nil)
	w := httptest.NewRecorder()

	h.proxyPassthrough(w, r, "files")

	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, received.Load())
}
