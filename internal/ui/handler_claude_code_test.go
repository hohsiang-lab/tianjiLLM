package ui

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

// stubRateLimitDB is a test-only stub for rateLimitStateReader.
type stubRateLimitDB struct {
	state db.OAuthTokenRateLimitState
	err   error
}

func (s *stubRateLimitDB) GetOAuthTokenRateLimitState(_ context.Context, _ string) (db.OAuthTokenRateLimitState, error) {
	return s.state, s.err
}

func ptr[T any](v T) *T { return &v }

func oauthKey(s string) *string { return ptr(s) }

func buildClaudeCodeHandler(tokens ...string) *UIHandler {
	var models []config.ModelConfig
	for _, tok := range tokens {
		models = append(models, config.ModelConfig{
			TianjiParams: config.TianjiParams{
				Model:  "anthropic/claude-sonnet-4-5",
				APIKey: oauthKey(tok),
			},
		})
	}
	return &UIHandler{
		Config:         &config.ProxyConfig{ModelList: models},
		RateLimitStore: callback.NewInMemoryRateLimitStore(),
	}
}

func TestEnumerateOAuthTokens_NilConfig(t *testing.T) {
	h := &UIHandler{}
	tokens := h.enumerateOAuthTokens()
	if tokens != nil {
		t.Errorf("expected nil tokens for nil config, got %v", tokens)
	}
}

func TestEnumerateOAuthTokens_FiltersNonOAuth(t *testing.T) {
	h := &UIHandler{
		Config: &config.ProxyConfig{
			ModelList: []config.ModelConfig{
				{TianjiParams: config.TianjiParams{Model: "anthropic/claude-sonnet-4-5", APIKey: ptr("sk-regular-key")}},
				{TianjiParams: config.TianjiParams{Model: "openai/gpt-4o", APIKey: ptr("sk-ant-oat01-valid-oauth-token-that-is-long-enough-to-pass-validation-abcdefghijk")}},
				{TianjiParams: config.TianjiParams{Model: "anthropic/claude-sonnet-4-5"}}, // nil key
			},
		},
	}
	tokens := h.enumerateOAuthTokens()
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens (non-anthropic and non-OAuth filtered), got %d", len(tokens))
	}
}

func TestEnumerateOAuthTokens_DeduplicatesSameToken(t *testing.T) {
	tok := "sk-ant-oat01-valid-oauth-token-that-is-long-enough-to-pass-validation-abcdefghijk"
	h := buildClaudeCodeHandler(tok, tok)
	tokens := h.enumerateOAuthTokens()
	if len(tokens) != 1 {
		t.Errorf("expected 1 token (deduplicated), got %d", len(tokens))
	}
}

func TestLoadClaudeCodeData_EmptyConfig(t *testing.T) {
	h := &UIHandler{Config: &config.ProxyConfig{}}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	data := h.loadClaudeCodeData(r, nil)
	if len(data.Cards) != 0 {
		t.Errorf("expected 0 cards, got %d", len(data.Cards))
	}
}

func TestLoadClaudeCodeData_WithStoreData(t *testing.T) {
	tok := "sk-ant-oat01-valid-oauth-token-that-is-long-enough-to-pass-validation-abcdefghijk"
	h := buildClaudeCodeHandler(tok)

	key := callback.RateLimitCacheKey(tok)
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		Unified5hUtilization: 0.42,
		Unified7dUtilization: 0.31,
		ParsedAt:             time.Now(),
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	data := h.loadClaudeCodeData(r, nil)

	if len(data.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(data.Cards))
	}
	card := data.Cards[0]
	if card.SessionUsage == nil || *card.SessionUsage != 0.42 {
		t.Errorf("SessionUsage = %v, want 0.42", card.SessionUsage)
	}
	if card.WeeklyUsage == nil || *card.WeeklyUsage != 0.31 {
		t.Errorf("WeeklyUsage = %v, want 0.31", card.WeeklyUsage)
	}
	if card.Error != "" {
		t.Errorf("Error = %q, want empty", card.Error)
	}
	// TimeWeightedScore with no reset times: tau=1 fallback -> score = 1/(1-r).
	wantComposite := 1.0 / (1.0 - 0.42/callback.DefaultGate5h)
	if card.CompositeScore == nil {
		t.Errorf("CompositeScore is nil, want ~%.4f", wantComposite)
	} else if math.Abs(*card.CompositeScore-wantComposite) > 1e-9 {
		t.Errorf("CompositeScore = %.10f, want %.10f", *card.CompositeScore, wantComposite)
	}
}

func TestLoadClaudeCodeData_CompositeScore_OneNilField(t *testing.T) {
	tok := "sk-ant-oat01-valid-oauth-token-that-is-long-enough-to-pass-validation-abcdefghijk"
	h := buildClaudeCodeHandler(tok)

	key := callback.RateLimitCacheKey(tok)

	// Only SessionUsage set; WeeklyUsage is nil -> treated as 0.
	// No reset times: tau=1 fallback.
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		Unified5hUtilization: 0.55,
		Unified7dUtilization: -1,
		ParsedAt:             time.Now(),
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	data := h.loadClaudeCodeData(r, nil)

	if len(data.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(data.Cards))
	}
	card := data.Cards[0]
	wantComposite := 1.0 / (1.0 - 0.55/callback.DefaultGate5h)
	if card.CompositeScore == nil {
		t.Errorf("CompositeScore is nil when only SessionUsage is set, want %.4f", wantComposite)
	} else if math.Abs(*card.CompositeScore-wantComposite) > 1e-9 {
		t.Errorf("CompositeScore = %.10f, want %.10f", *card.CompositeScore, wantComposite)
	}

	// Symmetric: only WeeklyUsage set; SessionUsage is nil → treated as 0.
	// 7d: r=0.40/0.9=0.444, 1/(1-0.444)=1.80. 5h: u=-1 → r=0 → 1/1=1.0. max=1.80
	// Use a fresh handler so the merge logic doesn't carry over 5h from the first assertion.
	h2 := buildClaudeCodeHandler(tok)
	h2.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		Unified5hUtilization: -1,
		Unified7dUtilization: 0.40,
		ParsedAt:             time.Now(),
	})

	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	data2 := h2.loadClaudeCodeData(r2, nil)
	card2 := data2.Cards[0]
	wantComposite2 := 1.0 / (1.0 - 0.40/0.9) // 1/(1-0.444) = 1.80
	if card2.CompositeScore == nil {
		t.Errorf("CompositeScore is nil when only WeeklyUsage is set, want %.4f", wantComposite2)
	} else if math.Abs(*card2.CompositeScore-wantComposite2) > 1e-9 {
		t.Errorf("CompositeScore = %.10f, want %.10f", *card2.CompositeScore, wantComposite2)
	}
}

func TestLoadClaudeCodeData_NilStore(t *testing.T) {
	tok := "sk-ant-oat01-valid-oauth-token-that-is-long-enough-to-pass-validation-abcdefghijk"
	h := &UIHandler{
		Config: &config.ProxyConfig{
			ModelList: []config.ModelConfig{
				{TianjiParams: config.TianjiParams{Model: "anthropic/claude-sonnet-4-5", APIKey: oauthKey(tok)}},
			},
		},
		RateLimitStore: nil, // nil store
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	data := h.loadClaudeCodeData(r, nil)

	if len(data.Cards) != 1 {
		t.Fatalf("expected 1 card even with nil store, got %d", len(data.Cards))
	}
	// Card should have nil usage (no data)
	if data.Cards[0].SessionUsage != nil {
		t.Error("expected nil SessionUsage with nil store")
	}
}

func TestLoadClaudeCodeData_BothNilUsage_CompositeScoreIsNil(t *testing.T) {
	tok := "sk-ant-oat01-valid-oauth-token-that-is-long-enough-to-pass-validation-abcdefghijk"
	h := buildClaudeCodeHandler(tok)

	key := callback.RateLimitCacheKey(tok)
	// Both SessionUsage and WeeklyUsage are nil — no data to compute a score from.
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		Unified5hUtilization: -1,
		Unified7dUtilization: -1,
		ParsedAt:             time.Now(),
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	data := h.loadClaudeCodeData(r, nil)

	if len(data.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(data.Cards))
	}
	if data.Cards[0].CompositeScore != nil {
		t.Errorf("CompositeScore = %v, want nil when both SessionUsage and WeeklyUsage are nil", data.Cards[0].CompositeScore)
	}
}

func TestHandleClaudeCodeAPI_JSONResponse(t *testing.T) {
	tok := "sk-ant-oat01-valid-oauth-token-that-is-long-enough-to-pass-validation-abcdefghijk"
	h := buildClaudeCodeHandler(tok)

	key := callback.RateLimitCacheKey(tok)
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		Unified5hUtilization: 0.5,
		Unified7dUtilization: -1,
		ParsedAt:             time.Now(),
	})

	r := httptest.NewRequest(http.MethodGet, "/ui/api/claude-code-usage", nil)
	w := httptest.NewRecorder()
	h.handleClaudeCodeAPI(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want 'application/json'", ct)
	}

	var cards []pages.ClaudeCodeTokenCard
	if err := json.NewDecoder(w.Body).Decode(&cards); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
}

func TestLoadClaudeCodeData_RuntimeStoreWinsOverDBParam(t *testing.T) {
	tok := "sk-ant-oat01-valid-oauth-token-that-is-long-enough-to-pass-validation-abcdefghijk"
	h := &UIHandler{
		Config:         &config.ProxyConfig{ModelList: []config.ModelConfig{{TianjiParams: config.TianjiParams{Model: "anthropic/claude-sonnet-4-5", APIKey: oauthKey(tok)}}}},
		RateLimitStore: nil,
	}

	key := callback.RateLimitCacheKey(tok)
	store := callback.NewInMemoryRateLimitStore()
	store.Set(key, callback.AnthropicOAuthRateLimitState{
		Unified5hUtilization: 0.61,
		Unified7dUtilization: 0.27,
		ParsedAt:             time.Now(),
	})
	// Assign RateLimitStore after construction to verify loadClaudeCodeData reads the
	// current field value rather than a captured snapshot.
	h.RateLimitStore = store

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	data := h.loadClaudeCodeData(r, &stubRateLimitDB{err: errors.New("should not be called when runtime store has data")})

	if len(data.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(data.Cards))
	}
	card := data.Cards[0]
	if card.SessionUsage == nil || *card.SessionUsage != 0.61 {
		t.Fatalf("SessionUsage = %v, want 0.61 from runtime store", card.SessionUsage)
	}
	if card.WeeklyUsage == nil || *card.WeeklyUsage != 0.27 {
		t.Fatalf("WeeklyUsage = %v, want 0.27 from runtime store", card.WeeklyUsage)
	}
	if card.Error != "" {
		t.Fatalf("Error = %q, want empty", card.Error)
	}
}

func TestHandleClaudeCodeAPI_EmptyReturnsEmptyArray(t *testing.T) {
	h := &UIHandler{Config: &config.ProxyConfig{}}

	r := httptest.NewRequest(http.MethodGet, "/ui/api/claude-code-usage", nil)
	w := httptest.NewRecorder()
	h.handleClaudeCodeAPI(w, r)

	// Should return null (nil cards marshals to null), not error.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestLoadClaudeCodeData_DBFallback_HitWhenStoreMiss verifies that when RateLimitStore
// has no entry for a token, loadClaudeCodeData falls back to the DB and uses the
// returned state to populate SessionUsage and WeeklyUsage.
func TestLoadClaudeCodeData_DBFallback_HitWhenStoreMiss(t *testing.T) {
	tok := "sk-ant-oat01-valid-oauth-token-that-is-long-enough-to-pass-validation-abcdefghijk"
	key := callback.RateLimitCacheKey(tok)

	h := buildClaudeCodeHandler(tok)
	// RateLimitStore is empty — simulates a restart/TTL eviction.
	stub := &stubRateLimitDB{
		state: db.OAuthTokenRateLimitState{
			TokenKey:             key,
			Unified5hUtilization: 0.42,
			Unified7dUtilization: 0.30,
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	data := h.loadClaudeCodeData(r, stub)

	if len(data.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(data.Cards))
	}
	card := data.Cards[0]
	if card.SessionUsage == nil || math.Abs(*card.SessionUsage-0.42) > 1e-9 {
		t.Errorf("SessionUsage = %v, want 0.42 (from DB fallback)", card.SessionUsage)
	}
	if card.WeeklyUsage == nil || math.Abs(*card.WeeklyUsage-0.30) > 1e-9 {
		t.Errorf("WeeklyUsage = %v, want 0.30 (from DB fallback)", card.WeeklyUsage)
	}
}

// TestLoadClaudeCodeData_DBFallback_ExpiredWindowZerosUsage verifies that when the
// DB fallback is used and a reset window has already passed, the corresponding usage
// is zeroed out rather than showing the stale pre-reset value (HO-568).
// This matches the behavior of InMemoryRateLimitStore.Get(), which applies
// NormalizeExpiredWindows before returning.
func TestLoadClaudeCodeData_DBFallback_ExpiredWindowZerosUsage(t *testing.T) {
	tok := "sk-ant-oat01-valid-oauth-token-that-is-long-enough-to-pass-validation-abcdefghijk"
	key := callback.RateLimitCacheKey(tok)

	expiredReset := strconv.FormatInt(time.Now().Add(-1*time.Hour).Unix(), 10) // 1h ago
	futureReset := strconv.FormatInt(time.Now().Add(6*24*time.Hour).Unix(), 10)

	h := buildClaudeCodeHandler(tok)
	stub := &stubRateLimitDB{
		state: db.OAuthTokenRateLimitState{
			TokenKey:             key,
			Unified5hUtilization: 0.92, // stale — window has expired
			Unified5hReset:       expiredReset,
			Unified7dUtilization: 0.30, // still active
			Unified7dReset:       futureReset,
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	data := h.loadClaudeCodeData(r, stub)

	if len(data.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(data.Cards))
	}
	card := data.Cards[0]

	// 5h window expired → usage must be 0, not the stale 0.92.
	if card.SessionUsage == nil || math.Abs(*card.SessionUsage-0.0) > 1e-9 {
		t.Errorf("SessionUsage = %v, want 0.0 (expired window should be zeroed)", card.SessionUsage)
	}
	// 7d window still active → usage is preserved.
	if card.WeeklyUsage == nil || math.Abs(*card.WeeklyUsage-0.30) > 1e-9 {
		t.Errorf("WeeklyUsage = %v, want 0.30 (active window should be preserved)", card.WeeklyUsage)
	}
}

// TestLoadClaudeCodeData_DBFallback_StoreMissNotFoundIsNoOp verifies that a
// pgx.ErrNoRows response from the DB fallback does not log a warning and leaves
// the card with nil SessionUsage/WeeklyUsage (normal for brand-new tokens).
func TestLoadClaudeCodeData_DBFallback_StoreMissNotFoundIsNoOp(t *testing.T) {
	tok := "sk-ant-oat01-valid-oauth-token-that-is-long-enough-to-pass-validation-abcdefghijk"

	h := buildClaudeCodeHandler(tok)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	data := h.loadClaudeCodeData(r, &stubRateLimitDB{err: callback.ErrRateLimitStateNotFound})

	if len(data.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(data.Cards))
	}
	card := data.Cards[0]
	if card.SessionUsage != nil || card.WeeklyUsage != nil {
		t.Errorf("expected nil usage for ErrNoRows, got session=%v weekly=%v", card.SessionUsage, card.WeeklyUsage)
	}
}

// TestLoadClaudeCodeData_DBFallback_StoreWinsOverDB verifies that when RateLimitStore
// has an entry, the DB fallback is NOT consulted (store data takes priority).
func TestLoadClaudeCodeData_DBFallback_StoreWinsOverDB(t *testing.T) {
	tok := "sk-ant-oat01-valid-oauth-token-that-is-long-enough-to-pass-validation-abcdefghijk"
	key := callback.RateLimitCacheKey(tok)

	h := buildClaudeCodeHandler(tok)
	h.RateLimitStore.Set(key, callback.AnthropicOAuthRateLimitState{
		Unified5hUtilization: 0.55,
		Unified7dUtilization: -1,
		ParsedAt:             time.Now(),
	})
	// DB would return different values — should never be called.
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	data := h.loadClaudeCodeData(r, &stubRateLimitDB{err: errors.New("should not be called")})

	if len(data.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(data.Cards))
	}
	card := data.Cards[0]
	if card.SessionUsage == nil || math.Abs(*card.SessionUsage-0.55) > 1e-9 {
		t.Errorf("SessionUsage = %v, want 0.55 (from store, not DB)", card.SessionUsage)
	}
}

// TestLoadClaudeCodeData_DBFallback_DBErrorIsGraceful verifies that when the DB
// returns an unexpected error (not ErrNoRows), loadClaudeCodeData fails-open:
// it returns a card with nil usage (no panic, no error response) and logs a warning.
func TestLoadClaudeCodeData_DBFallback_DBErrorIsGraceful(t *testing.T) {
	tok := "sk-ant-oat01-valid-oauth-token-that-is-long-enough-to-pass-validation-abcdefghijk"

	h := buildClaudeCodeHandler(tok)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	data := h.loadClaudeCodeData(r, &stubRateLimitDB{err: errors.New("connection reset by peer")})

	if len(data.Cards) != 1 {
		t.Fatalf("expected 1 card (fail-open), got %d", len(data.Cards))
	}
	card := data.Cards[0]
	// Fail-open: card is returned but without usage data.
	if card.SessionUsage != nil || card.WeeklyUsage != nil {
		t.Errorf("expected nil usage on DB error (fail-open), got session=%v weekly=%v",
			card.SessionUsage, card.WeeklyUsage)
	}
	if card.CompositeScore != nil {
		t.Errorf("expected nil CompositeScore on DB error, got %v", card.CompositeScore)
	}
}
