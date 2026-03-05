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
// Supports both the legacy per-type headers (requests/tokens) and the unified OAuth headers
// (unified-5h/7d utilization + status) returned by Anthropic for OAuth tokens.
// Integer fields use -1 as sentinel when missing or unparseable.
type AnthropicOAuthRateLimitState struct {
	// Cache key derived from the token (set at parse time).
	TokenKey string

	// Legacy per-type headers (present for both API keys and OAuth tokens on some responses).
	RequestsLimit     int
	RequestsRemaining int
	TokensLimit       int
	TokensRemaining   int
	RequestsResetAt   string
	TokensResetAt     string

	// Unified OAuth headers (present for OAuth tokens).
	UnifiedStatus         string // "allowed", "rate_limited", "overage", etc.
	UnifiedReset          string // raw unix timestamp string
	Unified5hStatus       string
	Unified5hReset        string
	Unified5hUtilization  float64 // fraction [0,1]; -1 = missing or unparseable
	Unified7dStatus       string
	Unified7dReset        string
	Unified7dUtilization  float64 // fraction [0,1]; -1 = missing or unparseable
	RepresentativeClaim   string  // "five_hour" or "seven_day"
	FallbackPercentage    float64 // fraction [0,1]; -1 = missing
	OverageDisabledReason string

	ParsedAt time.Time
}

// RateLimitCacheKey returns a short cache key derived from the token (sha256[:12]).
func RateLimitCacheKey(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h[:6])
}

// ParseAnthropicOAuthRateLimitHeaders parses Anthropic rate limit headers for a specific token.
// Must be called on ALL response statuses (including 429), not only 200.
func ParseAnthropicOAuthRateLimitHeaders(h http.Header, tokenKey string) AnthropicOAuthRateLimitState {
	state := AnthropicOAuthRateLimitState{
		TokenKey:             tokenKey,
		ParsedAt:             time.Now(),
		Unified5hUtilization: -1,
		Unified7dUtilization: -1,
		FallbackPercentage:   -1,
	}

	parseInt := func(name string) int {
		raw := h.Get(name)
		if raw == "" {
			return -1
		}
		v, err := strconv.Atoi(raw)
		if err != nil {
			return -1
		}
		return v
	}
	parseFloat := func(name string) float64 {
		raw := h.Get(name)
		if raw == "" {
			return -1
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return -1
		}
		return v
	}

	// Legacy per-type headers.
	state.RequestsLimit = parseInt("anthropic-ratelimit-requests-limit")
	state.RequestsRemaining = parseInt("anthropic-ratelimit-requests-remaining")
	state.TokensLimit = parseInt("anthropic-ratelimit-tokens-limit")
	state.TokensRemaining = parseInt("anthropic-ratelimit-tokens-remaining")
	state.RequestsResetAt = h.Get("anthropic-ratelimit-requests-reset")
	state.TokensResetAt = h.Get("anthropic-ratelimit-tokens-reset")

	// Unified OAuth headers.
	state.UnifiedStatus = h.Get("anthropic-ratelimit-unified-status")
	state.UnifiedReset = h.Get("anthropic-ratelimit-unified-reset")
	state.Unified5hStatus = h.Get("anthropic-ratelimit-unified-5h-status")
	state.Unified5hReset = h.Get("anthropic-ratelimit-unified-5h-reset")
	state.Unified5hUtilization = parseFloat("anthropic-ratelimit-unified-5h-utilization")
	state.Unified7dStatus = h.Get("anthropic-ratelimit-unified-7d-status")
	state.Unified7dReset = h.Get("anthropic-ratelimit-unified-7d-reset")
	state.Unified7dUtilization = parseFloat("anthropic-ratelimit-unified-7d-utilization")
	state.RepresentativeClaim = h.Get("anthropic-ratelimit-unified-representative-claim")
	state.FallbackPercentage = parseFloat("anthropic-ratelimit-unified-fallback-percentage")
	state.OverageDisabledReason = h.Get("anthropic-ratelimit-unified-overage-disabled-reason")

	return state
}

// RateLimitStore is the interface for storing per-token rate limit state.
type RateLimitStore interface {
	Set(key string, state AnthropicOAuthRateLimitState)
	Get(key string) (AnthropicOAuthRateLimitState, bool)
	GetAll() map[string]AnthropicOAuthRateLimitState
	Prune(ttl time.Duration)
	// GetUtilization returns the 5h utilization percentage (0-100) for a token key.
	// Returns (utilization, true) if data exists, or (0, false) if missing.
	GetUtilization(tokenKey string) (float64, bool)
	// GetLowestUtilization returns the token key with the lowest 5h utilization
	// among the given keys. Skips keys with no data or rate_limited status.
	// Returns ("", -1) if no valid candidate exists.
	GetLowestUtilization(tokenKeys []string) (string, float64)
}

type rateLimitEntry struct {
	state     AnthropicOAuthRateLimitState
	updatedAt time.Time
}

// InMemoryRateLimitStore is a thread-safe in-memory implementation of RateLimitStore.
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

func (s *InMemoryRateLimitStore) GetUtilization(tokenKey string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[tokenKey]
	if !ok || e.state.Unified5hUtilization < 0 {
		return 0, false
	}
	return e.state.Unified5hUtilization * 100, true
}

func (s *InMemoryRateLimitStore) GetLowestUtilization(tokenKeys []string) (string, float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bestKey := ""
	bestUtil := float64(-1)
	for _, key := range tokenKeys {
		e, ok := s.entries[key]
		if !ok {
			continue
		}
		if e.state.Unified5hStatus == "rate_limited" {
			continue
		}
		if e.state.Unified5hUtilization < 0 {
			continue
		}
		util := e.state.Unified5hUtilization * 100
		if bestKey == "" || util < bestUtil {
			bestKey = key
			bestUtil = util
		}
	}
	return bestKey, bestUtil
}
