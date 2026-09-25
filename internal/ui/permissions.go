package ui

import (
	"context"
	"net/http"
)

type Permission string

const (
	PermissionDashboardRead     Permission = "dashboard.read"
	PermissionKeysRead          Permission = "keys.read"
	PermissionKeysManage        Permission = "keys.manage"
	PermissionCredentialsManage Permission = "credentials.manage"
	PermissionModelsRead        Permission = "models.read"
	PermissionModelsManage      Permission = "models.manage"
	PermissionUsageRead         Permission = "usage.read"
	PermissionLogsRead          Permission = "logs.read"
	PermissionUsersManage       Permission = "users.manage"
)

type principalContextKey struct{}

type Principal struct {
	UserID string
	Role   string
}

var rolePermissions = map[string]map[Permission]struct{}{
	"proxy_admin": {
		PermissionDashboardRead: {}, PermissionKeysRead: {}, PermissionKeysManage: {},
		PermissionCredentialsManage: {}, PermissionModelsRead: {}, PermissionModelsManage: {},
		PermissionUsageRead: {}, PermissionLogsRead: {},
		PermissionUsersManage: {},
	},
	"internal_user": {
		PermissionDashboardRead: {}, PermissionKeysRead: {}, PermissionKeysManage: {},
		PermissionModelsRead: {}, PermissionModelsManage: {}, PermissionUsageRead: {},
		PermissionLogsRead: {},
	},
	"internal_user_viewer": {
		PermissionDashboardRead: {}, PermissionKeysRead: {}, PermissionModelsRead: {},
		PermissionUsageRead: {}, PermissionLogsRead: {},
	},
}

func normalizeUIRole(role string) string {
	if role == "admin" {
		return "proxy_admin"
	}
	return role
}

func roleHasPermission(role string, permission Permission) bool {
	permissions, ok := rolePermissions[normalizeUIRole(role)]
	if !ok {
		return false
	}
	_, ok = permissions[permission]
	return ok
}

func isAssignableUIRole(role string) bool {
	_, ok := rolePermissions[role]
	return ok
}

func principalFromContext(ctx context.Context) (*Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(*Principal)
	return principal, ok && principal != nil
}

func (h *UIHandler) resolveSessionPrincipal(r *http.Request) (*Principal, bool) {
	sessionManager := h.getSessionManager()
	if !sessionManager.GetBool(r.Context(), sessionAuthenticatedKey) {
		return nil, false
	}

	userID := sessionManager.GetString(r.Context(), sessionUserIDKey)
	if userID == "" {
		role := normalizeUIRole(sessionManager.GetString(r.Context(), sessionRoleKey))
		if role != "proxy_admin" {
			return nil, false
		}
		return &Principal{Role: role}, true
	}

	if h.DB == nil {
		return nil, false
	}
	user, err := h.DB.GetUser(r.Context(), userID)
	if err != nil || userStatusFromMetadata(user.Metadata) != "active" {
		_ = sessionManager.Destroy(r.Context())
		return nil, false
	}
	if sessionManager.GetInt64(r.Context(), sessionAuthVersionKey) != user.AuthVersion {
		_ = sessionManager.Destroy(r.Context())
		return nil, false
	}

	role := user.UserRole
	if _, ok := rolePermissions[role]; !ok {
		_ = sessionManager.Destroy(r.Context())
		return nil, false
	}
	return &Principal{UserID: user.UserID, Role: role}, true
}

func (h *UIHandler) currentPrincipal(r *http.Request) (*Principal, bool) {
	if principal, ok := principalFromContext(r.Context()); ok {
		return principal, true
	}
	return h.resolveSessionPrincipal(r)
}

func (h *UIHandler) requirePermission(permission Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := h.currentPrincipal(r)
			if !ok || !roleHasPermission(principal.Role, permission) {
				if r.Header.Get("HX-Request") == "true" {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
