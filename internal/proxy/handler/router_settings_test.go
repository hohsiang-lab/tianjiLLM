package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestRouterSettingsGet_NilSettings(t *testing.T) {
	h := &Handlers{Config: &config.ProxyConfig{}}
	req := httptest.NewRequest(http.MethodGet, "/router/settings", nil)
	w := httptest.NewRecorder()
	h.RouterSettingsGet(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "{}\n", w.Body.String())
}

func TestRouterSettingsGet_WithSettings(t *testing.T) {
	h := &Handlers{Config: &config.ProxyConfig{
		RouterSettings: &config.RouterSettings{RoutingStrategy: "cost-based"},
	}}
	req := httptest.NewRequest(http.MethodGet, "/router/settings", nil)
	w := httptest.NewRecorder()
	h.RouterSettingsGet(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "cost-based")
}

func TestRouterSettingsPatch_NilSettings(t *testing.T) {
	h := &Handlers{Config: &config.ProxyConfig{}}
	body, _ := json.Marshal(map[string]any{"routing_strategy": "round-robin"})
	req := httptest.NewRequest(http.MethodPatch, "/router/settings", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.RouterSettingsPatch(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRouterSettingsPatch_InvalidBody(t *testing.T) {
	h := &Handlers{Config: &config.ProxyConfig{
		RouterSettings: &config.RouterSettings{},
	}}
	req := httptest.NewRequest(http.MethodPatch, "/router/settings", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	h.RouterSettingsPatch(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRouterSettingsPatch_Success(t *testing.T) {
	retries := 3
	h := &Handlers{Config: &config.ProxyConfig{
		RouterSettings: &config.RouterSettings{
			RoutingStrategy: "round-robin",
			NumRetries:      &retries,
		},
	}}
	body, _ := json.Marshal(map[string]any{
		"routing_strategy": "cost-based",
		"num_retries":      float64(5),
		"allowed_fails":    float64(2),
		"cooldown_time":    float64(30),
	})
	req := httptest.NewRequest(http.MethodPatch, "/router/settings", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.RouterSettingsPatch(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "cost-based", h.Config.RouterSettings.RoutingStrategy)
	assert.Equal(t, 5, *h.Config.RouterSettings.NumRetries)
	assert.Equal(t, 2, *h.Config.RouterSettings.AllowedFails)
	assert.Equal(t, 30, *h.Config.RouterSettings.CooldownTime)
}
