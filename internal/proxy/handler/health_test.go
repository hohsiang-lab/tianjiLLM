package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHandlers() *Handlers {
	return &Handlers{
		Config:    &config.ProxyConfig{},
		Callbacks: callback.NewRegistry(),
	}
}

func TestHealthCheck(t *testing.T) {
	h := newTestHandlers()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health", nil)
	h.HealthCheck(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthLiveness(t *testing.T) {
	h := newTestHandlers()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health/liveness", nil)
	h.HealthLiveness(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthReadiness_NoDB(t *testing.T) {
	h := newTestHandlers()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health/readiness", nil)
	h.HealthReadiness(w, r)
	// no DB → should still return a response
	assert.Contains(t, []int{http.StatusOK, http.StatusServiceUnavailable}, w.Code)
}

func TestHealthServices(t *testing.T) {
	h := newTestHandlers()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health/services", nil)
	h.HealthServices(w, r)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp, "services")
}

func TestListModels_Empty(t *testing.T) {
	h := newTestHandlers()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/models", nil)
	h.ListModels(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListModels_WithModels(t *testing.T) {
	h := newTestHandlers()
	h.Config.ModelList = []config.ModelConfig{
		{ModelName: "gpt-4o"},
		{ModelName: "claude-3"},
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/models", nil)
	h.ListModels(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "gpt-4o")
}
