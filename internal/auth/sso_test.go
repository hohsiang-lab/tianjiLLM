package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewSSOHandler(t *testing.T) {
	h := NewSSOHandler(SSOConfig{ClientID: "id"})
	if h == nil {
		t.Fatal("nil")
	}
}

func TestNewSSOHandlerDefaultScopes(t *testing.T) {
	h := NewSSOHandler(SSOConfig{})
	if len(h.config.Scopes) != 3 {
		t.Fatalf("default scopes = %v", h.config.Scopes)
	}
}

func TestLoginURL(t *testing.T) {
	h := NewSSOHandler(SSOConfig{
		ClientID:    "myapp",
		AuthURL:     "https://idp.example.com/authorize",
		RedirectURI: "https://app.example.com/callback",
		Scopes:      []string{"openid", "email"},
	})
	u := h.LoginURL("state123")
	if !strings.HasPrefix(u, "https://idp.example.com/authorize?") {
		t.Fatalf("unexpected URL: %s", u)
	}
	if !strings.Contains(u, "client_id=myapp") {
		t.Fatal("missing client_id")
	}
	if !strings.Contains(u, "state=state123") {
		t.Fatal("missing state")
	}
	if !strings.Contains(u, "scope=openid+email") {
		t.Fatal("missing scope")
	}
}

func TestMapRole(t *testing.T) {
	h := NewSSOHandler(SSOConfig{
		RoleMapping: map[string]Role{
			"admins":     RoleProxyAdmin,
			"developers": RoleTeam,
			"users":      RoleInternalUser,
		},
	})

	tests := []struct {
		groups []string
		want   Role
	}{
		{[]string{"admins"}, RoleProxyAdmin},
		{[]string{"developers"}, RoleTeam},
		{[]string{"users", "admins"}, RoleProxyAdmin}, // highest wins
		{[]string{"unknown"}, RoleInternalUser},       // default
		{nil, RoleInternalUser},                       // no groups
	}

	for _, tt := range tests {
		got := h.MapRole(tt.groups)
		if got != tt.want {
			t.Errorf("MapRole(%v) = %v, want %v", tt.groups, got, tt.want)
		}
	}
}

func TestExchangeCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(TokenResponse{
			AccessToken: "tok123",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		})
	}))
	defer srv.Close()

	h := NewSSOHandler(SSOConfig{
		ClientID:     "id",
		ClientSecret: "secret",
		TokenURL:     srv.URL,
		RedirectURI:  "http://localhost/callback",
	})

	resp, err := h.ExchangeCode(context.Background(), "authcode")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if resp.AccessToken != "tok123" {
		t.Fatalf("AccessToken = %q", resp.AccessToken)
	}
}

func TestExchangeCodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	h := NewSSOHandler(SSOConfig{TokenURL: srv.URL})
	_, err := h.ExchangeCode(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetUserInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(401)
			return
		}
		_ = json.NewEncoder(w).Encode(UserInfo{
			Sub:   "user1",
			Email: "user@example.com",
			Name:  "Test User",
		})
	}))
	defer srv.Close()

	h := NewSSOHandler(SSOConfig{UserInfoURL: srv.URL})
	info, err := h.GetUserInfo(context.Background(), "tok")
	if err != nil {
		t.Fatalf("GetUserInfo: %v", err)
	}
	if info.Email != "user@example.com" {
		t.Fatalf("Email = %q", info.Email)
	}
}
