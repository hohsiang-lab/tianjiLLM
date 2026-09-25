package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// HealthCheck handles GET /health
func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// HealthReadiness handles GET /health/readiness
func (h *Handlers) HealthReadiness(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Check DB connectivity
	if h.DB != nil {
		if err := h.DB.Ping(r.Context()); err != nil {
			h.recordHealthCheck("database", "unhealthy", time.Since(start), err.Error())
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unhealthy",
				"error":  "database unreachable",
			})
			return
		}
		h.recordHealthCheck("database", "healthy", time.Since(start), "")
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

// recordHealthCheck persists a health check result to the DB (fire-and-forget).
func (h *Handlers) recordHealthCheck(modelName, status string, elapsed time.Duration, errMsg string) {
	if h.DB == nil {
		return
	}
	go func() {
		_ = h.DB.InsertHealthCheck(context.Background(), db.InsertHealthCheckParams{
			ModelName:      modelName,
			Status:         status,
			ResponseTimeMs: float64(elapsed.Milliseconds()),
			ErrorMessage:   errMsg,
		})
	}()
}

// HealthLiveness handles GET /health/liveness
func (h *Handlers) HealthLiveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "alive",
	})
}

// HealthServices handles GET /health/services.
func (h *Handlers) HealthServices(w http.ResponseWriter, r *http.Request) {
	services := map[string]string{}

	// Database
	if h.DB != nil {
		if err := h.DB.Ping(r.Context()); err != nil {
			services["database"] = "unhealthy: " + err.Error()
		} else {
			services["database"] = "healthy"
		}
	} else {
		services["database"] = "not_configured"
	}

	// Cache
	if h.Cache != nil {
		testKey := "tianji:health:ping"
		if err := h.Cache.Set(r.Context(), testKey, []byte("ok"), 0); err != nil {
			services["cache"] = "unhealthy: " + err.Error()
		} else {
			ct := "memory"
			if _, ok := h.Cache.(*cache.RedisCache); ok {
				ct = "redis"
			}
			services["cache"] = "healthy (" + ct + ")"
		}
	} else {
		services["cache"] = "not_configured"
	}

	// Callbacks
	if h.Callbacks != nil {
		services["callbacks"] = "configured"
	} else {
		services["callbacks"] = "not_configured"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"services": services,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
