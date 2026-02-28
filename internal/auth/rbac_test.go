package auth

import (
	"errors"
	"testing"
)

func TestNewRBACEngine(t *testing.T) {
	e := NewRBACEngine()
	if e == nil {
		t.Fatal("NewRBACEngine returned nil")
	}
	if len(e.routePermissions) == 0 {
		t.Fatal("routePermissions should not be empty")
	}
}

func TestCheckRouteAccess(t *testing.T) {
	e := NewRBACEngine()

	tests := []struct {
		name    string
		role    Role
		route   string
		wantErr error
	}{
		{"admin can access admin route", RoleProxyAdmin, "/key/generate", nil},
		{"admin can access team route", RoleProxyAdmin, "/key/info", nil},
		{"admin can access user route", RoleProxyAdmin, "/v1/chat/completions", nil},
		{"team can access team route", RoleTeam, "/key/info", nil},
		{"team cannot access admin route", RoleTeam, "/key/generate", ErrAccessDenied},
		{"internal user can access completions", RoleInternalUser, "/v1/chat/completions", nil},
		{"internal user cannot access admin route", RoleInternalUser, "/key/delete", ErrAccessDenied},
		{"end user cannot access completions", RoleEndUser, "/v1/chat/completions", ErrAccessDenied},
		{"unknown route allowed by default", RoleEndUser, "/health", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := e.CheckRouteAccess(tt.role, tt.route)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckRouteAccessInvalidRole(t *testing.T) {
	e := NewRBACEngine()
	err := e.CheckRouteAccess(Role("bogus"), "/key/generate")
	if !errors.Is(err, ErrInvalidRole) {
		t.Errorf("got %v, want ErrInvalidRole", err)
	}
}

func TestCheckRouteAccessPrefixMatch(t *testing.T) {
	e := NewRBACEngine()
	// /v1/chat/completions/xxx should match prefix /v1/chat/completions
	err := e.CheckRouteAccess(RoleInternalUser, "/v1/chat/completions/stream")
	if err != nil {
		t.Errorf("prefix match should allow access: %v", err)
	}
}

func TestCheckModelAccess(t *testing.T) {
	e := NewRBACEngine()

	tests := []struct {
		name      string
		allowed   []string
		requested string
		wantErr   error
	}{
		{"no restrictions", nil, "gpt-4o", nil},
		{"empty list", []string{}, "gpt-4o", nil},
		{"exact match", []string{"gpt-4o", "gpt-3.5-turbo"}, "gpt-4o", nil},
		{"not in list", []string{"gpt-4o"}, "claude-3", ErrAccessDenied},
		{"wildcard match", []string{"openai/*"}, "openai/gpt-4o", nil},
		{"wildcard no match", []string{"openai/*"}, "anthropic/claude", ErrAccessDenied},
		{"wildcard prefix only", []string{"openai/*"}, "openai-other", ErrAccessDenied},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := e.CheckModelAccess(tt.allowed, tt.requested)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseRole(t *testing.T) {
	tests := []struct {
		input   string
		want    Role
		wantErr bool
	}{
		{"proxy_admin", RoleProxyAdmin, false},
		{"team", RoleTeam, false},
		{"internal_user", RoleInternalUser, false},
		{"end_user", RoleEndUser, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseRole(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRole(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseRole(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
