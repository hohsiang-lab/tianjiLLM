package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func buildStore(limit, remaining int) *Store {
	store := NewStore()
	h := http.Header{}
	h.Set("anthropic-ratelimit-tokens-limit", itoa(limit))
	h.Set("anthropic-ratelimit-tokens-remaining", itoa(remaining))
	store.ParseAndUpdate("anthropic/xxxx", h)
	return store
}

func itoa(n int) string {
	return http.Header{}.Get("") + string(rune('0'+n%10)) // won't work for large nums
}

func buildStoreStr(limit, remaining string) *Store {
	store := NewStore()
	h := http.Header{}
	h.Set("anthropic-ratelimit-tokens-limit", limit)
	h.Set("anthropic-ratelimit-tokens-remaining", remaining)
	store.ParseAndUpdate("anthropic/xxxx", h)
	return store
}

func TestDiscordAlerter_BelowThreshold_Fires(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	alerter := NewDiscordAlerter(srv.URL, 0.20, time.Hour)
	store := buildStoreStr("800000", "100000") // 12.5% < 20% threshold

	alerter.Check("anthropic/xxxx", store)

	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("expected 1 webhook call, got %d", calls)
	}
}

func TestDiscordAlerter_AboveThreshold_NoFire(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	alerter := NewDiscordAlerter(srv.URL, 0.20, time.Hour)
	store := buildStoreStr("800000", "600000") // 75% > 20%

	alerter.Check("anthropic/xxxx", store)

	if atomic.LoadInt32(&calls) != 0 {
		t.Errorf("expected 0 webhook calls, got %d", calls)
	}
}

func TestDiscordAlerter_Cooldown(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	// Very long cooldown so second call is blocked
	alerter := NewDiscordAlerter(srv.URL, 0.20, time.Hour)
	store := buildStoreStr("800000", "100000") // below threshold

	alerter.Check("anthropic/xxxx", store)
	alerter.Check("anthropic/xxxx", store) // should be throttled

	if atomic.LoadInt32(&calls) != 1 {
		t.Errorf("expected 1 webhook call (throttled), got %d", calls)
	}
}

func TestDiscordAlerter_ZeroLimit_NoFire(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	alerter := NewDiscordAlerter(srv.URL, 0.20, time.Hour)
	store := NewStore() // no data
	alerter.Check("anthropic/xxxx", store)

	if atomic.LoadInt32(&calls) != 0 {
		t.Errorf("expected 0 calls for empty store, got %d", calls)
	}
}
