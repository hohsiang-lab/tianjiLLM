//go:build integration

package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// testDatabaseURL returns the integration test DB URL from environment.
func testDatabaseURL(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://tianji:tianji@localhost:5433/tianji_e2e"
	}
	return dsn
}

// setupTestDB connects to the test database and returns pool + queries.
func setupTestDB(t *testing.T) (*pgxpool.Pool, *db.Queries) {
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

	// Ensure clean state
	if cleanErr := queries.DeleteAllModelPricing(ctx); cleanErr != nil {
		t.Fatalf("clean ModelPricing table: %v", cleanErr)
	}

	return pool, queries
}

// buildIntegrationUpstream returns an httptest.Server serving n model entries.
func buildIntegrationUpstream(t *testing.T, n int) *httptest.Server {
	t.Helper()
	m := make(map[string]any, n)
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("test-model-%d", i)
		m[name] = map[string]any{
			"input_cost_per_token":  float64(i+1) * 0.0001,
			"output_cost_per_token": float64(i+1) * 0.0002,
			"max_input_tokens":      4096,
			"max_output_tokens":     2048,
			"max_tokens":            6144,
			"mode":                  "chat",
			"litellm_provider":      "test-provider",
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

// TestIntegration_SyncWritesToDB: full sync → DB has entries.
func TestIntegration_SyncWritesToDB(t *testing.T) {
	pool, queries := setupTestDB(t)
	srv := buildIntegrationUpstream(t, 60)

	calc := &Calculator{
		embedded:  make(map[string]ModelInfo),
		models:    make(map[string]ModelInfo),
		overrides: make(map[string]ModelInfo),
	}

	ctx := context.Background()
	count, err := SyncFromUpstream(ctx, pool, queries, calc, srv.URL, "")
	if err != nil {
		t.Fatalf("SyncFromUpstream failed: %v", err)
	}
	if count != 60 {
		t.Errorf("expected 60 synced, got %d", count)
	}

	// Verify DB has data
	entries, err := queries.ListModelPricing(ctx)
	if err != nil {
		t.Fatalf("ListModelPricing: %v", err)
	}
	if len(entries) != 60 {
		t.Errorf("expected 60 DB rows, got %d", len(entries))
	}
}

// TestIntegration_CalcLookupUsesDBAfterSync: after sync, calculator uses DB prices.
func TestIntegration_CalcLookupUsesDBAfterSync(t *testing.T) {
	pool, queries := setupTestDB(t)
	srv := buildIntegrationUpstream(t, 60)

	calc := &Calculator{
		embedded:  map[string]ModelInfo{"test-model-0": {InputCostPerToken: 9999}},
		models:    map[string]ModelInfo{"test-model-0": {InputCostPerToken: 9999}},
		overrides: make(map[string]ModelInfo),
	}

	ctx := context.Background()
	if _, err := SyncFromUpstream(ctx, pool, queries, calc, srv.URL, ""); err != nil {
		t.Fatalf("SyncFromUpstream: %v", err)
	}

	// After sync, test-model-0 should have the upstream price (0.0001), not 9999
	info := calc.lookup("test-model-0")
	if info == nil {
		t.Fatal("test-model-0 not found after sync")
	}
	if info.InputCostPerToken != 0.0001 {
		t.Errorf("expected 0.0001 from DB sync, got %v", info.InputCostPerToken)
	}
}

// TestIntegration_RestartReloadFromDB: simulates restart by re-listing and reloading.
func TestIntegration_RestartReloadFromDB(t *testing.T) {
	pool, queries := setupTestDB(t)
	srv := buildIntegrationUpstream(t, 60)

	calc1 := &Calculator{
		embedded:  make(map[string]ModelInfo),
		models:    make(map[string]ModelInfo),
		overrides: make(map[string]ModelInfo),
	}

	ctx := context.Background()
	if _, err := SyncFromUpstream(ctx, pool, queries, calc1, srv.URL, ""); err != nil {
		t.Fatalf("SyncFromUpstream: %v", err)
	}

	// Simulate restart: new calculator, reload from DB
	calc2 := &Calculator{
		embedded:  make(map[string]ModelInfo),
		models:    make(map[string]ModelInfo),
		overrides: make(map[string]ModelInfo),
	}
	entries, err := queries.ListModelPricing(ctx)
	if err != nil {
		t.Fatalf("ListModelPricing: %v", err)
	}
	calc2.ReloadFromDB(entries)

	// Both calculators should return same price for test-model-1
	info1 := calc1.lookup("test-model-1")
	info2 := calc2.lookup("test-model-1")
	if info1 == nil || info2 == nil {
		t.Fatal("test-model-1 not found in one of the calculators")
	}
	if info1.InputCostPerToken != info2.InputCostPerToken {
		t.Errorf("prices differ after reload: %v vs %v", info1, info2)
	}
}

// TestIntegration_SyncFailureRollback: server error → DB unchanged.
func TestIntegration_SyncFailureRollback(t *testing.T) {
	pool, queries := setupTestDB(t)
	ctx := context.Background()

	// First, put known data in DB
	_ = queries.DeleteAllModelPricing(ctx)

	calc := &Calculator{
		embedded:  make(map[string]ModelInfo),
		models:    make(map[string]ModelInfo),
		overrides: make(map[string]ModelInfo),
	}

	// Sync with 500 error server → should fail, DB stays empty
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer errSrv.Close()

	_, err := SyncFromUpstream(ctx, pool, queries, calc, errSrv.URL, "")
	if err == nil {
		t.Fatal("expected error from 500 server")
	}

	// DB should still be empty
	entries, _ := queries.ListModelPricing(ctx)
	if len(entries) != 0 {
		t.Errorf("expected DB unchanged (empty) after failed sync, got %d rows", len(entries))
	}
}

// TestIntegration_EmbeddedFallbackWhenDBEmpty: no DB data → embedded prices used.
func TestIntegration_EmbeddedFallbackWhenDBEmpty(t *testing.T) {
	// No DB needed for this test — just verifies Calculator fallback logic
	calc := &Calculator{
		embedded:  map[string]ModelInfo{"fallback-model": {InputCostPerToken: 0.42}},
		models:    make(map[string]ModelInfo),
		overrides: make(map[string]ModelInfo),
	}

	info := calc.lookup("fallback-model")
	if info == nil {
		t.Fatal("expected to find fallback-model in embedded")
	}
	if info.InputCostPerToken != 0.42 {
		t.Errorf("expected 0.42 from embedded, got %v", info.InputCostPerToken)
	}
}
