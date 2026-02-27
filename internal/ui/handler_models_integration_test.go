//go:build integration

package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
)

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://tianji:tianji@localhost:5433/tianji_e2e"
	}
	return dsn
}

func setupIntegrationDB(t *testing.T) (*pgxpool.Pool, *db.Queries) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDatabaseURL(t))
	if err != nil {
		t.Fatalf("connect test DB: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping test DB: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	queries := db.New(pool)
	if err := queries.DeleteAllModelPricing(ctx); err != nil {
		t.Fatalf("clean ModelPricing table: %v", err)
	}
	return pool, queries
}

func buildUpstreamServer(t *testing.T, n int) *httptest.Server {
	t.Helper()
	m := make(map[string]any, n)
	for i := 0; i < n; i++ {
		m[fmt.Sprintf("ui-test-model-%d", i)] = map[string]any{
			"input_cost_per_token":  float64(i+1) * 0.0001,
			"output_cost_per_token": float64(i+1) * 0.0002,
			"max_input_tokens":      4096,
			"max_output_tokens":     2048,
			"max_tokens":            6144,
			"mode":                  "chat",
			"litellm_provider":      "test",
		}
	}
	body, _ := json.Marshal(m)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestIntegrationUI_SyncPricingSuccess: POST sync-pricing → 200 + success toast + DB has data.
func TestIntegrationUI_SyncPricingSuccess(t *testing.T) {
	pool, queries := setupIntegrationDB(t)
	upstream := buildUpstreamServer(t, 60)

	t.Setenv("PRICING_UPSTREAM_URL", upstream.URL)
	// Use an error-returning stub to keep model count deterministic (graceful degradation skips it).
	orSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "disabled in tests", http.StatusServiceUnavailable)
	}))
	t.Cleanup(orSrv.Close)
	t.Setenv("OPENROUTER_PRICING_URL", orSrv.URL)

	h := &UIHandler{
		DB:      queries,
		Pool:    pool,
		Config:  &config.ProxyConfig{},
		Pricing: pricing.Default(),
	}

	req := httptest.NewRequest(http.MethodPost, "/ui/models/sync-pricing", nil)
	w := httptest.NewRecorder()
	h.handleSyncPricing(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Synced") {
		t.Errorf("expected 'Synced' in response body, got: %q", body)
	}

	// Verify DB has entries
	ctx := context.Background()
	entries, err := queries.ListModelPricing(ctx)
	if err != nil {
		t.Fatalf("ListModelPricing: %v", err)
	}
	if len(entries) != 60 {
		t.Errorf("expected 60 DB rows after sync, got %d", len(entries))
	}
}

// TestIntegrationUI_SyncPricingConcurrent409: second concurrent POST → 409.
func TestIntegrationUI_SyncPricingConcurrent409(t *testing.T) {
	pool, queries := setupIntegrationDB(t)

	h := &UIHandler{
		DB:      queries,
		Pool:    pool,
		Config:  &config.ProxyConfig{},
		Pricing: pricing.Default(),
	}

	// Lock mutex to simulate ongoing sync
	h.syncPricingMu.Lock()

	req := httptest.NewRequest(http.MethodPost, "/ui/models/sync-pricing", nil)
	w := httptest.NewRecorder()
	h.handleSyncPricing(w, req)

	h.syncPricingMu.Unlock()

	if w.Result().StatusCode != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Result().StatusCode)
	}
}

// TestIntegrationUI_SyncPricingDBNil: DB nil → error response.
func TestIntegrationUI_SyncPricingDBNil(t *testing.T) {
	h := &UIHandler{
		DB:      nil,
		Pool:    nil,
		Config:  &config.ProxyConfig{},
		Pricing: pricing.Default(),
	}

	req := httptest.NewRequest(http.MethodPost, "/ui/models/sync-pricing", nil)
	w := httptest.NewRecorder()
	h.handleSyncPricing(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "Database not configured") {
		t.Errorf("expected 'Database not configured', got: %q", body)
	}
}

// TestIntegrationUI_ConcurrentSafeRace: fire multiple goroutines, verify no data races.
func TestIntegrationUI_ConcurrentSafeRace(t *testing.T) {
	pool, queries := setupIntegrationDB(t)
	upstream := buildUpstreamServer(t, 60)

	t.Setenv("PRICING_UPSTREAM_URL", upstream.URL)
	orSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "disabled in tests", http.StatusServiceUnavailable)
	}))
	t.Cleanup(orSrv.Close)
	t.Setenv("OPENROUTER_PRICING_URL", orSrv.URL)

	h := &UIHandler{
		DB:      queries,
		Pool:    pool,
		Config:  &config.ProxyConfig{},
		Pricing: pricing.Default(),
	}

	const n = 5
	codes := make([]int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/ui/models/sync-pricing", nil)
			w := httptest.NewRecorder()
			h.handleSyncPricing(w, req)
			codes[idx] = w.Result().StatusCode
		}(i)
	}
	wg.Wait()

	for i, code := range codes {
		if code != http.StatusOK && code != http.StatusConflict {
			t.Errorf("goroutine %d: unexpected status %d", i, code)
		}
	}
}
