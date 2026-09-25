package handler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/stretchr/testify/assert"
)

func stateWith7d(hash string, u7d float64, resetIn time.Duration) callback.AnthropicOAuthRateLimitState {
	return callback.AnthropicOAuthRateLimitState{
		TokenKey:                   hash,
		UnifiedStatus:              callback.UnifiedStatusAllowed,
		Unified7dUtilization:       u7d,
		Unified7dReset:             fmt.Sprintf("%d", time.Now().Add(resetIn).Unix()),
		Unified7dSonnetUtilization: -1,
		Unified5hReset:             fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix()),
	}
}

func stickyUpstream(key string) nativeUpstream {
	return nativeUpstream{APIKey: key, BaseURL: "https://api.anthropic.com"}
}

func newStickyHandler(store callback.RateLimitStore) *Handlers {
	return &Handlers{
		Config:         &config.ProxyConfig{NativeUpstreamStrategy: config.StrategySticky},
		RateLimitStore: store,
	}
}

// Sticky reuses the same token across calls when the 5h window hasn't reset.
func TestStickySelect_KeepsCurrentTokenWhenAvailable(t *testing.T) {
	t.Parallel()
	rawA, rawB := oauthKey("STICK-A"), oauthKey("STICK-B")
	hashA, hashB := callback.RateLimitCacheKey(rawA), callback.RateLimitCacheKey(rawB)

	store := callback.NewInMemoryRateLimitStore()
	store.Set(hashA, stateWith7d(hashA, 0.5, 24*time.Hour))
	store.Set(hashB, stateWith7d(hashB, 0.3, 6*24*time.Hour))

	h := newStickyHandler(store)
	upstreams := []nativeUpstream{stickyUpstream(rawA), stickyUpstream(rawB)}

	first := h.stickySelect(context.Background(), "anthropic", "", upstreams)
	assert.Equal(t, rawA, first.APIKey, "first call should pick A (soonest 7d reset)")

	// B now has a sooner reset — sticky must still return A (same 5h window).
	store.Set(hashB, stateWith7d(hashB, 0.8, 1*time.Hour))
	second := h.stickySelect(context.Background(), "anthropic", "", upstreams)
	assert.Equal(t, rawA, second.APIKey, "sticky must reuse current token within same 5h window")
}

// When the current token disappears from `available`, sticky picks replacement.
func TestStickySelect_SwitchesWhenCurrentNotInAvailableSet(t *testing.T) {
	t.Parallel()
	rawA, rawB, rawC := oauthKey("SW-A"), oauthKey("SW-B"), oauthKey("SW-C")
	hashB, hashC := callback.RateLimitCacheKey(rawB), callback.RateLimitCacheKey(rawC)

	store := callback.NewInMemoryRateLimitStore()
	store.Set(hashB, stateWith7d(hashB, 0.4, 5*24*time.Hour))
	store.Set(hashC, stateWith7d(hashC, 0.6, 24*time.Hour))

	h := newStickyHandler(store)
	h.stickyState = map[string]stickyEntry{"anthropic:all": {APIKey: rawA, Reset5hAt: "12345"}}

	available := []nativeUpstream{stickyUpstream(rawB), stickyUpstream(rawC)}
	picked := h.stickySelect(context.Background(), "anthropic", "", available)
	assert.Equal(t, rawC, picked.APIKey, "should switch to C (soonest 7d reset)")
	assert.Equal(t, rawC, h.stickyState["anthropic:all"].APIKey)
}

// Cold start picks the token with the soonest 7d reset. Utilization is irrelevant
// to selection; only gates (5h ≥ 80%, 7d ≥ 90%) exclude tokens.
func TestStickySelect_ColdStartPicksSoonest7dReset(t *testing.T) {
	t.Parallel()
	rawA, rawB, rawC := oauthKey("COLD-A"), oauthKey("COLD-B"), oauthKey("COLD-C")
	hashA, hashB, hashC := callback.RateLimitCacheKey(rawA), callback.RateLimitCacheKey(rawB), callback.RateLimitCacheKey(rawC)

	store := callback.NewInMemoryRateLimitStore()
	// A: far 7d reset (6d) — should lose
	store.Set(hashA, stateWith7d(hashA, 0.1, 6*24*time.Hour))
	// B: soonest 7d reset (1d 23h) — should win despite higher util than A
	store.Set(hashB, stateWith7d(hashB, 0.7, (24*time.Hour)-(time.Minute)))
	// C: same order as B but slightly later (2d) — loses to B
	store.Set(hashC, stateWith7d(hashC, 0.3, 2*24*time.Hour))

	h := newStickyHandler(store)
	upstreams := []nativeUpstream{stickyUpstream(rawA), stickyUpstream(rawB), stickyUpstream(rawC)}

	picked := h.stickySelect(context.Background(), "anthropic", "", upstreams)
	assert.Equal(t, rawB, picked.APIKey,
		"B should win: soonest 7d reset (1d23h) beats C (2d) and A (6d); utilization does not matter")
}

// Single upstream takes the fast path without touching stickyState.
func TestStickySelect_SingleUpstreamShortcut(t *testing.T) {
	t.Parallel()
	rawA := oauthKey("SOLO")
	h := newStickyHandler(callback.NewInMemoryRateLimitStore())

	picked := h.stickySelect(context.Background(), "anthropic", "", []nativeUpstream{stickyUpstream(rawA)})
	assert.Equal(t, rawA, picked.APIKey)
	assert.Empty(t, h.stickyState, "single-upstream fast path should not touch stickyState")
}

// Unknown token (no store entry) has no 7d reset → treated as farthest reset
// (math.MaxInt64), so it loses to any token with a known reset timestamp.
func TestStickySelect_UnknownTokenLosesToKnownReset(t *testing.T) {
	t.Parallel()
	rawKnown, rawNew := oauthKey("KNOWN"), oauthKey("NEW")
	hashKnown := callback.RateLimitCacheKey(rawKnown)

	store := callback.NewInMemoryRateLimitStore()
	// Known token has a real reset timestamp — should beat the unknown token.
	store.Set(hashKnown, stateWith7d(hashKnown, 0.9, 24*time.Hour))

	h := newStickyHandler(store)
	upstreams := []nativeUpstream{stickyUpstream(rawKnown), stickyUpstream(rawNew)}

	picked := h.stickySelect(context.Background(), "anthropic", "", upstreams)
	assert.Equal(t, rawKnown, picked.APIKey,
		"unknown token has no reset timestamp (treated as farthest) and loses to any known token")
}

// Cold cluster: all tokens unknown. All get MaxInt64 sentinel; first upstream wins
// because strict < does not displace on tie.
func TestStickySelect_AllUnknown_FirstWins(t *testing.T) {
	t.Parallel()
	rawA, rawB := oauthKey("UNK-A"), oauthKey("UNK-B")

	h := newStickyHandler(callback.NewInMemoryRateLimitStore())
	upstreams := []nativeUpstream{stickyUpstream(rawA), stickyUpstream(rawB)}

	picked := h.stickySelect(context.Background(), "anthropic", "", upstreams)
	assert.Equal(t, rawA, picked.APIKey, "all unknown → first token wins (equal sentinel, strict < preserves first)")
}

// When the 5h window resets (new Unified5hReset timestamp), sticky re-evaluates
// instead of blindly reusing the current token. This prevents draining one
// token's 7d across many 5h cycles.
func TestStickySelect_ReevalsOn5hReset(t *testing.T) {
	t.Parallel()
	rawA, rawB := oauthKey("REEVAL-A"), oauthKey("REEVAL-B")
	hashA, hashB := callback.RateLimitCacheKey(rawA), callback.RateLimitCacheKey(rawB)

	reset5h := fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix())
	store := callback.NewInMemoryRateLimitStore()
	// A resets 7d in 5d, B resets 7d in 1d. B should be preferred (soonest reset).
	store.Set(hashA, callback.AnthropicOAuthRateLimitState{
		TokenKey: hashA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified7dUtilization: 0.3, Unified7dReset: fmt.Sprintf("%d", time.Now().Add(5*24*time.Hour).Unix()),
		Unified7dSonnetUtilization: -1, Unified5hReset: reset5h,
	})
	store.Set(hashB, callback.AnthropicOAuthRateLimitState{
		TokenKey: hashB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified7dUtilization: 0.5, Unified7dReset: fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()),
		Unified7dSonnetUtilization: -1, Unified5hReset: reset5h,
	})

	h := newStickyHandler(store)
	upstreams := []nativeUpstream{stickyUpstream(rawA), stickyUpstream(rawB)}

	// First pick: B wins (soonest 7d reset).
	first := h.stickySelect(context.Background(), "anthropic", "", upstreams)
	assert.Equal(t, rawB, first.APIKey)

	// Same 5h window → sticky reuses B.
	second := h.stickySelect(context.Background(), "anthropic", "", upstreams)
	assert.Equal(t, rawB, second.APIKey, "same 5h window → no re-eval")

	// Simulate 5h reset: change B's Unified5hReset to a new timestamp.
	newReset5h := fmt.Sprintf("%d", time.Now().Add(5*time.Hour).Unix())
	store.Set(hashB, callback.AnthropicOAuthRateLimitState{
		TokenKey: hashB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified7dUtilization: 0.5, Unified7dReset: fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()),
		Unified7dSonnetUtilization: -1, Unified5hReset: newReset5h,
	})

	// Now sticky should re-evaluate. B still has soonest 7d reset → picks B again,
	// but the point is that it DID re-evaluate (new entry recorded).
	third := h.stickySelect(context.Background(), "anthropic", "", upstreams)
	assert.Equal(t, rawB, third.APIKey, "B still best after re-eval")
	assert.Equal(t, newReset5h, h.stickyState["anthropic:all"].Reset5hAt, "entry must record new 5h reset")
}

// When 5h resets and a different token now has a sooner 7d reset, sticky switches.
func TestStickySelect_SwitchesOnReeval(t *testing.T) {
	t.Parallel()
	rawA, rawB := oauthKey("SWITCH-A"), oauthKey("SWITCH-B")
	hashA, hashB := callback.RateLimitCacheKey(rawA), callback.RateLimitCacheKey(rawB)

	reset5h := fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix())
	store := callback.NewInMemoryRateLimitStore()
	// Initially A has sooner 7d reset.
	store.Set(hashA, callback.AnthropicOAuthRateLimitState{
		TokenKey: hashA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified7dUtilization: 0.3, Unified7dReset: fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()),
		Unified7dSonnetUtilization: -1, Unified5hReset: reset5h,
	})
	store.Set(hashB, callback.AnthropicOAuthRateLimitState{
		TokenKey: hashB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified7dUtilization: 0.2, Unified7dReset: fmt.Sprintf("%d", time.Now().Add(5*24*time.Hour).Unix()),
		Unified7dSonnetUtilization: -1, Unified5hReset: reset5h,
	})

	h := newStickyHandler(store)
	upstreams := []nativeUpstream{stickyUpstream(rawA), stickyUpstream(rawB)}

	first := h.stickySelect(context.Background(), "anthropic", "", upstreams)
	assert.Equal(t, rawA, first.APIKey, "A has soonest 7d reset initially")

	// Simulate: 5h reset happens, AND B now has sooner 7d reset than A.
	newReset5h := fmt.Sprintf("%d", time.Now().Add(5*time.Hour).Unix())
	store.Set(hashA, callback.AnthropicOAuthRateLimitState{
		TokenKey: hashA, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified7dUtilization: 0.6, Unified7dReset: fmt.Sprintf("%d", time.Now().Add(5*24*time.Hour).Unix()),
		Unified7dSonnetUtilization: -1, Unified5hReset: newReset5h,
	})
	store.Set(hashB, callback.AnthropicOAuthRateLimitState{
		TokenKey: hashB, UnifiedStatus: callback.UnifiedStatusAllowed,
		Unified7dUtilization: 0.4, Unified7dReset: fmt.Sprintf("%d", time.Now().Add(12*time.Hour).Unix()),
		Unified7dSonnetUtilization: -1, Unified5hReset: newReset5h,
	})

	picked := h.stickySelect(context.Background(), "anthropic", "", upstreams)
	assert.Equal(t, rawB, picked.APIKey, "after 5h reset, B now has soonest 7d reset → switch")
}

// sticky5hReset returns false when entry.Reset5hAt is empty.
func TestSticky5hReset_EmptyReset5hAtReturnsFalse(t *testing.T) {
	t.Parallel()
	h := newStickyHandler(callback.NewInMemoryRateLimitStore())
	assert.False(t, h.sticky5hReset(stickyEntry{APIKey: oauthKey("ANY"), Reset5hAt: ""}))
}

// Production bug reproduction (2026-04-14): sticky re-evaluation after 5h reset
// must pick the token with the soonest 7d reset. Token A (near 7d reset) must beat
// Token B (far 7d reset) regardless of 5h utilization levels.
//
// Old behavior (TimeWeightedScore): B won because low 5h util → low score.
// New behavior (soonest 7d reset): A wins because 2d reset < 5d reset.
func TestStickySelect_PicksSoonest7dResetOnReeval(t *testing.T) {
	t.Parallel()
	rawA, rawB := oauthKey("PROD-A"), oauthKey("PROD-B")
	hashA, hashB := callback.RateLimitCacheKey(rawA), callback.RateLimitCacheKey(rawB)
	now := time.Now()

	// Initial 5h reset (old)
	oldReset5h := fmt.Sprintf("%d", now.Add(-1*time.Hour).Unix())
	// Post-reset 5h timestamp for A (simulating 5h cycle)
	newReset5h := fmt.Sprintf("%d", now.Add(5*time.Hour).Unix())

	store := callback.NewInMemoryRateLimitStore()
	// Token A after 5h reset: high 5h util but 7d reset is SOON (2d). Must win.
	store.Set(hashA, callback.AnthropicOAuthRateLimitState{
		TokenKey:                   hashA,
		UnifiedStatus:              callback.UnifiedStatusAllowed,
		Unified5hUtilization:       0.70,
		Unified5hReset:             newReset5h,
		Unified7dUtilization:       0.59,
		Unified7dReset:             fmt.Sprintf("%d", now.Add(2*24*time.Hour).Unix()),
		Unified7dSonnetUtilization: -1,
	})
	// Token B: low 5h util but 7d reset is FAR (5d). Must lose.
	store.Set(hashB, callback.AnthropicOAuthRateLimitState{
		TokenKey:                   hashB,
		UnifiedStatus:              callback.UnifiedStatusAllowed,
		Unified5hUtilization:       0.15,
		Unified5hReset:             fmt.Sprintf("%d", now.Add(3*time.Hour).Unix()),
		Unified7dUtilization:       0.36,
		Unified7dReset:             fmt.Sprintf("%d", now.Add(5*24*time.Hour).Unix()),
		Unified7dSonnetUtilization: -1,
	})

	h := newStickyHandler(store)
	// Simulate pre-existing sticky entry locked to A with the OLD 5h reset timestamp.
	// sticky5hReset() detects that A's live Unified5hReset (newReset5h) differs from
	// entry.Reset5hAt (oldReset5h), triggering re-evaluation.
	h.stickyState = map[string]stickyEntry{"anthropic:all": {APIKey: rawA, Reset5hAt: oldReset5h}}

	upstreams := []nativeUpstream{stickyUpstream(rawA), stickyUpstream(rawB)}
	picked := h.stickySelect(context.Background(), "anthropic", "", upstreams)
	assert.Equal(t, rawA, picked.APIKey,
		"after 5h reset re-eval: A (7d reset in 2d) beats B (7d reset in 5d); 5h utilization is irrelevant")
}

// ─── DB backfill ──────────────────────────────────────────────────────────────

// After a pod restart (empty store), stickySelect must query the DB, back-fill
// the store, and use the recovered state for selection (soonest 7d reset wins).
func TestStickySelect_BackFillsStoreAfterDBHit(t *testing.T) {
	t.Parallel()
	rawA, rawB := oauthKey("DBFILL-A"), oauthKey("DBFILL-B")
	hashA, hashB := callback.RateLimitCacheKey(rawA), callback.RateLimitCacheKey(rawB)

	store := callback.NewInMemoryRateLimitStore()
	// Token B in memory with a far 7d reset → A should win once recovered from DB.
	store.Set(hashB, callback.AnthropicOAuthRateLimitState{
		TokenKey:             hashB,
		Unified5hUtilization: 0.9,
		Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix()),
		Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(5*24*time.Hour).Unix()),
	})
	// Token A NOT in memory — will be recovered from DB with soonest 7d reset.

	dbCallCount := 0
	h := &Handlers{
		Config:         &config.ProxyConfig{NativeUpstreamStrategy: config.StrategySticky},
		RateLimitStore: store,
		RateLimitDB: &countingStubDB{
			onGet: func() { dbCallCount++ },
			state: callback.AnthropicOAuthRateLimitState{
				TokenKey:             hashA,
				Unified5hUtilization: 0.1,
				Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix()),
				Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(12*time.Hour).Unix()), // sooner than B (5d)
			},
		},
	}

	upstreams := []nativeUpstream{stickyUpstream(rawA), stickyUpstream(rawB)}

	// First call: A is memory miss → DB hit → back-fill → A wins (soonest 7d reset).
	picked := h.stickySelect(context.Background(), "anthropic", "", upstreams)
	assert.Equal(t, rawA, picked.APIKey, "DB-recovered token A should win over B (A has soonest 7d reset)")
	assert.Equal(t, 1, dbCallCount, "first call should query DB for token A")

	// Second call is within the same 5h window → sticky just reuses A, no DB needed.
	h.stickySelect(context.Background(), "anthropic", "", upstreams)
	assert.Equal(t, 1, dbCallCount, "sticky reuse path must not re-query DB")

	// Back-fill must use SetClean — A must not appear in the dirty set.
	snap := store.SnapshotDirty()
	assert.NotContains(t, snap, hashA, "DB back-fill must use SetClean — token A must not be in the dirty set")
}

// DB timeout: stickySelect must complete within the 200ms budget. The timed-out
// token has no recovered reset timestamp → treated as farthest reset → loses to
// any token with a known 7d reset.
func TestStickySelect_DBTimeout_TreatsTokenAsFarthest(t *testing.T) {
	t.Parallel()
	rawA, rawB := oauthKey("DBTOUT-A"), oauthKey("DBTOUT-B")
	hashB := callback.RateLimitCacheKey(rawB)

	store := callback.NewInMemoryRateLimitStore()
	// Token B in memory with a known 7d reset — A is a memory miss hitting a slow DB.
	store.Set(hashB, callback.AnthropicOAuthRateLimitState{
		TokenKey:             hashB,
		Unified5hUtilization: 0.5,
		Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix()),
		Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()),
	})

	h := &Handlers{
		Config:         &config.ProxyConfig{NativeUpstreamStrategy: config.StrategySticky},
		RateLimitStore: store,
		RateLimitDB:    &slowStubDB{delay: 2 * time.Second},
	}

	upstreams := []nativeUpstream{stickyUpstream(rawA), stickyUpstream(rawB)}

	start := time.Now()
	// A times out → no reset timestamp → treated as farthest → loses to B.
	picked := h.stickySelect(context.Background(), "anthropic", "", upstreams)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 500*time.Millisecond, "DB timeout must not block beyond 200ms budget per token")
	assert.Equal(t, rawB, picked.APIKey, "timed-out token (farthest reset sentinel) loses to token with known 7d reset")
}

// DB returning a generic error: token has no reset timestamp → treated as farthest
// reset → loses to any token with a known 7d reset.
func TestStickySelect_DBError_TreatsTokenAsFarthest(t *testing.T) {
	t.Parallel()
	rawA, rawB := oauthKey("DBERR-A"), oauthKey("DBERR-B")
	hashB := callback.RateLimitCacheKey(rawB)

	store := callback.NewInMemoryRateLimitStore()
	store.Set(hashB, callback.AnthropicOAuthRateLimitState{
		TokenKey:             hashB,
		Unified5hUtilization: 0.5,
		Unified5hReset:       fmt.Sprintf("%d", time.Now().Add(time.Hour).Unix()),
		Unified7dReset:       fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix()),
	})

	h := &Handlers{
		Config:         &config.ProxyConfig{NativeUpstreamStrategy: config.StrategySticky},
		RateLimitStore: store,
		RateLimitDB:    &errorStubDB{err: fmt.Errorf("db: connection refused")},
	}

	upstreams := []nativeUpstream{stickyUpstream(rawA), stickyUpstream(rawB)}

	// A errors → no reset timestamp → farthest → loses to B.
	picked := h.stickySelect(context.Background(), "anthropic", "", upstreams)
	assert.Equal(t, rawB, picked.APIKey, "DB-error token (farthest reset sentinel) loses to token with known 7d reset")
}
