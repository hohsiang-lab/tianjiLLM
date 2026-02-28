package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCacheControlMiddlewareNoAllowed(t *testing.T) {
	mw := NewCacheControlMiddleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"model":"gpt-4","cache":{"type":"semantic"}}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code: %d", rr.Code)
	}
}

func TestCacheControlMiddlewareNonPost(t *testing.T) {
	mw := NewCacheControlMiddleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), allowedCacheControlsKey, []string{"semantic"})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code: %d", rr.Code)
	}
}

func TestCacheControlMiddlewareAllowed(t *testing.T) {
	mw := NewCacheControlMiddleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"model":"gpt-4","cache":{"type":"semantic"}}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), allowedCacheControlsKey, []string{"semantic"})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code: %d", rr.Code)
	}
}

func TestCacheControlMiddlewareForbidden(t *testing.T) {
	mw := NewCacheControlMiddleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"model":"gpt-4","cache":{"type":"semantic"}}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), allowedCacheControlsKey, []string{"disk"})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 403 {
		t.Fatalf("code: %d", rr.Code)
	}
}

func TestCacheControlMiddlewareNoCache(t *testing.T) {
	mw := NewCacheControlMiddleware()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"model":"gpt-4"}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), allowedCacheControlsKey, []string{"semantic"})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code: %d", rr.Code)
	}
}

func TestExtractCacheControls(t *testing.T) {
	// map with type
	r := extractCacheControls(map[string]any{"type": "semantic"})
	if len(r) != 1 || r[0] != "semantic" {
		t.Fatalf("got %v", r)
	}

	// map without type
	r = extractCacheControls(map[string]any{"disk": true, "memory": true})
	if len(r) != 2 {
		t.Fatalf("got %v", r)
	}

	// array
	r = extractCacheControls([]any{"a", "b"})
	if len(r) != 2 {
		t.Fatalf("got %v", r)
	}

	// string
	r = extractCacheControls("single")
	if len(r) != 1 || r[0] != "single" {
		t.Fatalf("got %v", r)
	}

	// nil
	r = extractCacheControls(nil)
	if len(r) != 0 {
		t.Fatalf("got %v", r)
	}

	// int
	r = extractCacheControls(42)
	if len(r) != 0 {
		t.Fatalf("got %v", r)
	}
}
