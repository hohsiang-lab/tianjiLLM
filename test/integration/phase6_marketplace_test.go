package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/stretchr/testify/assert"
)

func phase6MarketplaceServer() *proxy.Server {
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{
			GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
		},
	}
	return proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})
}

func TestPhase6_MarketplaceJSON_NoDB(t *testing.T) {
	srv := phase6MarketplaceServer()

	// Marketplace endpoint is public (no auth required)
	req := httptest.NewRequest(http.MethodGet, "/claude-code/marketplace.json", nil)

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_PluginCreate_NoDB(t *testing.T) {
	srv := phase6MarketplaceServer()

	body := `{"name":"test-plugin","version":"1.0.0","description":"A test plugin"}`
	req := httptest.NewRequest(http.MethodPost, "/claude-code/plugins", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_PluginCreate_MissingName_NoDB(t *testing.T) {
	// Without DB, the handler returns 503 before validating input
	srv := phase6MarketplaceServer()

	body := `{"version":"1.0.0"}`
	req := httptest.NewRequest(http.MethodPost, "/claude-code/plugins", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_PluginList_NoDB(t *testing.T) {
	srv := phase6MarketplaceServer()

	req := httptest.NewRequest(http.MethodGet, "/claude-code/plugins", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_PluginEnable_NoDB(t *testing.T) {
	srv := phase6MarketplaceServer()

	req := httptest.NewRequest(http.MethodPost, "/claude-code/plugins/test-plugin/enable", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_PluginDelete_NoDB(t *testing.T) {
	srv := phase6MarketplaceServer()

	req := httptest.NewRequest(http.MethodDelete, "/claude-code/plugins/test-plugin", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
