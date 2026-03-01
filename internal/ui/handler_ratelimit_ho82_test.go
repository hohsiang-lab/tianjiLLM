package ui

// HO-82: Unit tests for handleRateLimitState handler (package-internal access).
// Tests verify JSON response format, sentinel values, and overage_disabled_reason.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
)

func buildHandlerWithStore(states ...callback.AnthropicOAuthRateLimitState) *UIHandler {
	store := callback.NewInMemoryRateLimitStore()
	for _, s := range states {
		store.Set(s.TokenKey, s)
	}
	return &UIHandler{RateLimitStore: store}
}

// TestHandleRateLimitState_EmptyStore verifies that /ui/api/rate-limit-state returns
// an empty JSON array (not null) when no rate limit data exists.
func TestHandleRateLimitState_EmptyStore(t *testing.T) {
	h := buildHandlerWithStore()

	req := httptest.NewRequest(http.MethodGet, "/ui/api/rate-limit-state", nil)
	w := httptest.NewRecorder()
	h.handleRateLimitState(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var items []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if items == nil {
		t.Error("response is null, expected empty array []")
	}
	if len(items) != 0 {
		t.Errorf("expected empty array, got %d items", len(items))
	}
}

// TestHandleRateLimitState_WithData verifies JSON fields including
// unified_5h_utilization, unified_7d_utilization, and overage_disabled_reason.
func TestHandleRateLimitState_WithData(t *testing.T) {
	state := callback.AnthropicOAuthRateLimitState{
		TokenKey:              "abc123def456",
		UnifiedStatus:         "allowed",
		Unified5hStatus:       "allowed",
		Unified5hUtilization:  0.42,
		Unified7dStatus:       "allowed",
		Unified7dUtilization:  0.31,
		OverageDisabledReason: "",
		ParsedAt:              time.Now(),
	}

	h := buildHandlerWithStore(state)

	req := httptest.NewRequest(http.MethodGet, "/ui/api/rate-limit-state", nil)
	w := httptest.NewRecorder()
	h.handleRateLimitState(w, req)

	var items []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]

	if item["token_key"] != "abc123def456" {
		t.Errorf("token_key = %v, want %q", item["token_key"], "abc123def456")
	}

	u5h, ok := item["unified_5h_utilization"].(float64)
	if !ok {
		t.Errorf("unified_5h_utilization missing or wrong type: %v", item["unified_5h_utilization"])
	} else if u5h != 0.42 {
		t.Errorf("unified_5h_utilization = %v, want 0.42", u5h)
	}

	u7d, ok := item["unified_7d_utilization"].(float64)
	if !ok {
		t.Errorf("unified_7d_utilization missing or wrong type: %v", item["unified_7d_utilization"])
	} else if u7d != 0.31 {
		t.Errorf("unified_7d_utilization = %v, want 0.31", u7d)
	}

	if _, ok := item["overage_disabled_reason"]; !ok {
		t.Error("overage_disabled_reason field missing from response")
	}
}

// TestHandleRateLimitState_SentinelUtilization verifies that when utilization is -1
// (header was absent), the JSON response preserves the -1 sentinel.
// The UI JS layer is responsible for rendering -1 as "—".
func TestHandleRateLimitState_SentinelUtilization(t *testing.T) {
	state := callback.AnthropicOAuthRateLimitState{
		TokenKey:             "tokenkey1",
		UnifiedStatus:        "allowed",
		Unified5hUtilization: -1,
		Unified7dUtilization: -1,
		ParsedAt:             time.Now(),
	}

	h := buildHandlerWithStore(state)

	req := httptest.NewRequest(http.MethodGet, "/ui/api/rate-limit-state", nil)
	w := httptest.NewRecorder()
	h.handleRateLimitState(w, req)

	var items []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	u5h, ok5h := items[0]["unified_5h_utilization"].(float64)
	if !ok5h {
		t.Fatalf("unified_5h_utilization is not float64")
	}
	u7d, ok7d := items[0]["unified_7d_utilization"].(float64)
	if !ok7d {
		t.Fatalf("unified_7d_utilization is not float64")
	}

	if u5h != -1 {
		t.Errorf("unified_5h_utilization = %v, want -1 (sentinel for absent header)", u5h)
	}
	if u7d != -1 {
		t.Errorf("unified_7d_utilization = %v, want -1 (sentinel for absent header)", u7d)
	}
}

// TestHandleRateLimitState_ContentType verifies the response Content-Type is JSON.
func TestHandleRateLimitState_ContentType(t *testing.T) {
	h := buildHandlerWithStore()

	req := httptest.NewRequest(http.MethodGet, "/ui/api/rate-limit-state", nil)
	w := httptest.NewRecorder()
	h.handleRateLimitState(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}
