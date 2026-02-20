package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Register search providers
	_ "github.com/praxisllmlab/tianjiLLM/internal/search"
)

func TestSearch_BraveRoundTrip(t *testing.T) {
	// Mock Brave upstream
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-brave-key", r.Header.Get("X-Subscription-Token"))
		assert.Contains(t, r.URL.Query().Get("q"), "golang")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"web":{"results":[{"title":"Go","url":"https://go.dev","description":"Go lang"}]}}`))
	}))
	defer upstream.Close()

	cfg := &config.ProxyConfig{
		GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
		SearchTools: []config.SearchToolConfig{
			{
				SearchToolName: "brave_search",
				TianjiParams: config.SearchToolTianjiParams{
					SearchProvider: "brave",
					APIKey:         "test-brave-key",
					APIBase:        upstream.URL,
				},
			},
		},
	}
	handlers := &handler.Handlers{Config: cfg}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	body := `{"query":"golang","max_results":5}`
	req := httptest.NewRequest(http.MethodPost, "/v1/search/brave_search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp search.SearchResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Len(t, resp.Results, 1)
	assert.Equal(t, "Go", resp.Results[0].Title)
}

func TestSearch_TavilyRoundTrip(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "test query", body["query"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"Result","url":"https://example.com","content":"snippet"}]}`))
	}))
	defer upstream.Close()

	cfg := &config.ProxyConfig{
		GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
		SearchTools: []config.SearchToolConfig{
			{
				SearchToolName: "tavily_search",
				TianjiParams: config.SearchToolTianjiParams{
					SearchProvider: "tavily",
					APIKey:         "tavily-key",
					APIBase:        upstream.URL,
				},
			},
		},
	}
	handlers := &handler.Handlers{Config: cfg}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	body := `{"query":"test query"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/search/tavily_search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSearch_ToolNotFound(t *testing.T) {
	cfg := &config.ProxyConfig{
		GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
	}
	handlers := &handler.Handlers{Config: cfg}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	body := `{"query":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/search/nonexistent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSearch_MissingQuery(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach upstream")
	}))
	defer upstream.Close()

	cfg := &config.ProxyConfig{
		GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
		SearchTools: []config.SearchToolConfig{
			{
				SearchToolName: "brave_search",
				TianjiParams: config.SearchToolTianjiParams{
					SearchProvider: "brave",
					APIKey:         "key",
					APIBase:        upstream.URL,
				},
			},
		},
	}
	handlers := &handler.Handlers{Config: cfg}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	body := `{"max_results":5}`
	req := httptest.NewRequest(http.MethodPost, "/v1/search/brave_search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "query is required")
}
