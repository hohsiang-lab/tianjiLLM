package handler

import (
	"encoding/json"
	"net/http"
)

// RouterSettingsGet handles GET /router/settings — returns current router settings.
func (h *Handlers) RouterSettingsGet(w http.ResponseWriter, r *http.Request) {
	if h.Config.RouterSettings == nil {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, h.Config.RouterSettings)
}

// RouterSettingsPatch handles PATCH /router/settings — updates router settings.
func (h *Handlers) RouterSettingsPatch(w http.ResponseWriter, r *http.Request) {
	if h.Config.RouterSettings == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "router_settings not configured",
		})
		return
	}

	var patch map[string]any
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body: " + err.Error(),
		})
		return
	}

	// Apply known fields
	if v, ok := patch["routing_strategy"].(string); ok {
		h.Config.RouterSettings.RoutingStrategy = v
	}
	if v, ok := patch["num_retries"].(float64); ok {
		n := int(v)
		h.Config.RouterSettings.NumRetries = &n
	}
	if v, ok := patch["allowed_fails"].(float64); ok {
		n := int(v)
		h.Config.RouterSettings.AllowedFails = &n
	}
	if v, ok := patch["cooldown_time"].(float64); ok {
		n := int(v)
		h.Config.RouterSettings.CooldownTime = &n
	}

	writeJSON(w, http.StatusOK, h.Config.RouterSettings)
}
