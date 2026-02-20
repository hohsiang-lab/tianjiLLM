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

	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newWildcardTestServer(t *testing.T) *proxy.Server {
	t.Helper()
	apiKey := "sk-test-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "openai/*",
				TianjiParams: config.TianjiParams{
					Model:  "openai/*",
					APIKey: &apiKey,
				},
			},
		},
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
	}

	handlers := &handler.Handlers{
		Config: cfg,
	}

	return proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	})
}

func TestWildcard_MatchesAnyModel(t *testing.T) {
	srv := newWildcardTestServer(t)

	body := `{
		"model": "openai/gpt-4o-mini",
		"messages": [{"role": "user", "content": "Hello"}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Should find a matching config via wildcard, not 404
	assert.NotEqual(t, http.StatusNotFound, w.Code,
		"wildcard should match openai/gpt-4o-mini")
}

func TestWildcard_NoMatchForDifferentProvider(t *testing.T) {
	srv := newWildcardTestServer(t)

	body := `{
		"model": "anthropic/claude-3",
		"messages": [{"role": "user", "content": "Hello"}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code,
		"wildcard openai/* should not match anthropic/claude-3")
}

func TestWildcard_ListModelsShowsWildcard(t *testing.T) {
	srv := newWildcardTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 1)
}
