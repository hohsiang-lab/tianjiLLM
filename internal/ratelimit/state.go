package ratelimit

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// State holds the latest rate limit info for a single provider key.
type State struct {
	TokensLimit       int
	TokensRemaining   int
	TokensReset       time.Time
	RequestsLimit     int
	RequestsRemaining int
	UpdatedAt         time.Time
}

// Store is a thread-safe in-memory store of rate limit states keyed by providerKey.
type Store struct {
	mu    sync.RWMutex
	state map[string]*State
}

// NewStore creates a new Store.
func NewStore() *Store {
	return &Store{state: make(map[string]*State)}
}

// ParseAndUpdate reads Anthropic rate-limit headers and updates the store for providerKey.
// Missing fields preserve their previous values.
func (s *Store) ParseAndUpdate(providerKey string, h http.Header) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, ok := s.state[providerKey]
	if !ok {
		st = &State{}
		s.state[providerKey] = st
	}

	if v := h.Get("anthropic-ratelimit-tokens-limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			st.TokensLimit = n
		}
	}
	if v := h.Get("anthropic-ratelimit-tokens-remaining"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			st.TokensRemaining = n
		}
	}
	if v := h.Get("anthropic-ratelimit-tokens-reset"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			st.TokensReset = t
		}
	}
	if v := h.Get("anthropic-ratelimit-requests-limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			st.RequestsLimit = n
		}
	}
	if v := h.Get("anthropic-ratelimit-requests-remaining"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			st.RequestsRemaining = n
		}
	}
	st.UpdatedAt = time.Now()
}

// Get returns a copy of the State for providerKey.
func (s *Store) Get(providerKey string) (*State, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.state[providerKey]
	if !ok {
		return nil, false
	}
	cp := *st
	return &cp, true
}

// All returns a snapshot of all states.
func (s *Store) All() map[string]State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]State, len(s.state))
	for k, v := range s.state {
		out[k] = *v
	}
	return out
}
