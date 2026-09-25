package callback

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// mockRateLimitDB implements RateLimitDB for testing.
type mockRateLimitDB struct {
	mu      sync.Mutex
	rows    []db.OAuthTokenRateLimitState
	upserts []db.UpsertOAuthTokenRateLimitStateParams

	upsertErr error
	selectErr error
}

func (m *mockRateLimitDB) UpsertOAuthTokenRateLimitState(_ context.Context, arg db.UpsertOAuthTokenRateLimitStateParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.upsertErr != nil {
		return m.upsertErr
	}
	m.upserts = append(m.upserts, arg)
	return nil
}

func (m *mockRateLimitDB) GetActiveOAuthTokenRateLimitStates(_ context.Context) ([]db.OAuthTokenRateLimitState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.selectErr != nil {
		return nil, m.selectErr
	}
	return m.rows, nil
}

func (m *mockRateLimitDB) GetOAuthTokenRateLimitState(_ context.Context, tokenKey string) (db.OAuthTokenRateLimitState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.selectErr != nil {
		return db.OAuthTokenRateLimitState{}, m.selectErr
	}
	for _, row := range m.rows {
		if row.TokenKey == tokenKey {
			return row, nil
		}
	}
	return db.OAuthTokenRateLimitState{}, errors.New("not found")
}

func freshTimestamp() pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Now(), Valid: true}
}

// --- Phase 3: Preload Tests (US1) ---

func TestPreload_LoadsRecentStates(t *testing.T) {
	mdb := &mockRateLimitDB{
		rows: []db.OAuthTokenRateLimitState{
			{TokenKey: "tok1", Unified5hUtilization: 0.30, UnifiedStatus: UnifiedStatusAllowed, UpdatedAt: freshTimestamp()},
			{TokenKey: "tok2", Unified5hUtilization: 0.60, UnifiedStatus: UnifiedStatusAllowed, UpdatedAt: freshTimestamp()},
		},
	}
	store := NewInMemoryRateLimitStore()

	PreloadRateLimitStore(context.Background(), mdb, store)

	for _, key := range []string{"tok1", "tok2"} {
		if _, ok := store.Get(key); !ok {
			t.Errorf("store.Get(%q) = not found after preload", key)
		}
	}
}

func TestPreload_EmptyDB(t *testing.T) {
	mdb := &mockRateLimitDB{rows: nil}
	store := NewInMemoryRateLimitStore()

	PreloadRateLimitStore(context.Background(), mdb, store)

	if all := store.GetAll(); len(all) != 0 {
		t.Errorf("store has %d entries after preload of empty DB, want 0", len(all))
	}
}

// TestPreload_LoadsOldUpdatedAtWithActive7dWindow is the regression test for the
// sticky-token dead-loop bug: a token whose updated_at is days old (> 8h preload TTL)
// but whose 7d window is still active must be preloaded. Without it, stickyScore
// returns (1, 0) and the token is never selected → never updated → dead loop.
func TestPreload_LoadsOldUpdatedAtWithActive7dWindow(t *testing.T) {
	future7dReset := fmt.Sprintf("%d", time.Now().Add(3*24*time.Hour).Unix()) // 3 days from now
	staleUpdatedAt := pgtype.Timestamptz{Time: time.Now().Add(-65 * time.Hour), Valid: true}

	mdb := &mockRateLimitDB{
		rows: []db.OAuthTokenRateLimitState{
			{
				TokenKey:             "stale-but-active",
				Unified7dUtilization: 0.17,
				Unified7dReset:       future7dReset,
				UnifiedStatus:        UnifiedStatusAllowed,
				UpdatedAt:            staleUpdatedAt,
			},
		},
	}
	store := NewInMemoryRateLimitStore()

	PreloadRateLimitStore(context.Background(), mdb, store)

	state, ok := store.Get("stale-but-active")
	if !ok {
		t.Fatal("store.Get('stale-but-active') not found — token with active 7d window must be preloaded regardless of updated_at age")
	}
	if state.Unified7dUtilization != 0.17 {
		t.Errorf("7d_util = %v, want 0.17", state.Unified7dUtilization)
	}
}

func TestPreload_CorrectFieldMapping(t *testing.T) {
	futureReset := fmt.Sprintf("%d", time.Now().Add(2*time.Hour).Unix())
	mdb := &mockRateLimitDB{
		rows: []db.OAuthTokenRateLimitState{
			{
				TokenKey:                   "tok1",
				UnifiedStatus:              UnifiedStatusAllowed,
				UnifiedReset:               futureReset,
				Unified5hStatus:            UnifiedStatusAllowed,
				Unified5hUtilization:       0.42,
				Unified5hReset:             futureReset,
				Unified7dStatus:            UnifiedStatusRejected,
				Unified7dUtilization:       0.88,
				Unified7dReset:             futureReset,
				Unified7dSonnetStatus:      UnifiedStatusAllowed,
				Unified7dSonnetUtilization: 0.33,
				Unified7dSonnetReset:       futureReset,
				OrgID:                      "org-123",
				UpdatedAt:                  freshTimestamp(),
			},
		},
	}
	store := NewInMemoryRateLimitStore()

	PreloadRateLimitStore(context.Background(), mdb, store)

	state, ok := store.Get("tok1")
	if !ok {
		t.Fatal("store.Get('tok1') not found")
	}
	if state.Unified5hUtilization != 0.42 {
		t.Errorf("5h_util = %v, want 0.42", state.Unified5hUtilization)
	}
	if state.Unified7dUtilization != 0.88 {
		t.Errorf("7d_util = %v, want 0.88", state.Unified7dUtilization)
	}
	if state.Unified7dSonnetUtilization != 0.33 {
		t.Errorf("7d_sonnet_util = %v, want 0.33", state.Unified7dSonnetUtilization)
	}
	if state.OrganizationID != "org-123" {
		t.Errorf("org_id = %q, want 'org-123'", state.OrganizationID)
	}
	if state.UnifiedStatus != UnifiedStatusAllowed {
		t.Errorf("status = %q, want 'allowed'", state.UnifiedStatus)
	}
	if state.UnifiedReset != futureReset {
		t.Errorf("UnifiedReset = %q, want %q", state.UnifiedReset, futureReset)
	}
}

func TestPreload_DBConnectionError(t *testing.T) {
	mdb := &mockRateLimitDB{selectErr: errors.New("connection refused")}
	store := NewInMemoryRateLimitStore()

	// Should not panic.
	PreloadRateLimitStore(context.Background(), mdb, store)

	if all := store.GetAll(); len(all) != 0 {
		t.Errorf("store has %d entries after DB error, want 0", len(all))
	}
}

// --- Phase 4: Flush Tests (US2) ---

func TestFlush_WritesDirtyKeys(t *testing.T) {
	mdb := &mockRateLimitDB{}
	store := NewInMemoryRateLimitStore()
	store.Set("a", AnthropicOAuthRateLimitState{TokenKey: "a", Unified5hUtilization: 0.1})
	store.Set("b", AnthropicOAuthRateLimitState{TokenKey: "b", Unified5hUtilization: 0.2})

	flusher := NewRateLimitFlusher(store, mdb, 30*time.Second)
	flusher.flush(context.Background())

	mdb.mu.Lock()
	defer mdb.mu.Unlock()
	if len(mdb.upserts) != 2 {
		t.Fatalf("flush wrote %d rows, want 2", len(mdb.upserts))
	}
}

func TestFlush_OnlyDirtyKeys(t *testing.T) {
	mdb := &mockRateLimitDB{}
	store := NewInMemoryRateLimitStore()
	store.Set("a", AnthropicOAuthRateLimitState{TokenKey: "a", Unified5hUtilization: 0.1})
	store.Set("a", AnthropicOAuthRateLimitState{TokenKey: "a", Unified5hUtilization: 0.2}) // overwrite

	flusher := NewRateLimitFlusher(store, mdb, 30*time.Second)
	flusher.flush(context.Background())

	mdb.mu.Lock()
	defer mdb.mu.Unlock()
	if len(mdb.upserts) != 1 {
		t.Fatalf("flush wrote %d rows, want 1 (deduped)", len(mdb.upserts))
	}
	if mdb.upserts[0].Unified5hUtilization != 0.2 {
		t.Errorf("flushed 5h_util = %v, want 0.2 (latest)", mdb.upserts[0].Unified5hUtilization)
	}
}

func TestFlush_429UpdatesDB(t *testing.T) {
	mdb := &mockRateLimitDB{}
	store := NewInMemoryRateLimitStore()
	store.Set("throttled", AnthropicOAuthRateLimitState{
		TokenKey:             "throttled",
		UnifiedStatus:        UnifiedStatusRejected,
		UnifiedReset:         "1700000000",
		Unified5hUtilization: 1.00,
	})

	flusher := NewRateLimitFlusher(store, mdb, 30*time.Second)
	flusher.flush(context.Background())

	mdb.mu.Lock()
	defer mdb.mu.Unlock()
	if len(mdb.upserts) != 1 {
		t.Fatal("expected 1 upsert")
	}
	if mdb.upserts[0].Unified5hUtilization != 1.00 {
		t.Errorf("flushed 5h_util = %v, want 1.00", mdb.upserts[0].Unified5hUtilization)
	}
	if mdb.upserts[0].UnifiedReset != "1700000000" {
		t.Errorf("flushed UnifiedReset = %q, want '1700000000'", mdb.upserts[0].UnifiedReset)
	}
}

func TestFlush_DBUnavailable(t *testing.T) {
	mdb := &mockRateLimitDB{upsertErr: errors.New("connection refused")}
	store := NewInMemoryRateLimitStore()
	store.Set("a", AnthropicOAuthRateLimitState{TokenKey: "a", Unified5hUtilization: 0.5})

	flusher := NewRateLimitFlusher(store, mdb, 30*time.Second)
	flusher.flush(context.Background())

	// Store should still have the data.
	if _, ok := store.Get("a"); !ok {
		t.Error("store lost entry after failed flush")
	}

	// Dirty keys should be retained for retry.
	snap := store.SnapshotDirty()
	if _, ok := snap["a"]; !ok {
		t.Error("dirty key 'a' should be retained after failed flush")
	}
	// Re-mark so future tests aren't affected.
	store.MarkDirty([]string{"a"})
}

func TestFlush_GracefulShutdown(t *testing.T) {
	mdb := &mockRateLimitDB{}
	store := NewInMemoryRateLimitStore()
	store.Set("final", AnthropicOAuthRateLimitState{TokenKey: "final", Unified5hUtilization: 0.99})

	flusher := NewRateLimitFlusher(store, mdb, 1*time.Hour) // long interval — only shutdown triggers flush

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		flusher.Start(ctx)
		close(done)
	}()

	// Cancel immediately to trigger shutdown flush.
	cancel()
	<-done

	mdb.mu.Lock()
	defer mdb.mu.Unlock()
	if len(mdb.upserts) != 1 {
		t.Fatalf("shutdown flush wrote %d rows, want 1", len(mdb.upserts))
	}
	if mdb.upserts[0].TokenKey != "final" {
		t.Errorf("flushed key = %q, want 'final'", mdb.upserts[0].TokenKey)
	}
}

func TestFlush_ClearsDirtyAfterSuccess(t *testing.T) {
	mdb := &mockRateLimitDB{}
	store := NewInMemoryRateLimitStore()
	store.Set("a", AnthropicOAuthRateLimitState{TokenKey: "a"})

	flusher := NewRateLimitFlusher(store, mdb, 30*time.Second)
	flusher.flush(context.Background())

	snap := store.SnapshotDirty()
	if len(snap) != 0 {
		t.Errorf("dirty set has %d keys after successful flush, want 0", len(snap))
	}
}

func TestFlush_RetainsDirtyOnFailure(t *testing.T) {
	mdb := &mockRateLimitDB{upsertErr: errors.New("db error")}
	store := NewInMemoryRateLimitStore()
	store.Set("a", AnthropicOAuthRateLimitState{TokenKey: "a"})

	flusher := NewRateLimitFlusher(store, mdb, 30*time.Second)
	flusher.flush(context.Background())

	snap := store.SnapshotDirty()
	if _, ok := snap["a"]; !ok {
		t.Error("dirty key 'a' should be retained after failed flush")
	}
}

func TestFlush_NoDirtyKeys(t *testing.T) {
	mdb := &mockRateLimitDB{}
	store := NewInMemoryRateLimitStore()

	flusher := NewRateLimitFlusher(store, mdb, 30*time.Second)
	flusher.flush(context.Background())

	mdb.mu.Lock()
	defer mdb.mu.Unlock()
	if len(mdb.upserts) != 0 {
		t.Errorf("flush wrote %d rows when no dirty keys, want 0", len(mdb.upserts))
	}
}

// --- Phase 5: Multi-Pod Sync Tests (US3) ---

func TestMultiPod_PreloadSeesOtherPodWrites(t *testing.T) {
	// Simulate Pod A's flush by inserting directly into the mock DB.
	mdb := &mockRateLimitDB{
		rows: []db.OAuthTokenRateLimitState{
			{TokenKey: "podA-tok", Unified5hUtilization: 0.77, UnifiedStatus: UnifiedStatusAllowed, OrgID: "org-A", UpdatedAt: freshTimestamp()},
		},
	}

	// Pod B starts fresh and preloads.
	store := NewInMemoryRateLimitStore()
	PreloadRateLimitStore(context.Background(), mdb, store)

	state, ok := store.Get("podA-tok")
	if !ok {
		t.Fatal("Pod B's store.Get('podA-tok') not found after preload")
	}
	if state.Unified5hUtilization != 0.77 {
		t.Errorf("5h_util = %v, want 0.77", state.Unified5hUtilization)
	}
}

func TestMultiPod_ConcurrentUpsert(t *testing.T) {
	mdb := &mockRateLimitDB{}
	store := NewInMemoryRateLimitStore()
	flusher := NewRateLimitFlusher(store, mdb, 30*time.Second)

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			store.Set("shared", AnthropicOAuthRateLimitState{
				TokenKey:             "shared",
				Unified5hUtilization: float64(i) * 0.1,
			})
			flusher.flush(context.Background())
		}(i)
	}
	wg.Wait()

	// No panics, no errors. The final state is whatever the last write was.
	if _, ok := store.Get("shared"); !ok {
		t.Error("store.Get('shared') not found after concurrent upserts")
	}
}
