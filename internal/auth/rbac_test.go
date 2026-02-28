package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRBACEngine_AdminAccessAdmin(t *testing.T) {
	e := NewRBACEngine()
	err := e.CheckRouteAccess(RoleProxyAdmin, "/key/generate")
	assert.NoError(t, err)
}

func TestRBACEngine_TeamDeniedAdmin(t *testing.T) {
	e := NewRBACEngine()
	err := e.CheckRouteAccess(RoleTeam, "/key/generate")
	assert.ErrorIs(t, err, ErrAccessDenied)
}

func TestRBACEngine_InternalUserAccessCompletions(t *testing.T) {
	e := NewRBACEngine()
	err := e.CheckRouteAccess(RoleInternalUser, "/v1/chat/completions")
	assert.NoError(t, err)
}

func TestRBACEngine_EndUserDeniedCompletions(t *testing.T) {
	e := NewRBACEngine()
	err := e.CheckRouteAccess(RoleEndUser, "/v1/chat/completions")
	assert.ErrorIs(t, err, ErrAccessDenied)
}

func TestRBACEngine_UnknownRouteAllowed(t *testing.T) {
	e := NewRBACEngine()
	err := e.CheckRouteAccess(RoleEndUser, "/health")
	assert.NoError(t, err)
}

func TestRBACEngine_InvalidRole(t *testing.T) {
	e := NewRBACEngine()
	err := e.CheckRouteAccess(Role("bogus"), "/key/generate")
	assert.ErrorIs(t, err, ErrInvalidRole)
}

func TestRBACEngine_TeamAccessTeamRoutes(t *testing.T) {
	e := NewRBACEngine()
	assert.NoError(t, e.CheckRouteAccess(RoleTeam, "/key/info"))
	assert.NoError(t, e.CheckRouteAccess(RoleTeam, "/team/list"))
}

func TestRBACEngine_AdminAccessAll(t *testing.T) {
	e := NewRBACEngine()
	routes := []string{"/key/generate", "/key/info", "/v1/chat/completions", "/health"}
	for _, r := range routes {
		assert.NoError(t, e.CheckRouteAccess(RoleProxyAdmin, r))
	}
}

func TestCheckModelAccess_NoRestrictions(t *testing.T) {
	e := NewRBACEngine()
	err := e.CheckModelAccess(nil, "gpt-4o")
	assert.NoError(t, err)
}

func TestCheckModelAccess_ExactMatch(t *testing.T) {
	e := NewRBACEngine()
	err := e.CheckModelAccess([]string{"gpt-4o", "claude-3"}, "gpt-4o")
	assert.NoError(t, err)
}

func TestCheckModelAccess_WildcardMatch(t *testing.T) {
	e := NewRBACEngine()
	err := e.CheckModelAccess([]string{"openai/*"}, "openai/gpt-4o")
	assert.NoError(t, err)
}

func TestCheckModelAccess_Denied(t *testing.T) {
	e := NewRBACEngine()
	err := e.CheckModelAccess([]string{"gpt-4o"}, "claude-3")
	assert.ErrorIs(t, err, ErrAccessDenied)
}

func TestParseRole_Valid(t *testing.T) {
	role, err := ParseRole("proxy_admin")
	require.NoError(t, err)
	assert.Equal(t, RoleProxyAdmin, role)
}

func TestParseRole_Invalid(t *testing.T) {
	_, err := ParseRole("superadmin")
	assert.ErrorIs(t, err, ErrInvalidRole)
}
