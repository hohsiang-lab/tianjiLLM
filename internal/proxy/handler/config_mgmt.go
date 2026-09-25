package handler

import (
	"encoding/json"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// ConfigGet handles GET /config.
func (h *Handlers) ConfigGet(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"general_settings": h.Config.GeneralSettings,
		"tianji_settings":  h.Config.TianjiSettings,
		"model_list":       h.Config.ModelList,
	}

	if h.Config.RouterSettings != nil {
		resp["router_settings"] = h.Config.RouterSettings
	}

	writeJSON(w, http.StatusOK, resp)
}

// ConfigUpdate handles POST /config/update.
func (h *Handlers) ConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Callbacks   []string        `json:"callbacks"`
		PassThrough json.RawMessage `json:"pass_through_endpoints"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	if req.Callbacks != nil {
		h.Config.TianjiSettings.Callbacks = req.Callbacks
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
