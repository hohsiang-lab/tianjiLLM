package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openrouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateResponse_OpenRouterDefaultRouteForwardsResponses(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotModel string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var payload struct {
			Model string `json:"model"`
		}
		require.NoError(t, json.Unmarshal(body, &payload))
		gotModel = payload.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_test","object":"response","status":"completed"}`))
	}))
	defer upstream.Close()
	target, err := url.Parse(upstream.URL)
	require.NoError(t, err)
	originalTransport := http.DefaultTransport
	http.DefaultTransport = rewriteResponsesHostTransport{
		sourceHost: "openrouter.ai",
		target:     target,
		base:       originalTransport,
	}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	apiKey := "sk-openrouter"
	h := &Handlers{Config: &config.ProxyConfig{
		ModelList: []config.ModelConfig{{
			ModelName: "openrouter/*",
			TianjiParams: config.TianjiParams{
				Model:  "openrouter/*",
				APIKey: &apiKey,
			},
		}},
	}}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"openrouter/free","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/api/v1/responses", gotPath)
	assert.Equal(t, "Bearer sk-openrouter", gotAuth)
	assert.Equal(t, "free", gotModel)
}

func TestCreateResponse_UnknownModelDoesNotFallbackToFirstModel(t *testing.T) {
	var upstreamCalled bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_test","object":"response","status":"completed"}`))
	}))
	defer upstream.Close()
	target, err := url.Parse(upstream.URL)
	require.NoError(t, err)
	originalTransport := http.DefaultTransport
	http.DefaultTransport = rewriteResponsesHostTransport{
		sourceHost: "openrouter.ai",
		target:     target,
		base:       originalTransport,
	}
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	h := &Handlers{Config: &config.ProxyConfig{ModelList: []config.ModelConfig{{
		ModelName: "openrouter/free",
		TianjiParams: config.TianjiParams{
			Model: "openrouter/free",
		},
	}}}}
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"unknown-model","input":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CreateResponse(w, req)

	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	var response struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "model_not_found", response.Error.Code)
	assert.Equal(t, `model "unknown-model" not found`, response.Error.Message)
	assert.False(t, upstreamCalled, "unknown model must not be sent to the first configured model")
}

func TestProxyOpenAIUpstream_DoesNotForwardCallerAuthorizationWithoutAPIKey(t *testing.T) {
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	h := &Handlers{}
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"free"}`))
	req.Header.Set("Authorization", "Bearer caller-credential")
	w := httptest.NewRecorder()

	h.proxyOpenAIUpstream(w, req, upstream.URL+"/api/v1", "", false)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, gotAuth)
}

func TestRewriteOpenAIProxyPath_RootBasePreservesVersionedPath(t *testing.T) {
	target, err := url.Parse("https://example.com/")
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	rewriteOpenAIProxyPath(req, target)

	assert.Equal(t, "/v1/responses", req.URL.Path)
}

func TestResolveOpenAIEndpointRouteForRequest_DoesNotEnableOpenRouterDefaultRoute(t *testing.T) {
	h := &Handlers{Config: &config.ProxyConfig{
		ModelList: []config.ModelConfig{{
			ModelName: "openrouter/*",
			TianjiParams: config.TianjiParams{
				Model: "openrouter/*",
			},
		}},
	}}
	req := httptest.NewRequest(http.MethodPost, "/v1/responses/compact", strings.NewReader(`{"model":"openrouter/free"}`))

	route, err := h.resolveOpenAIEndpointRouteForRequest(req)

	require.NoError(t, err)
	assert.Empty(t, route.Upstream)
}

func TestCompactResponse_UnknownModelReturnsModelNotFound(t *testing.T) {
	h := &Handlers{Config: &config.ProxyConfig{
		ModelList: []config.ModelConfig{{
			ModelName: "openrouter/free",
			TianjiParams: config.TianjiParams{
				Model: "openrouter/free",
			},
		}},
	}}
	req := httptest.NewRequest(http.MethodPost, "/v1/responses/compact", strings.NewReader(`{"model":"unknown-model"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.CompactResponse(w, req)

	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "model_not_found", response.Error.Code)
}

func TestResolveOpenAIEndpointRouteForResponses_RejectsOfficialOpenAICustomBase(t *testing.T) {
	apiBase := "https://example.com/v1"
	h := &Handlers{Config: &config.ProxyConfig{
		ModelList: []config.ModelConfig{{
			ModelName: "openai/*",
			TianjiParams: config.TianjiParams{
				Model:   "openai/*",
				APIBase: &apiBase,
			},
		}},
	}}
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"openai/gpt-4o"}`))

	route, err := h.resolveOpenAIEndpointRouteForResponses(req)

	require.Error(t, err)
	assert.Empty(t, route.ModelName)
}

type rewriteResponsesHostTransport struct {
	sourceHost string
	target     *url.URL
	base       http.RoundTripper
}

func (t rewriteResponsesHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != t.sourceHost {
		return t.base.RoundTrip(req)
	}
	clone := req.Clone(req.Context())
	copiedURL := *clone.URL
	clone.URL = &copiedURL
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	clone.Host = t.target.Host
	return t.base.RoundTrip(clone)
}
