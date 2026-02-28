package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIPWhitelistDisabled(t *testing.T) {
	wl := NewIPWhitelist(nil)
	if wl.enabled {
		t.Fatal("should be disabled")
	}

	handler := wl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code: %d", rr.Code)
	}
}

func TestIPWhitelistAllowed(t *testing.T) {
	wl := NewIPWhitelist([]string{"10.0.0.1"})

	handler := wl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:5678"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code: %d", rr.Code)
	}
}

func TestIPWhitelistBlocked(t *testing.T) {
	wl := NewIPWhitelist([]string{"10.0.0.1"})

	handler := wl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:5678"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 403 {
		t.Fatalf("code: %d", rr.Code)
	}
}

func TestIPWhitelistAddRemoveList(t *testing.T) {
	wl := NewIPWhitelist(nil)
	wl.Add("1.1.1.1")
	if !wl.enabled {
		t.Fatal("should be enabled")
	}
	ips := wl.List()
	if len(ips) != 1 || ips[0] != "1.1.1.1" {
		t.Fatalf("list: %v", ips)
	}
	wl.Remove("1.1.1.1")
	if wl.enabled {
		t.Fatal("should be disabled")
	}
}

func TestExtractIPNoPort(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4" // no port
	ip := extractIP(req)
	if ip != "1.2.3.4" {
		t.Fatalf("got %q", ip)
	}
}
