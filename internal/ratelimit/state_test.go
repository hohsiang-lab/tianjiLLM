package ratelimit

import (
	"net/http"
	"testing"
)

func TestParseAndUpdate(t *testing.T) {
	store := NewStore()
	h := http.Header{}
	h.Set("anthropic-ratelimit-tokens-limit", "800000")
	h.Set("anthropic-ratelimit-tokens-remaining", "120000")
	h.Set("anthropic-ratelimit-tokens-reset", "2026-03-01T00:00:00Z")
	h.Set("anthropic-ratelimit-requests-limit", "1000")
	h.Set("anthropic-ratelimit-requests-remaining", "50")

	store.ParseAndUpdate("anthropic/sk-xxxx", h)

	st, ok := store.Get("anthropic/sk-xxxx")
	if !ok {
		t.Fatal("expected state to exist")
	}
	if st.TokensLimit != 800000 {
		t.Errorf("TokensLimit = %d, want 800000", st.TokensLimit)
	}
	if st.TokensRemaining != 120000 {
		t.Errorf("TokensRemaining = %d, want 120000", st.TokensRemaining)
	}
	if st.RequestsLimit != 1000 {
		t.Errorf("RequestsLimit = %d, want 1000", st.RequestsLimit)
	}
	if st.RequestsRemaining != 50 {
		t.Errorf("RequestsRemaining = %d, want 50", st.RequestsRemaining)
	}
}

func TestParseAndUpdate_PartialHeaders(t *testing.T) {
	store := NewStore()
	// First populate full state
	h1 := http.Header{}
	h1.Set("anthropic-ratelimit-tokens-limit", "800000")
	h1.Set("anthropic-ratelimit-tokens-remaining", "500000")
	store.ParseAndUpdate("key1", h1)

	// Partial update: only remaining changes
	h2 := http.Header{}
	h2.Set("anthropic-ratelimit-tokens-remaining", "200000")
	store.ParseAndUpdate("key1", h2)

	st, _ := store.Get("key1")
	if st.TokensLimit != 800000 {
		t.Errorf("TokensLimit should be preserved, got %d", st.TokensLimit)
	}
	if st.TokensRemaining != 200000 {
		t.Errorf("TokensRemaining = %d, want 200000", st.TokensRemaining)
	}
}

func TestStore_EmptyGet(t *testing.T) {
	store := NewStore()
	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent key")
	}
}

func TestStore_All(t *testing.T) {
	store := NewStore()
	h := http.Header{}
	h.Set("anthropic-ratelimit-tokens-limit", "1000")
	h.Set("anthropic-ratelimit-tokens-remaining", "100")
	store.ParseAndUpdate("key1", h)
	store.ParseAndUpdate("key2", h)

	all := store.All()
	if len(all) != 2 {
		t.Errorf("All() len = %d, want 2", len(all))
	}
}
