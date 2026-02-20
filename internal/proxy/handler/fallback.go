package handler

import (
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// fallbackStore holds in-memory fallback configuration.
// In a production deployment this would be persisted to DB.
var (
	fallbackMu    sync.RWMutex
	fallbackStore = map[string]any{}
)

// FallbackCreate handles POST /fallback — set fallback config for a model.
func (h *Handlers) FallbackCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model     string `json:"model"`
		Fallbacks []any  `json:"fallbacks"`
	}
	if err := decodeJSON(r, &req); err != nil || req.Model == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "model required", Type: "invalid_request_error"},
		})
		return
	}

	fallbackMu.Lock()
	fallbackStore[req.Model] = req.Fallbacks
	fallbackMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "model": req.Model})
}

// FallbackGet handles GET /fallback/{model} — get fallback config.
func (h *Handlers) FallbackGet(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model")

	fallbackMu.RLock()
	fb, ok := fallbackStore[modelName]
	fallbackMu.RUnlock()

	if !ok {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "no fallback configured", Type: "not_found"},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"model": modelName, "fallbacks": fb})
}

// FallbackDelete handles DELETE /fallback/{model} — remove fallback config.
func (h *Handlers) FallbackDelete(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model")

	fallbackMu.Lock()
	delete(fallbackStore, modelName)
	fallbackMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "model": modelName})
}
