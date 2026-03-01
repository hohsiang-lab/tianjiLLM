// Package ratelimitstate provides per-token in-memory storage for Anthropic rate limit state.
package ratelimitstate

import (
	"sync"
	"time"
)

// DimensionState holds rate limit data for one quota dimension (tokens or requests).
type DimensionState struct {
	Limit     int64
	Remaining int64
	ResetsAt  time.Time
}

// Snapshot is one capture of Anthropic rate limit headers for a single token/key.
type Snapshot struct {
	CapturedAt   time.Time
	InputTokens  *DimensionState // nil when headers were absent
	OutputTokens *DimensionState
	Requests     *DimensionState
}

// Store holds rate limit state for one token. Thread-safe.
type Store struct {
	mu   sync.RWMutex
	snap *Snapshot
}

// New creates an empty Store.
func New() *Store { return &Store{} }

// Set overwrites the stored snapshot.
func (s *Store) Set(snap *Snapshot) {
	s.mu.Lock()
	s.snap = snap
	s.mu.Unlock()
}

// Get returns the latest snapshot and true, or nil+false when no data has been stored.
func (s *Store) Get() (*Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snap == nil {
		return nil, false
	}
	return s.snap, true
}

// ---------------------------------------------------------------------------
// Global multi-token registry
// ---------------------------------------------------------------------------

var (
	globalMu     sync.RWMutex
	globalStores = make(map[string]*Store)
)

// GetOrCreate returns the Store for keyHash, creating one if it doesn't exist.
func GetOrCreate(keyHash string) *Store {
	globalMu.Lock()
	defer globalMu.Unlock()
	if s, ok := globalStores[keyHash]; ok {
		return s
	}
	s := New()
	globalStores[keyHash] = s
	return s
}

// ListAll returns a copy of the global registry (keyHash → Store).
func ListAll() map[string]*Store {
	globalMu.RLock()
	defer globalMu.RUnlock()
	out := make(map[string]*Store, len(globalStores))
	for k, v := range globalStores {
		out[k] = v
	}
	return out
}
