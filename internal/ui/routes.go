package ui

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/ui/assets"
)

// RegisterRoutes mounts all UI routes onto a chi subrouter at /ui.
func (h *UIHandler) RegisterRoutes(r chi.Router) {
	// Static assets (no auth)
	staticFS, _ := fs.Sub(assets.Static, ".")
	r.Handle("/static/*", http.StripPrefix("/ui/static/", http.FileServer(http.FS(staticFS))))

	// Login (no auth)
	r.HandleFunc("/login", h.handleLogin)

	// Logout
	r.Post("/logout", h.handleLogout)

	// Protected pages
	r.Group(func(r chi.Router) {
		r.Use(h.sessionAuth)

		r.Get("/", h.handleDashboard)

		// Keys
		r.Get("/keys", h.handleKeys)
		r.Get("/keys/table", h.handleKeysTable)
		r.Post("/keys/create", h.handleKeyCreate)
		r.Post("/keys/delete", h.handleKeyDelete)
		r.Post("/keys/block", h.handleKeyBlock)
		r.Post("/keys/unblock", h.handleKeyUnblock)

		// Key Detail
		r.Get("/keys/{token}", h.handleKeyDetail)
		r.Get("/keys/{token}/edit", h.handleKeyEdit)
		r.Get("/keys/{token}/settings", h.handleKeySettings)
		r.Post("/keys/{token}/update", h.handleKeyUpdate)
		r.Post("/keys/{token}/delete", h.handleKeyDetailDelete)
		r.Post("/keys/{token}/regenerate", h.handleKeyRegenerate)

		// Models
		r.Get("/models", h.handleModels)
		r.Get("/models/table", h.handleModelsTable)
		r.Post("/models/create", h.handleModelCreate)
		r.Get("/models/edit", h.handleModelEdit)
		r.Post("/models/update", h.handleModelUpdate)
		r.Post("/models/delete", h.handleModelDelete)

		// Usage
		r.Get("/usage", h.handleUsage)
		r.Get("/usage/tab", h.handleUsageTab)
		r.Get("/usage/top-keys", h.handleUsageTopKeys)
		r.Get("/usage/export", h.handleUsageExport)

		// Logs
		r.Get("/logs", h.handleLogs)
		r.Get("/logs/table", h.handleLogsTable)
	})
}
