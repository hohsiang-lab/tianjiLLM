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
		r.Post("/models/sync-pricing", h.handleSyncPricing)

		// Usage
		r.Get("/usage", h.handleUsage)
		r.Get("/usage/tab", h.handleUsageTab)
		r.Get("/usage/top-keys", h.handleUsageTopKeys)
		r.Get("/usage/export", h.handleUsageExport)

		// Teams
		r.Get("/teams", h.handleTeams)
		r.Get("/teams/table", h.handleTeamsTable)
		r.Post("/teams/create", h.handleTeamCreate)

		// Team Detail (must come before /{team_id}/block etc.)
		r.Get("/teams/{team_id}", h.handleTeamDetail)
		r.Post("/teams/{team_id}/update", h.handleTeamUpdate)
		r.Post("/teams/{team_id}/members/add", h.handleTeamMemberAdd)
		r.Post("/teams/{team_id}/members/remove", h.handleTeamMemberRemove)
		r.Post("/teams/{team_id}/models/add", h.handleTeamModelAdd)
		r.Post("/teams/{team_id}/models/remove", h.handleTeamModelRemove)

		r.Post("/teams/{team_id}/block", h.handleTeamBlock)
		r.Post("/teams/{team_id}/unblock", h.handleTeamUnblock)
		r.Post("/teams/{team_id}/delete", h.handleTeamDelete)

		// Organizations
		r.Get("/orgs", h.handleOrgs)
		r.Get("/orgs/table", h.handleOrgsTable)
		r.Post("/orgs/create", h.handleOrgCreate)

		// Org Detail
		r.Get("/orgs/{org_id}", h.handleOrgDetail)
		r.Post("/orgs/{org_id}/update", h.handleOrgUpdate)
		r.Post("/orgs/{org_id}/delete", h.handleOrgDelete)
		r.Post("/orgs/{org_id}/members/add", h.handleOrgMemberAdd)
		r.Post("/orgs/{org_id}/members/update", h.handleOrgMemberUpdate)
		r.Post("/orgs/{org_id}/members/remove", h.handleOrgMemberRemove)

		// Logs
		r.Get("/logs", h.handleLogs)
		r.Get("/logs/table", h.handleLogsTable)
	})
}
