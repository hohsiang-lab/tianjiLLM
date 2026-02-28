package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponseIDSecurityEncryptDecrypt(t *testing.T) {
	s := NewResponseIDSecurity("test-secret")

	encrypted := s.EncryptID("resp-123", "user-1", "team-1")
	if encrypted == "" {
		t.Fatal("empty")
	}
	if !strings.Contains(encrypted, "resp-123.") {
		t.Fatalf("got %q", encrypted)
	}

	// Valid decrypt
	id, valid := s.DecryptID(encrypted, "user-1", "team-1")
	if !valid {
		t.Fatal("should be valid")
	}
	if id != "resp-123" {
		t.Fatalf("got %q", id)
	}

	// Wrong user
	_, valid = s.DecryptID(encrypted, "user-2", "team-1")
	if valid {
		t.Fatal("should be invalid for wrong user")
	}

	// Plain ID (not encrypted) passes through
	id, valid = s.DecryptID("plain-id", "user-1", "team-1")
	if !valid {
		t.Fatal("plain should pass through")
	}
	if id != "plain-id" {
		t.Fatalf("got %q", id)
	}
}

func TestResponseSecurityMiddlewareNil(t *testing.T) {
	mw := NewResponseSecurityMiddleware(nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/v1/responses/resp-123", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code: %d", rr.Code)
	}
}

func TestResponseSecurityMiddlewareNonResponsePath(t *testing.T) {
	s := NewResponseIDSecurity("secret")
	mw := NewResponseSecurityMiddleware(s)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/v1/chat/completions", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code: %d", rr.Code)
	}
}

func TestResponseSecurityMiddlewareMasterKey(t *testing.T) {
	s := NewResponseIDSecurity("secret")
	mw := NewResponseSecurityMiddleware(s)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/v1/responses/resp-123", nil)
	ctx := context.WithValue(req.Context(), ContextKeyIsMasterKey, true)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code: %d", rr.Code)
	}
}

func TestResponseSecurityMiddlewareForbidden(t *testing.T) {
	s := NewResponseIDSecurity("secret")
	encrypted := s.EncryptID("resp-123", "user-1", "team-1")

	mw := NewResponseSecurityMiddleware(s)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/v1/responses/"+encrypted, nil)
	ctx := context.WithValue(req.Context(), ContextKeyUserID, "user-2")
	ctx = context.WithValue(ctx, ContextKeyTeamID, "team-2")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 403 {
		t.Fatalf("code: %d", rr.Code)
	}
}

func TestResponseSecurityMiddlewareValid(t *testing.T) {
	s := NewResponseIDSecurity("secret")
	encrypted := s.EncryptID("resp-123", "user-1", "team-1")

	mw := NewResponseSecurityMiddleware(s)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/v1/responses/"+encrypted, nil)
	ctx := context.WithValue(req.Context(), ContextKeyUserID, "user-1")
	ctx = context.WithValue(ctx, ContextKeyTeamID, "team-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code: %d", rr.Code)
	}
}
