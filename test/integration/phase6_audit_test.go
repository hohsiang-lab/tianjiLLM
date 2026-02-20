package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhase6_AuditLogEndpoint_NoDB(t *testing.T) {
	// Without DB, audit endpoint should return 503
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{
			GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
		},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	req := httptest.NewRequest(http.MethodGet, "/audit", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	errObj, _ := resp["error"].(map[string]any)
	assert.Contains(t, errObj["message"], "database not configured")
}

func TestPhase6_AuditLogGetEndpoint_NoDB(t *testing.T) {
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{
			GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
		},
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	req := httptest.NewRequest(http.MethodGet, "/audit/some-id", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
