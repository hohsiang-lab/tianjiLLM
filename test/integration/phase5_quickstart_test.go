package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/mcp"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/praxisllmlab/tianjiLLM/internal/router/strategy"
	"github.com/praxisllmlab/tianjiLLM/internal/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Register providers
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	// Register search providers
	_ "github.com/praxisllmlab/tianjiLLM/internal/search"
)

// TestPhase5_MCPRESTToolsList verifies MCP REST tools/list returns tools from manager.
func TestPhase5_MCPRESTToolsList(t *testing.T) {
	mgr := mcp.NewManager()
	mcpServer := mcp.NewMCPServer(mgr)
	mcp.SyncTools(mcpServer, mgr)
	restHandler := (&mcp.RESTHandler{Manager: mgr}).Handler()

	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:       &handler.Handlers{Config: &config.ProxyConfig{}},
		MasterKey:      "sk-master",
		MCPRESTHandler: restHandler,
	})

	req := httptest.NewRequest(http.MethodGet, "/mcp-rest/tools/list", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Tools []any `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotNil(t, body.Tools, "tools should be an array (possibly empty)")
}

// TestPhase5_MCPRESTToolsCallNotFound verifies MCP REST tools/call returns isError for unknown tool.
func TestPhase5_MCPRESTToolsCallNotFound(t *testing.T) {
	mgr := mcp.NewManager()
	restHandler := (&mcp.RESTHandler{Manager: mgr}).Handler()

	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:       &handler.Handlers{Config: &config.ProxyConfig{}},
		MasterKey:      "sk-master",
		MCPRESTHandler: restHandler,
	})

	// Call with nonexistent tool — MCP protocol returns 200 with isError=true in body
	body := `{"name":"nonexistent-tool","arguments":{}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp-rest/tools/call", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer sk-master")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result struct {
		IsError bool `json:"isError"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.True(t, result.IsError, "should have isError=true for nonexistent tool")
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].Text, "not found")
}

// TestPhase5_MCPRESTNoAuth verifies MCP REST requires auth.
func TestPhase5_MCPRESTNoAuth(t *testing.T) {
	mgr := mcp.NewManager()
	restHandler := (&mcp.RESTHandler{Manager: mgr}).Handler()

	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:       &handler.Handlers{Config: &config.ProxyConfig{}},
		MasterKey:      "sk-master",
		MCPRESTHandler: restHandler,
	})

	req := httptest.NewRequest(http.MethodGet, "/mcp-rest/tools/list", nil)
	// No Authorization header
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestPhase5_SearchBraveE2E tests full search round-trip through proxy.
func TestPhase5_SearchBraveE2E(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-brave-key", r.Header.Get("X-Subscription-Token"))
		assert.Contains(t, r.URL.Query().Get("q"), "Go 1.23")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"web":{"results":[{"title":"Go 1.23 Release Notes","url":"https://go.dev/doc/go1.23","description":"Go 1.23 features"}]}}`))
	}))
	defer upstream.Close()

	cfg := &config.ProxyConfig{
		SearchTools: []config.SearchToolConfig{
			{
				SearchToolName: "brave-search",
				TianjiParams: config.SearchToolTianjiParams{
					SearchProvider: "brave",
					APIKey:         "test-brave-key",
					APIBase:        upstream.URL,
				},
			},
		},
	}

	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  &handler.Handlers{Config: cfg},
		MasterKey: "sk-master",
	})

	body := `{"query":"latest Go 1.23 features","max_results":5}`
	req := httptest.NewRequest(http.MethodPost, "/v1/search/brave-search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp search.SearchResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "Go 1.23 Release Notes", resp.Results[0].Title)
	assert.Equal(t, "https://go.dev/doc/go1.23", resp.Results[0].URL)
}

// TestPhase5_ImageVariations tests /v1/images/variations route exists and proxies.
func TestPhase5_ImageVariations(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.True(t, strings.HasPrefix(r.URL.Path, "/v1/images/variations"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"created":1234567890,"data":[{"url":"https://example.com/image.png"}]}`))
	}))
	defer upstream.Close()

	apiKey := "sk-test"
	cfg := &config.ProxyConfig{
		AssistantSettings: &config.AssistantSettings{
			APIBase: upstream.URL,
			APIKey:  apiKey,
		},
	}

	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  &handler.Handlers{Config: cfg},
		MasterKey: "sk-master",
	})

	// Build a simple multipart-like request (reverse proxy forwards as-is)
	body := `--boundary\r\nContent-Disposition: form-data; name="model"\r\n\r\ndall-e-2\r\n--boundary--`
	req := httptest.NewRequest(http.MethodPost, "/v1/images/variations", strings.NewReader(body))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	req.Header.Set("Authorization", "Bearer sk-master")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Created int `json:"created"`
		Data    []struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1234567890, resp.Created)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "https://example.com/image.png", resp.Data[0].URL)
}

// TestPhase5_ImageVariationsNotConfigured tests /v1/images/variations without assistant settings.
func TestPhase5_ImageVariationsNotConfigured(t *testing.T) {
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  &handler.Handlers{Config: &config.ProxyConfig{}},
		MasterKey: "sk-master",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/images/variations", strings.NewReader(""))
	req.Header.Set("Authorization", "Bearer sk-master")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

// TestPhase5_DiscoveryModelGroupInfo tests /model_group/info with router.
func TestPhase5_DiscoveryModelGroupInfo(t *testing.T) {
	models := []config.ModelConfig{
		{
			ModelName:    "gpt-4o",
			TianjiParams: config.TianjiParams{Model: "openai/gpt-4o"},
		},
		{
			ModelName:    "gpt-4o",
			TianjiParams: config.TianjiParams{Model: "openai/gpt-4o-2024-05-13"},
		},
		{
			ModelName:    "claude",
			TianjiParams: config.TianjiParams{Model: "anthropic/claude-sonnet-4-5-20250929"},
		},
	}

	rtr := router.New(models, strategy.NewShuffle(), router.RouterSettings{})
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers: &handler.Handlers{
			Config: &config.ProxyConfig{},
			Router: rtr,
		},
		MasterKey: "sk-master",
	})

	req := httptest.NewRequest(http.MethodGet, "/model_group/info", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data []struct {
			ModelGroup     string   `json:"model_group"`
			Providers      []string `json:"providers"`
			NumDeployments int      `json:"num_deployments"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(t, body.Data, 2, "should have 2 model groups (gpt-4o, claude)")

	// Find gpt-4o group
	var gpt4o bool
	for _, g := range body.Data {
		if g.ModelGroup == "gpt-4o" {
			assert.Equal(t, 2, g.NumDeployments)
			assert.Contains(t, g.Providers, "openai")
			gpt4o = true
		}
	}
	assert.True(t, gpt4o, "should contain gpt-4o model group")
}

// TestPhase5_DiscoveryModelGroupInfoFilter tests ?model_group= filter.
func TestPhase5_DiscoveryModelGroupInfoFilter(t *testing.T) {
	models := []config.ModelConfig{
		{ModelName: "gpt-4o", TianjiParams: config.TianjiParams{Model: "openai/gpt-4o"}},
		{ModelName: "claude", TianjiParams: config.TianjiParams{Model: "anthropic/claude-sonnet-4-5-20250929"}},
	}

	rtr := router.New(models, strategy.NewShuffle(), router.RouterSettings{})
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  &handler.Handlers{Config: &config.ProxyConfig{}, Router: rtr},
		MasterKey: "sk-master",
	})

	req := httptest.NewRequest(http.MethodGet, "/model_group/info?model_group=claude", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data []struct {
			ModelGroup string `json:"model_group"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	assert.Equal(t, "claude", body.Data[0].ModelGroup)
}

// TestPhase5_DiscoveryPublicProviders tests /public/providers (no auth).
func TestPhase5_DiscoveryPublicProviders(t *testing.T) {
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  &handler.Handlers{Config: &config.ProxyConfig{}},
		MasterKey: "sk-master",
	})

	// No auth header — public endpoint
	req := httptest.NewRequest(http.MethodGet, "/public/providers", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Data []string `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, len(body.Data) > 0, "should have at least one provider")

	// Verify sorted
	for i := 1; i < len(body.Data); i++ {
		assert.True(t, body.Data[i-1] <= body.Data[i], "providers not sorted: %s > %s", body.Data[i-1], body.Data[i])
	}
}

// TestPhase5_DiscoveryPublicModelCostMap tests /public/tianji_model_cost_map (no auth).
func TestPhase5_DiscoveryPublicModelCostMap(t *testing.T) {
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  &handler.Handlers{Config: &config.ProxyConfig{}},
		MasterKey: "sk-master",
	})

	req := httptest.NewRequest(http.MethodGet, "/public/tianji_model_cost_map", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, len(body) > 0, "model cost map should not be empty")
}

// TestPhase5_SearchNoAuth tests search requires auth.
func TestPhase5_SearchNoAuth(t *testing.T) {
	cfg := &config.ProxyConfig{
		SearchTools: []config.SearchToolConfig{
			{
				SearchToolName: "brave-search",
				TianjiParams: config.SearchToolTianjiParams{
					SearchProvider: "brave",
					APIKey:         "key",
				},
			},
		},
	}

	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  &handler.Handlers{Config: cfg},
		MasterKey: "sk-master",
	})

	body := `{"query":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/search/brave-search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No auth header
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestPhase5_DiscoveryModelGroupInfoNoAuth tests /model_group/info requires auth.
func TestPhase5_DiscoveryModelGroupInfoNoAuth(t *testing.T) {
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  &handler.Handlers{Config: &config.ProxyConfig{}},
		MasterKey: "sk-master",
	})

	req := httptest.NewRequest(http.MethodGet, "/model_group/info", nil)
	// No auth header
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
