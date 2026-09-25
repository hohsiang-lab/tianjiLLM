package oci

import (
	"net/http"
	"testing"
)

func TestSetupHeaders(t *testing.T) {
	p := &Provider{}
	req, _ := http.NewRequest("POST", "http://example.com", nil)
	p.SetupHeaders(req, "test-key")
	if req.Header.Get("Authorization") != "Bearer test-key" {
		t.Fatalf("auth: got %q", req.Header.Get("Authorization"))
	}
	if req.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("content-type: got %q", req.Header.Get("Content-Type"))
	}
}

func TestSetupHeadersNoKey(t *testing.T) {
	p := &Provider{}
	req, _ := http.NewRequest("POST", "http://example.com", nil)
	p.SetupHeaders(req, "")
	if req.Header.Get("Authorization") != "" {
		t.Fatal("should not set auth header with empty key")
	}
}

func TestGetSupportedParams(t *testing.T) {
	p := &Provider{}
	params := p.GetSupportedParams()
	if len(params) == 0 {
		t.Fatal("expected non-empty params")
	}
}
