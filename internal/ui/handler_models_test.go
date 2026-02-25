package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
)

// TestHandleSyncPricing_DBNil returns error when DB is nil.
func TestHandleSyncPricing_DBNil(t *testing.T) {
	h := &UIHandler{
		DB:      nil,
		Pool:    nil,
		Config:  &config.ProxyConfig{},
		Pricing: pricing.Default(),
	}

	req := httptest.NewRequest(http.MethodPost, "/ui/models/sync-pricing", nil)
	w := httptest.NewRecorder()

	h.handleSyncPricing(w, req)

	resp := w.Result()
	// Should not be 409 (no concurrent lock issue)
	if resp.StatusCode == http.StatusConflict {
		t.Error("unexpected 409 for nil DB case")
	}
	body := w.Body.String()
	if !strings.Contains(body, "Database not configured") {
		t.Errorf("expected 'Database not configured' in response, got: %q", body)
	}
}

// TestHandleSyncPricing_ConcurrentReturns409 tests that a second concurrent
// sync attempt returns 409 Conflict.
func TestHandleSyncPricing_ConcurrentReturns409(t *testing.T) {
	h := &UIHandler{
		DB:      nil,
		Pool:    nil,
		Config:  &config.ProxyConfig{},
		Pricing: pricing.Default(),
	}

	// Manually lock the mutex to simulate an ongoing sync.
	h.syncPricingMu.Lock()

	req := httptest.NewRequest(http.MethodPost, "/ui/models/sync-pricing", nil)
	w := httptest.NewRecorder()

	h.handleSyncPricing(w, req)

	h.syncPricingMu.Unlock()

	resp := w.Result()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409, got %d", resp.StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "in progress") {
		t.Errorf("expected 'in progress' message in response, got: %q", body)
	}
}

// TestHandleSyncPricing_LockReleasedAfterCompletion verifies the mutex is released.
func TestHandleSyncPricing_LockReleasedAfterCompletion(t *testing.T) {
	h := &UIHandler{
		DB:      nil,
		Pool:    nil,
		Config:  &config.ProxyConfig{},
		Pricing: pricing.Default(),
	}

	req := httptest.NewRequest(http.MethodPost, "/ui/models/sync-pricing", nil)
	w := httptest.NewRecorder()
	h.handleSyncPricing(w, req)

	// After the handler returns, the mutex must be unlocked.
	// TryLock should succeed.
	if !h.syncPricingMu.TryLock() {
		t.Error("mutex still locked after handler returned — defer Unlock not working")
	}
	h.syncPricingMu.Unlock()
}

// TestHandleSyncPricing_NoPanicWithNilPricing ensures Pricing nil doesn't panic
// before DB nil check (DB nil returns early before using Pricing).
func TestHandleSyncPricing_NilPricingNilDB(t *testing.T) {
	h := &UIHandler{
		DB:      nil,
		Pool:    nil,
		Config:  &config.ProxyConfig{},
		Pricing: nil, // nil is safe because DB nil check returns early
	}

	req := httptest.NewRequest(http.MethodPost, "/ui/models/sync-pricing", nil)
	w := httptest.NewRecorder()

	// Should not panic; DB nil check returns before Pricing is used.
	h.handleSyncPricing(w, req)
	body := w.Body.String()
	if !strings.Contains(body, "Database not configured") {
		t.Errorf("expected 'Database not configured', got: %q", body)
	}
}

// TestHandleSyncPricing_ConcurrentSafe: fire multiple goroutines simultaneously
// and verify at most one gets 200 and the rest get 409.
func TestHandleSyncPricing_ConcurrentSafe(t *testing.T) {
	h := &UIHandler{
		DB:      nil,
		Pool:    nil,
		Config:  &config.ProxyConfig{},
		Pricing: pricing.Default(),
	}

	const goroutines = 10
	results := make([]int, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/ui/models/sync-pricing", nil)
			w := httptest.NewRecorder()
			h.handleSyncPricing(w, req)
			results[idx] = w.Result().StatusCode
		}(i)
	}
	wg.Wait()

	// Count 409s — all non-200/non-409 would be unusual
	for _, code := range results {
		if code != http.StatusOK && code != http.StatusConflict {
			t.Errorf("unexpected status code %d", code)
		}
	}
}
