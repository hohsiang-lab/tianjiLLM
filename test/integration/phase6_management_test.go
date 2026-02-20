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

func phase6MgmtServer() *proxy.Server {
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

func TestPhase6_KeyAliases_NoDB(t *testing.T) {
	srv := phase6MgmtServer()

	req := httptest.NewRequest(http.MethodGet, "/key/aliases", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_KeyInfoV2_NoDB(t *testing.T) {
	srv := phase6MgmtServer()

	body := `{"keys":["hash1","hash2"]}`
	req := httptest.NewRequest(http.MethodPost, "/v2/key/info", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_ServiceAccountKey_NoDB(t *testing.T) {
	srv := phase6MgmtServer()

	body := `{"key_alias":"svc-key","team_id":"team-1"}`
	req := httptest.NewRequest(http.MethodPost, "/key/service-account/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_TeamAvailable_NoDB(t *testing.T) {
	srv := phase6MgmtServer()

	req := httptest.NewRequest(http.MethodGet, "/team/available", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_TeamPermissions_NoDB(t *testing.T) {
	srv := phase6MgmtServer()

	req := httptest.NewRequest(http.MethodGet, "/team/permissions?team_id=test-team", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_GlobalActivity_NoDB(t *testing.T) {
	srv := phase6MgmtServer()

	req := httptest.NewRequest(http.MethodGet, "/global/activity", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_GlobalSpendReset_NoDB(t *testing.T) {
	srv := phase6MgmtServer()

	req := httptest.NewRequest(http.MethodPost, "/global/spend/reset", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_CacheHitStats_NoDB(t *testing.T) {
	srv := phase6MgmtServer()

	req := httptest.NewRequest(http.MethodGet, "/global/activity/cache_hits", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_GlobalSpendReport_NoDB(t *testing.T) {
	srv := phase6MgmtServer()

	req := httptest.NewRequest(http.MethodGet, "/global/spend/report?group_by=team", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
