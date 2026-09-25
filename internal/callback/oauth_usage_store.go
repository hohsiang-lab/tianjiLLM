package callback

import (
	"sync"
	"time"
)

// OAuthUsageState holds per-token usage data from the Anthropic OAuth Usage API
// or parsed from response headers. Used by the Claude Code tab.
// Usage fields are *float64: nil = unknown/missing, non-nil = [0,1] fraction.
type OAuthUsageState struct {
	TokenKey string

	// Session (5h window)
	SessionUsage   *float64 // nil = unknown; [0,1] fraction
	SessionResetAt string   // ISO 8601 (from Usage API) or raw unix timestamp (from response headers)

	// Weekly (7d window) — all models
	WeeklyUsage   *float64
	WeeklyResetAt string

	// Weekly (7d window) — Sonnet only (headers only; Usage API does not provide this)
	SonnetUsage   *float64
	SonnetResetAt string

	// Extra usage (overage)
	ExtraUsageEnabled     *bool
	ExtraUsageLimit       *float64
	ExtraUsageUsed        *float64
	ExtraUsageUtilization *float64

	// Overage status from headers
	OverageStatus         string
	OverageDisabledReason string

	// Organization ID (from response headers, not Usage API)
	OrgID string

	// Error state (reserved; currently always "" since SetFromHeaders clears it)
	Error string

	// Timestamps
	FetchedAt time.Time
}

// OAuthUsageStore stores per-token OAuth usage state for the Claude Code tab.
type OAuthUsageStore interface {
	Get(tokenKey string) (OAuthUsageState, bool)
	Set(tokenKey string, state OAuthUsageState)
	SetFromHeaders(tokenKey string, rl AnthropicOAuthRateLimitState)
	GetAll() map[string]OAuthUsageState
}

// InMemoryOAuthUsageStore is a thread-safe in-memory implementation.
type InMemoryOAuthUsageStore struct {
	mu    sync.RWMutex
	store map[string]OAuthUsageState
}

func NewInMemoryOAuthUsageStore() *InMemoryOAuthUsageStore {
	return &InMemoryOAuthUsageStore{store: make(map[string]OAuthUsageState)}
}

func (s *InMemoryOAuthUsageStore) Get(tokenKey string) (OAuthUsageState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.store[tokenKey]
	return state, ok
}

func (s *InMemoryOAuthUsageStore) Set(tokenKey string, state OAuthUsageState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state.TokenKey = tokenKey
	s.store[tokenKey] = state
}

func (s *InMemoryOAuthUsageStore) SetFromHeaders(tokenKey string, rl AnthropicOAuthRateLimitState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.store[tokenKey] // zero value is fine: all *float64 are nil = unknown
	existing.TokenKey = tokenKey
	existing.FetchedAt = time.Now()

	if rl.Unified5hUtilization >= 0 {
		existing.SessionUsage = &rl.Unified5hUtilization
	}
	existing.SessionResetAt = rl.Unified5hReset

	if rl.Unified7dUtilization >= 0 {
		existing.WeeklyUsage = &rl.Unified7dUtilization
	}
	existing.WeeklyResetAt = rl.Unified7dReset

	if rl.Unified7dSonnetUtilization >= 0 {
		existing.SonnetUsage = &rl.Unified7dSonnetUtilization
	}
	existing.SonnetResetAt = rl.Unified7dSonnetReset

	existing.OverageStatus = rl.UnifiedStatus
	existing.OverageDisabledReason = rl.OverageDisabledReason

	if rl.OrganizationID != "" {
		existing.OrgID = rl.OrganizationID
	}

	existing.Error = ""
	s.store[tokenKey] = existing
}

func (s *InMemoryOAuthUsageStore) GetAll() map[string]OAuthUsageState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]OAuthUsageState, len(s.store))
	for k, v := range s.store {
		out[k] = v
	}
	return out
}
