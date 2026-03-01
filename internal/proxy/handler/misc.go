package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// RoutesList handles GET /routes — returns all registered routes.
// The route list is populated at startup and stored in the Handlers struct.
func (h *Handlers) RoutesList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "see server routes"})
}

// TransformRequest handles POST /transform_request — preview provider transformation.
func (h *Handlers) TransformRequest(w http.ResponseWriter, r *http.Request) {
	var req model.ChatCompletionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	prov, _, apiKey, err := h.resolveProvider(r.Context(), &req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "resolve provider: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	transformed, err := prov.TransformRequest(r.Context(), &req, apiKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "transform: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	body, _ := io.ReadAll(transformed.Body)
	writeJSON(w, http.StatusOK, map[string]any{
		"method":  transformed.Method,
		"url":     transformed.URL.String(),
		"headers": transformed.Header,
		"body":    string(body),
	})
}

// ConfigV2Get handles GET /v2/config — extended config view.
func (h *Handlers) ConfigV2Get(w http.ResponseWriter, r *http.Request) {
	if h.Config == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "no config loaded", Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"model_list":       h.Config.ModelList,
		"general_settings": h.Config.GeneralSettings,
		"tianji_settings":  h.Config.TianjiSettings,
		"router_settings":  h.Config.RouterSettings,
	})
}

// HealthCheckHistory handles GET /health/checks — list recent health check records.
func (h *Handlers) HealthCheckHistory(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	limit := int32(50)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = int32(n)
		}
	}

	checks, err := h.DB.ListHealthChecks(r.Context(), db.ListHealthChecksParams{
		Column1: r.URL.Query().Get("model"),
		Limit:   limit,
		Offset:  0,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list health checks: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": checks})
}

// ErrorLogsList handles GET /errors — list recent error log entries.
func (h *Handlers) ErrorLogsList(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "database not configured", Type: "internal_error"},
		})
		return
	}

	limit := int32(50)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = int32(n)
		}
	}

	logs, err := h.DB.ListErrorLogs(r.Context(), db.ListErrorLogsParams{
		QueryLimit:  limit,
		QueryOffset: 0,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "list error logs: " + err.Error(), Type: "internal_error"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": logs})
}
