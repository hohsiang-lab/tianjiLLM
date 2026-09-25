package ui

import (
	"io/fs"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/praxisllmlab/tianjiLLM/internal/ui/assets"
)

// parsePage parses the page query param. Returns 1 for any invalid or out-of-range value.
func parsePage(s string) int {
	if s == "" {
		return 1
	}
	p, err := strconv.Atoi(s)
	if err != nil || p < 1 {
		return 1
	}
	return p
}

// RegisterRoutes mounts all UI routes onto a chi subrouter at /ui.
func (h *UIHandler) RegisterRoutes(r chi.Router) {
	r.Use(h.getSessionManager().LoadAndSave)

	// Static assets (no auth)
	staticFS, _ := fs.Sub(assets.Static, ".")
	r.Handle("/static/*", http.StripPrefix("/ui/static/", http.FileServer(http.FS(staticFS))))

	// Login (no auth)
	r.Get("/login", h.handleLogin)
	r.Post("/login", h.handleLogin)

	// Logout
	r.Post("/logout", h.handleLogout)

	// Protected pages
	r.Group(func(r chi.Router) {
		r.Use(h.sessionAuth)

		r.With(h.requirePermission(PermissionDashboardRead)).Get("/", h.handleDashboard)

		// Keys
		r.With(h.requirePermission(PermissionKeysRead)).Get("/keys", h.handleKeys)
		r.With(h.requirePermission(PermissionKeysRead)).Get("/keys/table", h.handleKeysTable)
		r.With(h.requirePermission(PermissionKeysManage)).Post("/keys/create", h.handleKeyCreate)
		r.With(h.requirePermission(PermissionKeysManage)).Post("/keys/delete", h.handleKeyDelete)
		r.With(h.requirePermission(PermissionKeysManage)).Post("/keys/block", h.handleKeyBlock)
		r.With(h.requirePermission(PermissionKeysManage)).Post("/keys/unblock", h.handleKeyUnblock)

		// Key Detail
		r.With(h.requirePermission(PermissionKeysRead)).Get("/keys/{token}", h.handleKeyDetail)
		r.With(h.requirePermission(PermissionKeysManage)).Get("/keys/{token}/edit", h.handleKeyEdit)
		r.With(h.requirePermission(PermissionKeysManage)).Get("/keys/{token}/settings", h.handleKeySettings)
		r.With(h.requirePermission(PermissionKeysManage)).Post("/keys/{token}/update", h.handleKeyUpdate)
		r.With(h.requirePermission(PermissionKeysManage)).Post("/keys/{token}/delete", h.handleKeyDetailDelete)
		r.With(h.requirePermission(PermissionKeysManage)).Post("/keys/{token}/regenerate", h.handleKeyRegenerate)

		// Credentials
		r.With(h.requirePermission(PermissionCredentialsManage)).Get("/credentials", h.handleCredentials)
		r.With(h.requirePermission(PermissionCredentialsManage)).Get("/credentials/{credential_id}", h.handleCredentialDetail)
		r.With(h.requirePermission(PermissionCredentialsManage)).Post("/credentials/{credential_id}/test", h.handleCredentialTest)
		r.With(h.requirePermission(PermissionCredentialsManage)).Post("/credentials/{credential_id}/refresh", h.handleCredentialRefresh)
		r.With(h.requirePermission(PermissionCredentialsManage)).Post("/credentials/{credential_id}/disable", h.handleCredentialDisable)
		r.With(h.requirePermission(PermissionCredentialsManage)).Post("/credentials/{credential_id}/enable", h.handleCredentialEnable)
		r.With(h.requirePermission(PermissionCredentialsManage)).Post("/credentials/{credential_id}/delete", h.handleCredentialDelete)

		// Models
		r.With(h.requirePermission(PermissionModelsRead)).Get("/models", h.handleModels)
		r.With(h.requirePermission(PermissionModelsRead)).Get("/models/table", h.handleModelsTable)
		r.With(h.requirePermission(PermissionModelsManage)).Post("/models/create", h.handleModelCreate)
		r.With(h.requirePermission(PermissionModelsManage)).Get("/models/edit", h.handleModelEdit)
		r.With(h.requirePermission(PermissionModelsManage)).Post("/models/update", h.handleModelUpdate)
		r.With(h.requirePermission(PermissionModelsManage)).Post("/models/delete", h.handleModelDelete)
		r.With(h.requirePermission(PermissionModelsManage)).Post("/models/sync-pricing", h.handleSyncPricing)

		// Usage
		r.With(h.requirePermission(PermissionUsageRead)).Get("/usage", h.handleUsage)
		r.With(h.requirePermission(PermissionUsageRead)).Get("/usage/tab", h.handleUsageTab)
		r.With(h.requirePermission(PermissionUsageRead)).Get("/usage/top-keys", h.handleUsageTopKeys)
		r.With(h.requirePermission(PermissionUsageRead)).Get("/usage/export", h.handleUsageExport)
		r.With(h.requirePermission(PermissionUsageRead)).Get("/api/claude-code-usage", h.handleClaudeCodeAPI)
		r.With(h.requirePermission(PermissionUsageRead)).Get("/api/codex-usage", h.handleCodexUsageAPI)

		// OAuth token management
		r.With(h.requirePermission(PermissionCredentialsManage)).Get("/openai/connect", h.handleOpenAIConnect)
		r.With(h.requirePermission(PermissionCredentialsManage)).Post("/openai/device/start", h.handleOpenAIDeviceStart)
		r.With(h.requirePermission(PermissionCredentialsManage)).Post("/openai/device/status", h.handleOpenAIDeviceStatus)
		r.With(h.requirePermission(PermissionCredentialsManage)).Post("/openai/device/cancel", h.handleOpenAIDeviceCancel)
		r.With(h.requirePermission(PermissionCredentialsManage)).Post("/openai/callback-url", h.handleOpenAICallbackURL)
		r.With(h.requirePermission(PermissionCredentialsManage)).Post("/credentials/{credential_id}/codex-usage/refresh", h.handleCredentialCodexUsageRefresh)
		r.With(h.requirePermission(PermissionCredentialsManage)).Post("/credentials/{credential_id}/codex-usage/reset", h.handleCredentialCodexUsageReset)
		r.With(h.requirePermission(PermissionCredentialsManage)).Post("/oauth-tokens/disable", h.handleOAuthTokenDisable)
		r.With(h.requirePermission(PermissionCredentialsManage)).Post("/oauth-tokens/enable", h.handleOAuthTokenEnable)

		// Logs
		r.With(h.requirePermission(PermissionLogsRead)).Get("/logs", h.handleLogs)
		r.With(h.requirePermission(PermissionLogsRead)).Get("/logs/table", h.handleLogsTable)
		r.With(h.requirePermission(PermissionLogsRead)).Get("/logs/detail", h.handleLogDetail)

		// Users (admin only)
		r.Group(func(r chi.Router) {
			r.Use(h.requireAdmin)
			r.Get("/users", h.handleUsers)
			r.Get("/users/table", h.handleUsersTable)
			r.Post("/users/create", h.handleUserCreate)
			r.Get("/users/{user_id}", h.handleUserDetail)
			r.Get("/users/{user_id}/edit", h.handleUserEdit)
			r.Post("/users/{user_id}/update", h.handleUserUpdate)
			r.Post("/users/{user_id}/block", h.handleUserBlock)
			r.Post("/users/{user_id}/unblock", h.handleUserUnblock)
			r.Post("/users/{user_id}/delete", h.handleUserDelete)
			r.Post("/users/{user_id}/social-auth/enrollment", h.handleUserSocialAuthEnrollment)
			r.Post("/users/{user_id}/social-auth/identities/{identity_id}/delete", h.handleUserIdentityDelete)
		})
	})
}
