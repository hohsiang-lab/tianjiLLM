package callback

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestSetFromHeaders_NewEntry(t *testing.T) {
	store := NewInMemoryOAuthUsageStore()
	rl := AnthropicOAuthRateLimitState{
		TokenKey:                   "tok1",
		Unified5hUtilization:       0.3,
		Unified5hReset:             "1774850400",
		Unified7dUtilization:       0.6,
		Unified7dReset:             "1774900000",
		Unified7dSonnetUtilization: 0.4,
		Unified7dSonnetReset:       "1774900000",
		UnifiedStatus:              UnifiedStatusAllowed,
		OrganizationID:             "org-123",
	}
	store.SetFromHeaders("tok1", rl)

	state, ok := store.Get("tok1")
	if !ok {
		t.Fatal("expected to find tok1")
	}
	if state.SessionUsage == nil || *state.SessionUsage != 0.3 {
		t.Errorf("SessionUsage = %v, want 0.3", state.SessionUsage)
	}
	if state.WeeklyUsage == nil || *state.WeeklyUsage != 0.6 {
		t.Errorf("WeeklyUsage = %v, want 0.6", state.WeeklyUsage)
	}
	if state.SonnetUsage == nil || *state.SonnetUsage != 0.4 {
		t.Errorf("SonnetUsage = %v, want 0.4", state.SonnetUsage)
	}
	if state.OrgID != "org-123" {
		t.Errorf("OrgID = %q, want 'org-123'", state.OrgID)
	}
	if state.OverageStatus != "allowed" {
		t.Errorf("OverageStatus = %q, want 'allowed'", state.OverageStatus)
	}
	if state.Error != "" {
		t.Errorf("Error = %q, want empty", state.Error)
	}
}

func TestSetFromHeaders_PreservesExistingValues(t *testing.T) {
	store := NewInMemoryOAuthUsageStore()
	v := 0.5
	store.Set("tok2", OAuthUsageState{
		SessionUsage: &v,
		OrgID:        "org-old",
	})

	// Headers with -1 utilization (missing) and empty org ID should not overwrite.
	rl := AnthropicOAuthRateLimitState{
		TokenKey:                   "tok2",
		Unified5hUtilization:       -1,  // missing → should NOT overwrite
		Unified7dUtilization:       0.7, // present → should overwrite
		Unified7dSonnetUtilization: -1,  // missing
		OrganizationID:             "",  // empty → should NOT overwrite
	}
	store.SetFromHeaders("tok2", rl)

	state, _ := store.Get("tok2")
	if state.SessionUsage == nil || *state.SessionUsage != 0.5 {
		t.Errorf("SessionUsage = %v, want 0.5 (preserved)", state.SessionUsage)
	}
	if state.WeeklyUsage == nil || *state.WeeklyUsage != 0.7 {
		t.Errorf("WeeklyUsage = %v, want 0.7 (updated)", state.WeeklyUsage)
	}
	if state.OrgID != "org-old" {
		t.Errorf("OrgID = %q, want 'org-old' (preserved)", state.OrgID)
	}
}

func TestSetFromHeaders_ClearsError(t *testing.T) {
	store := NewInMemoryOAuthUsageStore()
	store.Set("tok3", OAuthUsageState{Error: "api-error"})

	store.SetFromHeaders("tok3", AnthropicOAuthRateLimitState{
		TokenKey:             "tok3",
		Unified5hUtilization: 0.1,
	})

	state, _ := store.Get("tok3")
	if state.Error != "" {
		t.Errorf("Error = %q, want empty (cleared by SetFromHeaders)", state.Error)
	}
}

func TestGetAll_ReturnsCopy(t *testing.T) {
	store := NewInMemoryOAuthUsageStore()
	v := 0.1
	store.Set("k1", OAuthUsageState{SessionUsage: &v})

	all := store.GetAll()
	// Mutating the returned map should not affect the store.
	delete(all, "k1")

	_, ok := store.Get("k1")
	if !ok {
		t.Error("GetAll returned reference, not copy — store was mutated")
	}
}

// TestPreloadOAuthUsageStore_LoadsFromRateLimitStateTable verifies that after
// a restart, OAuthUsageStore is populated from OAuthTokenRateLimitState rows
// so the Claude Code UI tab shows data without waiting for a live request.
func TestPreloadOAuthUsageStore_LoadsFromRateLimitStateTable(t *testing.T) {
	mdb := &mockRateLimitDB{
		rows: []db.OAuthTokenRateLimitState{
			{
				TokenKey:             "tok1",
				UnifiedStatus:        UnifiedStatusAllowed,
				Unified5hUtilization: 0.30,
				Unified5hReset:       "1999999999", // far future
				Unified7dUtilization: 0.60,
				Unified7dReset:       "1999999998",
				UpdatedAt:            pgtype.Timestamptz{Time: time.Now(), Valid: true},
			},
		},
	}

	usageStore := NewInMemoryOAuthUsageStore()
	PreloadOAuthUsageStore(context.Background(), mdb, usageStore)

	state, ok := usageStore.Get("tok1")
	assert.True(t, ok, "usage store should have tok1 after preload")
	assert.NotNil(t, state.SessionUsage, "SessionUsage should be populated")
	assert.InDelta(t, 0.30, *state.SessionUsage, 0.001)
	assert.NotNil(t, state.WeeklyUsage)
	assert.InDelta(t, 0.60, *state.WeeklyUsage, 0.001)
}

// TestPreloadOAuthUsageStore_EmptyDB results in empty store (no panic).
func TestPreloadOAuthUsageStore_EmptyDB(t *testing.T) {
	mdb := &mockRateLimitDB{rows: nil}
	usageStore := NewInMemoryOAuthUsageStore()

	PreloadOAuthUsageStore(context.Background(), mdb, usageStore)

	assert.Empty(t, usageStore.GetAll())
}

// TestPreloadOAuthUsageStore_DoesNotOverwriteLiveData verifies that if a live
// request already updated the usage store before preload runs (race at startup),
// the live data wins.
func TestPreloadOAuthUsageStore_DoesNotOverwriteLiveData(t *testing.T) {
	mdb := &mockRateLimitDB{
		rows: []db.OAuthTokenRateLimitState{
			{
				TokenKey:             "tok-live",
				Unified5hUtilization: 0.10, // stale DB value
				UpdatedAt:            pgtype.Timestamptz{Time: time.Now().Add(-5 * time.Minute), Valid: true},
			},
		},
	}

	usageStore := NewInMemoryOAuthUsageStore()
	// Simulate live data already in store from a real request
	liveVal := 0.90
	usageStore.Set("tok-live", OAuthUsageState{
		TokenKey:     "tok-live",
		SessionUsage: &liveVal,
		FetchedAt:    time.Now(), // more recent than DB
	})

	PreloadOAuthUsageStore(context.Background(), mdb, usageStore)

	state, _ := usageStore.Get("tok-live")
	assert.NotNil(t, state.SessionUsage)
	assert.InDelta(t, 0.90, *state.SessionUsage, 0.001,
		"live data should not be overwritten by stale DB preload")
}

// TestPreloadOAuthUsageStore_DBError verifies that a DB error leaves the store
// empty and does not panic.
func TestPreloadOAuthUsageStore_DBError(t *testing.T) {
	mdb := &mockRateLimitDB{selectErr: errors.New("connection refused")}
	usageStore := NewInMemoryOAuthUsageStore()

	PreloadOAuthUsageStore(context.Background(), mdb, usageStore)

	assert.Empty(t, usageStore.GetAll(), "store should be empty after DB error")
}
