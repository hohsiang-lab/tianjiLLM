package handler

import (
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// CachePing handles GET /cache/ping.
func (h *Handlers) CachePing(w http.ResponseWriter, r *http.Request) {
	if h.Cache == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "cache not configured", Type: "internal_error"},
		})
		return
	}

	// Try a set/get to verify cache works
	ctx := r.Context()
	testKey := "tianji:cache:ping"
	if err := h.Cache.Set(ctx, testKey, []byte("pong"), 0); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"cache":  cacheType(h.Cache),
	})
}

// CacheDelete handles POST /cache/delete.
func (h *Handlers) CacheDelete(w http.ResponseWriter, r *http.Request) {
	if h.Cache == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "cache not configured", Type: "internal_error"},
		})
		return
	}

	var req struct {
		Keys []string `json:"keys"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "invalid request: " + err.Error(), Type: "invalid_request_error"},
		})
		return
	}

	for _, key := range req.Keys {
		_ = h.Cache.Delete(r.Context(), key)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"deleted_keys": len(req.Keys),
	})
}

// CacheFlushAll handles POST /cache/flushall.
func (h *Handlers) CacheFlushAll(w http.ResponseWriter, r *http.Request) {
	if h.Cache == nil {
		writeJSON(w, http.StatusServiceUnavailable, model.ErrorResponse{
			Error: model.ErrorDetail{Message: "cache not configured", Type: "internal_error"},
		})
		return
	}

	// If Redis cache, use FlushDB
	if rc, ok := h.Cache.(*cache.RedisCache); ok {
		if err := rc.Client().FlushDB(r.Context()).Err(); err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
				Error: model.ErrorDetail{Message: "flush failed: " + err.Error(), Type: "internal_error"},
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func cacheType(c cache.Cache) string {
	switch c.(type) {
	case *cache.RedisCache:
		return "redis"
	default:
		return "memory"
	}
}
