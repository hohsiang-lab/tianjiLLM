package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newIntegrationServer(t *testing.T) *proxy.Server {
	t.Helper()
	apiKey := "sk-test"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "gpt-4o",
				TianjiParams: config.TianjiParams{
					Model:  "openai/gpt-4o",
					APIKey: &apiKey,
				},
			},
		},
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
			Port:      4000,
		},
	}

	handlers := &handler.Handlers{Config: cfg}
	return proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	})
}

func TestFullFlow_HealthThenChatCompletion(t *testing.T) {
	srv := newIntegrationServer(t)

	// Step 1: Health check
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Step 2: List models
	req = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var modelsResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &modelsResp))
	assert.Equal(t, "list", modelsResp["object"])

	// Step 3: Chat completion
	body := `{"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}`
	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// May succeed (200) or fail with 502 (no real upstream)
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadGateway)
}

func TestFullFlow_AuthRequired(t *testing.T) {
	srv := newIntegrationServer(t)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/chat/completions"},
		{http.MethodPost, "/v1/embeddings"},
		{http.MethodPost, "/v1/completions"},
		{http.MethodGet, "/v1/models"},
		{http.MethodPost, "/key/generate"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code,
				"expected 401 for %s %s without auth", ep.method, ep.path)
		})
	}
}
