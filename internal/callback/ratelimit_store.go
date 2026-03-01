package callback

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// AnthropicOAuthRateLimitState holds parsed rate limit header values for a single OAuth token.
// TODO: implement — extend with all Anthropic rate limit header fields.
type AnthropicOAuthRateLimitState struct {
	TokenKey           string
	RequestsLimit      int
	RequestsRemaining  int
	TokensLimit        int
	TokensRemaining    int
	RequestsResetAt    string
	TokensResetAt      string
	ParsedAt           time.Time
}

// RateLimitCacheKey returns a short cache key derived from the token (sha256[:12]).
// TODO: implement — used as map key in InMemoryRateLimitStore.
func RateLimitCacheKey(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h[:6])
}

// ParseAnthropicOAuthRateLimitHeaders parses Anthropic rate limit headers for a specific token.
// FR-019: must be called on ALL response statuses (including 429), not only 200.
// TODO: implement — extend header field parsing.
func ParseAnthropicOAuthRateLimitHeaders(h http.Header, tokenKey string) AnthropicOAuthRateLimitState {
	state := AnthropicOAuthRateLimitState{
		TokenKey: tokenKey,
		ParsedAt: time.Now(),
	}
	if v := h.Get("anthropic-ratelimit-requests-limit"); v != "" {
		state.RequestsLimit, _ = strconv.Atoi(v)
	}
	if v := h.Get("anthropic-ratelimit-requests-remaining"); v != "" {
		state.RequestsRemaining, _ = strconv.Atoi(v)
	}
	if v := h.Get("anthropic-ratelimit-tokens-limit"); v != "" {
		state.TokensLimit, _ = strconv.Atoi(v)
	}
	if v := h.Get("anthropic-ratelimit-tokens-remaining"); v != "" {
		state.TokensRemaining, _ = strconv.Atoi(v)
	}
	state.RequestsResetAt = h.Get("anthropic-ratelimit-requests-reset")
	state.TokensResetAt = h.Get("anthropic-ratelimit-tokens-reset")
	return state
}

// RateLimitStore is the interface for storing per-token rate limit state.
// TODO: implement — extend with TTL eviction.
type RateLimitStore interface {
	Set(key string, state AnthropicOAuthRateLimitState)
	Get(key string) (AnthropicOAuthRateLimitState, bool)
	GetAll() map[string]AnthropicOAuthRateLimitState
	Prune(ttl time.Duration)
}

type rateLimitEntry struct {
	state     AnthropicOAuthRateLimitState
	updatedAt time.Time
}

// InMemoryRateLimitStore is a thread-safe in-memory implementation of RateLimitStore.
// TODO: implement — add TTL prune goroutine in server init.
type InMemoryRateLimitStore struct {
	mu      sync.RWMutex
	entries map[string]rateLimitEntry
}

// NewInMemoryRateLimitStore creates an empty InMemoryRateLimitStore.
func NewInMemoryRateLimitStore() *InMemoryRateLimitStore {
	return &InMemoryRateLimitStore{entries: make(map[string]rateLimitEntry)}
}

func (s *InMemoryRateLimitStore) Set(key string, state AnthropicOAuthRateLimitState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = rateLimitEntry{state: state, updatedAt: time.Now()}
}

func (s *InMemoryRateLimitStore) Get(key string) (AnthropicOAuthRateLimitState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[key]
	return e.state, ok
}

func (s *InMemoryRateLimitStore) GetAll() map[string]AnthropicOAuthRateLimitState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]AnthropicOAuthRateLimitState, len(s.entries))
	for k, e := range s.entries {
		out[k] = e.state
	}
	return out
}

func (s *InMemoryRateLimitStore) Prune(ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-ttl)
	for k, e := range s.entries {
		if e.updatedAt.Before(cutoff) {
			delete(s.entries, k)
		}
	}
}
