package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRolePermissions(t *testing.T) {
	assert.True(t, roleHasPermission("proxy_admin", PermissionUsersManage))
	assert.True(t, roleHasPermission("admin", PermissionCredentialsManage))

	assert.True(t, roleHasPermission("internal_user", PermissionModelsManage))
	assert.False(t, roleHasPermission("internal_user", PermissionCredentialsManage))
	assert.False(t, roleHasPermission("internal_user", PermissionUsersManage))

	assert.True(t, roleHasPermission("internal_user_viewer", PermissionModelsRead))
	assert.True(t, roleHasPermission("internal_user_viewer", PermissionUsageRead))
	assert.False(t, roleHasPermission("internal_user_viewer", PermissionModelsManage))
	assert.False(t, roleHasPermission("unknown", PermissionDashboardRead))

	assert.True(t, isAssignableUIRole("proxy_admin"))
	assert.True(t, isAssignableUIRole("internal_user"))
	assert.True(t, isAssignableUIRole("internal_user_viewer"))
	assert.False(t, isAssignableUIRole("admin"), "admin is reserved for the master-key session")
	assert.False(t, isAssignableUIRole("unknown"))
}

func TestRequirePermissionFailClosed(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/ui/credentials", nil)
	setNonAdminSession(t, req, h)

	called := false
	handler := h.getSessionManager().LoadAndSave(
		h.requirePermission(PermissionCredentialsManage)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		})),
	)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, called)
}
