package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/stretchr/testify/assert"
)

// --- Health tests ---

func TestHealthReadiness_NilDB2(t *testing.T) {
	h := &Handlers{}
	w := httptest.NewRecorder()
	h.HealthReadiness(w, httptest.NewRequest(http.MethodGet, "/health/readiness", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthReadiness_DBHealthy(t *testing.T) {
	h := &Handlers{DB: newMockStore()}
	w := httptest.NewRecorder()
	h.HealthReadiness(w, httptest.NewRequest(http.MethodGet, "/health/readiness", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthServices_AllConfigured(t *testing.T) {
	mc := cache.NewMemoryCache()
	h := &Handlers{DB: newMockStore(), Cache: mc}
	w := httptest.NewRecorder()
	h.HealthServices(w, httptest.NewRequest(http.MethodGet, "/health/services", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

func TestHealthServices_NoDB_NoCache(t *testing.T) {
	h := &Handlers{}
	w := httptest.NewRecorder()
	h.HealthServices(w, httptest.NewRequest(http.MethodGet, "/health/services", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "not_configured")
}

// --- Audit helper tests ---

func TestCreateAuditLog_NilDB(t *testing.T) {
	h := &Handlers{Config: &config.ProxyConfig{}}
	// Should not panic
	h.createAuditLog(context.Background(), "create", "keys", "k1", "admin", "sk-xxx", nil, map[string]string{"a": "b"})
}

func TestCreateAuditLog_Disabled(t *testing.T) {
	h := &Handlers{
		DB:     newMockStore(),
		Config: &config.ProxyConfig{GeneralSettings: config.GeneralSettings{StoreAuditLogs: false}},
	}
	h.createAuditLog(context.Background(), "create", "keys", "k1", "admin", "sk-xxx", nil, nil)
}

func TestCreateAuditLog_Enabled(t *testing.T) {
	h := &Handlers{
		DB:     newMockStore(),
		Config: &config.ProxyConfig{GeneralSettings: config.GeneralSettings{StoreAuditLogs: true}},
	}
	h.createAuditLog(context.Background(), "create", "keys", "k1", "admin", "sk-xxx", map[string]string{"old": "val"}, map[string]string{"new": "val"})
}

func TestDispatchEvent_NilDispatcher(t *testing.T) {
	h := &Handlers{}
	// Should not panic
	h.dispatchEvent(context.Background(), "key.created", "k1", nil)
}

// --- RoutesList ---

func TestRoutesList(t *testing.T) {
	h := &Handlers{}
	w := httptest.NewRecorder()
	h.RoutesList(w, httptest.NewRequest(http.MethodGet, "/routes", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- TokenCount ---

func TestTokenCount_NoModel(t *testing.T) {
	h := &Handlers{}
	body := bytes.NewReader([]byte(`{"text":"hello"}`))
	req := httptest.NewRequest(http.MethodPost, "/utils/token_counter", body)
	w := httptest.NewRecorder()
	h.TokenCount(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTokenCount_Text(t *testing.T) {
	h := &Handlers{}
	body := bytes.NewReader([]byte(`{"model":"gpt-4","text":"hello world"}`))
	req := httptest.NewRequest(http.MethodPost, "/utils/token_counter", body)
	w := httptest.NewRecorder()
	h.TokenCount(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTokenCount_Messages(t *testing.T) {
	h := &Handlers{}
	body := bytes.NewReader([]byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`))
	req := httptest.NewRequest(http.MethodPost, "/utils/token_counter", body)
	w := httptest.NewRecorder()
	h.TokenCount(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
