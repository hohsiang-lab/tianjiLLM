package callback

import (
	"crypto/sha256"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// clampUtilization clamps v to [0, 1]. The exact sentinel value -1 (meaning "no data")
// is passed through unchanged. Any other out-of-range value is clamped to [0, 1].
func clampUtilization(v float64) float64 {
	if v == -1 {
		return v
	}
	return math.Max(0, math.Min(1.0, v))
}

// BothWindowsExpired reports whether both the 5h and 7d Anthropic rate-limit
// windows for state have passed as of now. Zero reset times (no header received)
// are treated as NOT expired. Use this instead of duplicating the check.
func BothWindowsExpired(state AnthropicOAuthRateLimitState, now time.Time) bool {
	t5h := ParseResetTime(state.Unified5hReset)
	t7d := ParseResetTime(state.Unified7dReset)
	return !t5h.IsZero() && now.After(t5h) &&
		!t7d.IsZero() && now.After(t7d)
}

// NormalizeExpiredWindows returns a copy of state with expired windows zeroed out.
// If a window's reset time has passed, its utilization is set to 0 and status cleared,
// because Anthropic has already reset the counter — the stored value is stale.
// Zero reset times (no header received) are treated as NOT expired (conservative).
// The unified status is cleared when its own reset has passed.
func NormalizeExpiredWindows(state AnthropicOAuthRateLimitState, now time.Time) AnthropicOAuthRateLimitState {
	if t := ParseResetTime(state.Unified5hReset); !t.IsZero() && now.After(t) {
		state.Unified5hUtilization = 0
		state.Unified5hStatus = ""
	}
	if t := ParseResetTime(state.Unified7dReset); !t.IsZero() && now.After(t) {
		state.Unified7dUtilization = 0
		state.Unified7dStatus = ""
	}
	if t := ParseResetTime(state.Unified7dSonnetReset); !t.IsZero() && now.After(t) {
		state.Unified7dSonnetUtilization = 0
		state.Unified7dSonnetStatus = ""
	}
	if t := ParseResetTime(state.UnifiedReset); !t.IsZero() && now.After(t) {
		state.UnifiedStatus = ""
	}
	return state
}

// ParseResetTime parses a unix timestamp string into time.Time.
// Returns zero time if empty or unparseable.
// Zero time is the canonical signal for "no reset header received yet".
func ParseResetTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		log.Printf("warn: ratelimit: cannot parse reset timestamp %q: %v", s, err)
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

// Unified status values returned in the anthropic-ratelimit-unified-* headers.
//
// Confirmed values from Anthropic OAuth headers (as of 2026-04):
//   - "allowed"         — normal quota available
//   - "allowed_warning" — quota exhausted but extra usage (overage credit) active; requests still accepted
//   - "rejected"        — quota exhausted and no extra usage; requests rejected
//
// Note: "rate_limited" and "overage" are NOT official Anthropic unified-status values.
const (
	UnifiedStatusAllowed        = "allowed"
	UnifiedStatusAllowedWarning = "allowed_warning" // quota exhausted but extra usage (overage credit) is active
	UnifiedStatusRejected       = "rejected"        // quota exhausted; no extra usage available
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
	UnifiedStatus              string // "allowed", "allowed_warning", "rejected" (see UnifiedStatus* consts)
	UnifiedReset               string // raw unix timestamp string
	Unified5hStatus            string
	Unified5hReset             string
	Unified5hUtilization       float64 // fraction [0,1]; -1 = missing or unparseable
	Unified7dStatus            string
	Unified7dReset             string
	Unified7dUtilization       float64 // fraction [0,1]; -1 = missing or unparseable
	Unified7dSonnetStatus      string
	Unified7dSonnetUtilization float64 // fraction [0,1]; -1 = missing
	Unified7dSonnetReset       string
	RepresentativeClaim        string  // "five_hour" or "seven_day"
	FallbackPercentage         float64 // fraction [0,1]; -1 = missing
	OverageDisabledReason      string
	OrganizationID             string // from anthropic-organization-id header

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
		TokenKey:                   tokenKey,
		ParsedAt:                   time.Now(),
		Unified5hUtilization:       -1,
		Unified7dUtilization:       -1,
		Unified7dSonnetUtilization: -1,
		FallbackPercentage:         -1,
	}

	parseInt := func(name string) int {
		raw := h.Get(name)
		if raw == "" {
			return -1
		}
		v, err := strconv.Atoi(raw)
		if err != nil {
			log.Printf("ratelimit: cannot parse header %q value %q: %v", name, raw, err)
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
			log.Printf("ratelimit: cannot parse header %q value %q: %v", name, raw, err)
			return -1
		}
		return v
	}
	parseUtilization := func(name string) float64 {
		v := parseFloat(name)
		if v == -1 {
			// NOTE: -1.0 is our internal sentinel for "no data". If Anthropic ever sends
			// a literal "-1" in the header, it would be silently treated as no data.
			// This is considered an acceptable edge case as utilization can never be -1.
			return v // passthrough sentinel
		}
		if v < 0 || v > 1.0 {
			log.Printf("warn: ratelimit: utilization header %q for token %s out of range (got %.4f), clamping to [0,1]", name, tokenKey, v)
			v = math.Max(0, math.Min(1.0, v))
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
	state.Unified5hUtilization = parseUtilization("anthropic-ratelimit-unified-5h-utilization")
	state.Unified7dStatus = h.Get("anthropic-ratelimit-unified-7d-status")
	state.Unified7dReset = h.Get("anthropic-ratelimit-unified-7d-reset")
	state.Unified7dUtilization = parseUtilization("anthropic-ratelimit-unified-7d-utilization")
	state.Unified7dSonnetStatus = h.Get("anthropic-ratelimit-unified-7d_sonnet-status")
	state.Unified7dSonnetReset = h.Get("anthropic-ratelimit-unified-7d_sonnet-reset")
	state.Unified7dSonnetUtilization = parseUtilization("anthropic-ratelimit-unified-7d_sonnet-utilization")
	state.RepresentativeClaim = h.Get("anthropic-ratelimit-unified-representative-claim")
	state.FallbackPercentage = parseFloat("anthropic-ratelimit-unified-fallback-percentage")
	state.OverageDisabledReason = h.Get("anthropic-ratelimit-unified-overage-disabled-reason")
	state.OrganizationID = h.Get("anthropic-organization-id")

	return state
}

// RateLimitStore is the interface for storing per-token rate limit state.
type RateLimitStore interface {
	Set(key string, state AnthropicOAuthRateLimitState)
	// SetClean writes the entry without marking it dirty. Use this when
	// back-filling from the DB to avoid immediately flushing the same data
	// back to the DB on the next flush cycle.
	SetClean(key string, state AnthropicOAuthRateLimitState)
	Get(key string) (AnthropicOAuthRateLimitState, bool)
	GetAll() map[string]AnthropicOAuthRateLimitState
	SetOpenAIQuotaState(subjectID string, state OpenAIQuotaState)
	GetOpenAIQuotaState(subjectID string, now time.Time) (OpenAIQuotaState, bool)
}

type rateLimitEntry struct {
	state     AnthropicOAuthRateLimitState
	updatedAt time.Time
}

// InMemoryRateLimitStore is a thread-safe in-memory implementation of RateLimitStore.
// A single mutex protects both entries and the dirty set to eliminate race conditions
// between Set/SnapshotDirty/PruneExpired.
type InMemoryRateLimitStore struct {
	mu            sync.Mutex
	entries       map[string]rateLimitEntry
	dirty         map[string]bool
	openAIEntries map[string]OpenAIQuotaState
}

// NewInMemoryRateLimitStore creates an empty InMemoryRateLimitStore.
func NewInMemoryRateLimitStore() *InMemoryRateLimitStore {
	return &InMemoryRateLimitStore{
		entries:       make(map[string]rateLimitEntry),
		dirty:         make(map[string]bool),
		openAIEntries: make(map[string]OpenAIQuotaState),
	}
}

func (s *InMemoryRateLimitStore) Set(key string, state AnthropicOAuthRateLimitState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.putLocked(key, state)
	s.dirty[key] = true
}

// SetClean writes an entry without marking it dirty. Used for DB preload and
// back-fill from DB to avoid re-flushing data that was just read from the DB.
func (s *InMemoryRateLimitStore) SetClean(key string, state AnthropicOAuthRateLimitState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.putLocked(key, state)
}

func (s *InMemoryRateLimitStore) putLocked(key string, state AnthropicOAuthRateLimitState) {
	if existing, ok := s.entries[key]; ok {
		state = mergeRateLimitState(existing.state, state)
	}
	s.entries[key] = rateLimitEntry{state: state, updatedAt: time.Now()}
}

// mergeRateLimitState merges incoming state into existing, preserving existing
// values when the incoming field carries the "no data" sentinel (-1 for floats,
// "" for strings, -1 for ints). This prevents a non-Sonnet response from wiping
// out Sonnet-only utilization data (and vice versa for any model-specific window).
func mergeRateLimitState(existing, incoming AnthropicOAuthRateLimitState) AnthropicOAuthRateLimitState {
	// Always take incoming metadata.
	// TokenKey and ParsedAt come from the latest response.

	// Merge float64 fields where -1 = no data.
	mergeFloat := func(old, new float64) float64 {
		if new == -1 {
			return old
		}
		return new
	}
	// Merge string fields where "" = no data.
	mergeStr := func(old, new string) string {
		if new == "" {
			return old
		}
		return new
	}
	// Merge int fields where -1 = no data.
	mergeInt := func(old, new int) int {
		if new == -1 {
			return old
		}
		return new
	}

	// Per-type (legacy) headers.
	incoming.RequestsLimit = mergeInt(existing.RequestsLimit, incoming.RequestsLimit)
	incoming.RequestsRemaining = mergeInt(existing.RequestsRemaining, incoming.RequestsRemaining)
	incoming.TokensLimit = mergeInt(existing.TokensLimit, incoming.TokensLimit)
	incoming.TokensRemaining = mergeInt(existing.TokensRemaining, incoming.TokensRemaining)
	incoming.RequestsResetAt = mergeStr(existing.RequestsResetAt, incoming.RequestsResetAt)
	incoming.TokensResetAt = mergeStr(existing.TokensResetAt, incoming.TokensResetAt)

	// Unified OAuth headers.
	incoming.UnifiedStatus = mergeStr(existing.UnifiedStatus, incoming.UnifiedStatus)
	incoming.UnifiedReset = mergeStr(existing.UnifiedReset, incoming.UnifiedReset)
	incoming.Unified5hStatus = mergeStr(existing.Unified5hStatus, incoming.Unified5hStatus)
	incoming.Unified5hReset = mergeStr(existing.Unified5hReset, incoming.Unified5hReset)
	incoming.Unified5hUtilization = mergeFloat(existing.Unified5hUtilization, incoming.Unified5hUtilization)
	incoming.Unified7dStatus = mergeStr(existing.Unified7dStatus, incoming.Unified7dStatus)
	incoming.Unified7dReset = mergeStr(existing.Unified7dReset, incoming.Unified7dReset)
	incoming.Unified7dUtilization = mergeFloat(existing.Unified7dUtilization, incoming.Unified7dUtilization)
	incoming.Unified7dSonnetStatus = mergeStr(existing.Unified7dSonnetStatus, incoming.Unified7dSonnetStatus)
	incoming.Unified7dSonnetReset = mergeStr(existing.Unified7dSonnetReset, incoming.Unified7dSonnetReset)
	incoming.Unified7dSonnetUtilization = mergeFloat(existing.Unified7dSonnetUtilization, incoming.Unified7dSonnetUtilization)
	incoming.RepresentativeClaim = mergeStr(existing.RepresentativeClaim, incoming.RepresentativeClaim)
	incoming.FallbackPercentage = mergeFloat(existing.FallbackPercentage, incoming.FallbackPercentage)
	incoming.OverageDisabledReason = mergeStr(existing.OverageDisabledReason, incoming.OverageDisabledReason)
	incoming.OrganizationID = mergeStr(existing.OrganizationID, incoming.OrganizationID)

	return incoming
}

// MarkDirty re-marks the given keys as dirty (used to retry failed flushes).
func (s *InMemoryRateLimitStore) MarkDirty(keys []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range keys {
		s.dirty[k] = true
	}
}

// SnapshotDirty atomically reads all dirty entries and clears the dirty set.
// Returns a map of key → state for all dirty keys that still exist in the store.
func (s *InMemoryRateLimitStore) SnapshotDirty() map[string]AnthropicOAuthRateLimitState {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]AnthropicOAuthRateLimitState, len(s.dirty))
	for k := range s.dirty {
		if e, ok := s.entries[k]; ok {
			out[k] = e.state
		}
	}
	s.dirty = make(map[string]bool)
	return out
}

// Get returns a normalized copy of the state for key. Expired windows are zeroed
// before returning. ParseResetTime may log on malformed timestamps while the lock
// is held — acceptable here because Get processes a single entry (unlike PruneExpired
// which snapshots first to avoid holding the lock during N log calls).
func (s *InMemoryRateLimitStore) Get(key string) (AnthropicOAuthRateLimitState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[key]
	if !ok {
		return e.state, false
	}
	return NormalizeExpiredWindows(e.state, time.Now()), true
}

func (s *InMemoryRateLimitStore) GetAll() map[string]AnthropicOAuthRateLimitState {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	out := make(map[string]AnthropicOAuthRateLimitState, len(s.entries))
	for k, e := range s.entries {
		out[k] = NormalizeExpiredWindows(e.state, now)
	}
	return out
}

func (s *InMemoryRateLimitStore) SetOpenAIQuotaState(subjectID string, state OpenAIQuotaState) {
	if subjectID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.openAIEntries == nil {
		s.openAIEntries = make(map[string]OpenAIQuotaState)
	}
	state.SubjectID = subjectID
	if existing, ok := s.openAIEntries[subjectID]; ok {
		state = MergeOpenAIQuotaState(existing, state)
	}
	s.openAIEntries[subjectID] = state
}

func (s *InMemoryRateLimitStore) GetOpenAIQuotaState(subjectID string, now time.Time) (OpenAIQuotaState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.openAIEntries[subjectID]
	if !ok {
		return OpenAIQuotaState{}, false
	}
	return state.Normalize(now), true
}

// PruneExpired removes entries whose both 5h and 7d reset windows have passed,
// meaning the utilization counters have been reset by Anthropic and the stored
// values are genuinely stale. Dirty entries are always skipped.
// Respects Anthropic's actual reset schedule so idle-but-valid data is not discarded.
//
// Zero reset times (empty string → no reset header received yet) are treated as
// NOT expired — the token is new or in-flight and must be preserved.
func (s *InMemoryRateLimitStore) PruneExpired() {
	now := time.Now()

	// Snapshot under lock to avoid holding the lock during ParseResetTime's log.Printf.
	s.mu.Lock()
	type candidate struct {
		key   string
		state AnthropicOAuthRateLimitState
	}
	var candidates []candidate
	for k, e := range s.entries {
		if !s.dirty[k] {
			candidates = append(candidates, candidate{key: k, state: e.state})
		}
	}
	s.mu.Unlock()

	// Evaluate expiry outside the lock (ParseResetTime may log on bad input).
	var toDelete []string
	for _, c := range candidates {
		if BothWindowsExpired(c.state, now) {
			toDelete = append(toDelete, c.key)
		}
	}

	if len(toDelete) == 0 {
		return
	}

	// Re-acquire lock to delete. Re-check dirty to avoid deleting a key that
	// was written between the snapshot and now.
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range toDelete {
		if !s.dirty[k] {
			delete(s.entries, k)
		}
	}
}
