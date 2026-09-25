package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/stretchr/testify/assert"
)

func TestOpenAIOAuthRoutes_PublicCallbackDoesNotRequireAPIAuth(t *testing.T) {
	cfg := &config.ProxyConfig{}
	cfg.GeneralSettings.OpenAIOAuth.Enabled = true
	handlers := &handler.Handlers{
		Config: cfg,
		Cache:  cache.NewMemoryCache(),
	}
	server := NewServer(ServerConfig{Handlers: handlers, MasterKey: "test-master-key-32-bytes-long!!!"})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/openai/callback?state=missing", nil)
	server.Router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid")
}
