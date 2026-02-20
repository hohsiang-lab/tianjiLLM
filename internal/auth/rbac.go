package auth

import (
	"errors"
	"strings"
)

// Role represents user role levels in TianjiLLM.
type Role string

const (
	RoleProxyAdmin   Role = "proxy_admin"
	RoleTeam         Role = "team"
	RoleInternalUser Role = "internal_user"
	RoleEndUser      Role = "end_user"
)

var (
	ErrAccessDenied = errors.New("access denied")
	ErrInvalidRole  = errors.New("invalid role")
)

// roleLevel maps roles to numeric levels for comparison.
var roleLevel = map[Role]int{
	RoleProxyAdmin:   100,
	RoleTeam:         50,
	RoleInternalUser: 30,
	RoleEndUser:      10,
}

// RBACEngine enforces role-based access control.
type RBACEngine struct {
	routePermissions map[string]Role // route prefix → minimum required role
}

// NewRBACEngine creates a new RBAC engine with default route permissions.
func NewRBACEngine() *RBACEngine {
	return &RBACEngine{
		routePermissions: map[string]Role{
			// Admin-only routes
			"/key/generate":       RoleProxyAdmin,
			"/key/delete":         RoleProxyAdmin,
			"/key/update":         RoleProxyAdmin,
			"/team/new":           RoleProxyAdmin,
			"/team/delete":        RoleProxyAdmin,
			"/team/update":        RoleProxyAdmin,
			"/user/new":           RoleProxyAdmin,
			"/user/delete":        RoleProxyAdmin,
			"/organization":       RoleProxyAdmin,
			"/credentials":        RoleProxyAdmin,
			"/model_access_group": RoleProxyAdmin,
			"/budget":             RoleProxyAdmin,
			"/callback":           RoleProxyAdmin,
			"/cache":              RoleProxyAdmin,
			"/router":             RoleProxyAdmin,
			"/sso":                RoleProxyAdmin,

			// Team-level routes
			"/key/info":  RoleTeam,
			"/key/list":  RoleTeam,
			"/team/list": RoleTeam,

			// Internal user routes (completions, models, etc.)
			"/v1/chat/completions": RoleInternalUser,
			"/v1/completions":      RoleInternalUser,
			"/v1/embeddings":       RoleInternalUser,
			"/v1/models":           RoleInternalUser,
			"/v1/files":            RoleInternalUser,
			"/v1/batches":          RoleInternalUser,
			"/v1/fine_tuning":      RoleInternalUser,
			"/v1/rerank":           RoleInternalUser,
			"/v1/images":           RoleInternalUser,
			"/v1/audio":            RoleInternalUser,
			"/v1/moderations":      RoleInternalUser,
			"/v1/responses":        RoleInternalUser,

			// Health endpoints — no auth required (handled separately)
		},
	}
}

// CheckRouteAccess verifies the user's role has access to the given route.
func (e *RBACEngine) CheckRouteAccess(role Role, route string) error {
	requiredRole := e.findRequiredRole(route)
	if requiredRole == "" {
		// No permission defined — allow by default
		return nil
	}

	userLevel, ok := roleLevel[role]
	if !ok {
		return ErrInvalidRole
	}

	requiredLevel, ok := roleLevel[requiredRole]
	if !ok {
		return ErrInvalidRole
	}

	if userLevel < requiredLevel {
		return ErrAccessDenied
	}

	return nil
}

// CheckModelAccess verifies the user has access to the requested model.
func (e *RBACEngine) CheckModelAccess(allowedModels []string, requestedModel string) error {
	if len(allowedModels) == 0 {
		return nil // no model restrictions
	}

	for _, m := range allowedModels {
		if m == requestedModel {
			return nil
		}
		// Wildcard match: "openai/*" matches "openai/gpt-4o"
		if strings.HasSuffix(m, "/*") {
			prefix := strings.TrimSuffix(m, "*")
			if strings.HasPrefix(requestedModel, prefix) {
				return nil
			}
		}
	}

	return ErrAccessDenied
}

// findRequiredRole finds the minimum role for a route using prefix matching.
func (e *RBACEngine) findRequiredRole(route string) Role {
	// Exact match first
	if role, ok := e.routePermissions[route]; ok {
		return role
	}

	// Prefix match
	for prefix, role := range e.routePermissions {
		if strings.HasPrefix(route, prefix) {
			return role
		}
	}

	return ""
}

// ParseRole converts a string to a Role, returning ErrInvalidRole if unknown.
func ParseRole(s string) (Role, error) {
	role := Role(s)
	if _, ok := roleLevel[role]; !ok {
		return "", ErrInvalidRole
	}
	return role, nil
}
