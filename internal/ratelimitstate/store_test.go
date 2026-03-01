// Package ratelimitstate_test covers the process-level global store for
// Anthropic rate limit state (FR-005, SC-002, SC-004).
package ratelimitstate_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/ratelimitstate"
)

// ---------------------------------------------------------------------------
// FR-007 / SC-004: initial state is empty (no data yet)
// ---------------------------------------------------------------------------

// TestGet_InitialState verifies that before any Set call the store returns
// a zero-value / nil snapshot and a clear "no data" signal.
func TestGet_InitialState(t *testing.T) {
	s := ratelimitstate.New()
	snap, ok := s.Get()
	assert.False(t, ok, "Get should return ok=false when no data has been set")
	assert.Nil(t, snap, "snapshot should be nil when no data has been set")
}

// ---------------------------------------------------------------------------
// FR-005 / SC-002: Set persists data; Get returns it accurately
// ---------------------------------------------------------------------------

// TestSetAndGet verifies that a stored snapshot is returned exactly as-is.
func TestSetAndGet(t *testing.T) {
	s := ratelimitstate.New()

	snap := &ratelimitstate.Snapshot{
		CapturedAt: time.Now().UTC(),
		InputTokens: &ratelimitstate.DimensionState{
			Limit:     100_000,
			Remaining: 80_000,
			ResetsAt:  time.Now().Add(60 * time.Second).UTC(),
		},
		OutputTokens: &ratelimitstate.DimensionState{
			Limit:     50_000,
			Remaining: 49_000,
			ResetsAt:  time.Now().Add(55 * time.Second).UTC(),
		},
		Requests: &ratelimitstate.DimensionState{
			Limit:     1_000,
			Remaining: 995,
			ResetsAt:  time.Now().Add(58 * time.Second).UTC(),
		},
	}

	s.Set(snap)

	got, ok := s.Get()
	require.True(t, ok, "Get should return ok=true after Set")
	require.NotNil(t, got)

	assert.Equal(t, snap.CapturedAt.Unix(), got.CapturedAt.Unix(), "CapturedAt must match")

	require.NotNil(t, got.InputTokens)
	assert.Equal(t, snap.InputTokens.Limit, got.InputTokens.Limit)
	assert.Equal(t, snap.InputTokens.Remaining, got.InputTokens.Remaining)
	assert.Equal(t, snap.InputTokens.ResetsAt.Unix(), got.InputTokens.ResetsAt.Unix())

	require.NotNil(t, got.OutputTokens)
	assert.Equal(t, snap.OutputTokens.Limit, got.OutputTokens.Limit)
	assert.Equal(t, snap.OutputTokens.Remaining, got.OutputTokens.Remaining)

	require.NotNil(t, got.Requests)
	assert.Equal(t, snap.Requests.Limit, got.Requests.Limit)
	assert.Equal(t, snap.Requests.Remaining, got.Requests.Remaining)
}

// ---------------------------------------------------------------------------
// FR-008: partial header — nil dimension fields are preserved
// ---------------------------------------------------------------------------

// TestSet_PartialSnapshot verifies that a snapshot with some nil dimensions
// is stored and retrieved without modification.
func TestSet_PartialSnapshot(t *testing.T) {
	s := ratelimitstate.New()

	snap := &ratelimitstate.Snapshot{
		CapturedAt: time.Now().UTC(),
		InputTokens: &ratelimitstate.DimensionState{
			Limit:     100_000,
			Remaining: 80_000,
			ResetsAt:  time.Now().Add(60 * time.Second).UTC(),
		},
		OutputTokens: nil,
		Requests:     nil,
	}

	s.Set(snap)
	got, ok := s.Get()
	require.True(t, ok)
	assert.Nil(t, got.OutputTokens, "nil OutputTokens must be preserved (FR-008)")
	assert.Nil(t, got.Requests, "nil Requests must be preserved (FR-008)")
	assert.NotNil(t, got.InputTokens)
}

// ---------------------------------------------------------------------------
// SC-002: most-recent Set wins (no stale data)
// ---------------------------------------------------------------------------

// TestSet_Overwrite verifies that the latest snapshot overwrites the previous one.
func TestSet_Overwrite(t *testing.T) {
	s := ratelimitstate.New()

	first := &ratelimitstate.Snapshot{
		CapturedAt: time.Now().UTC(),
		Requests:   &ratelimitstate.DimensionState{Limit: 1000, Remaining: 500},
	}
	second := &ratelimitstate.Snapshot{
		CapturedAt: time.Now().Add(time.Second).UTC(),
		Requests:   &ratelimitstate.DimensionState{Limit: 1000, Remaining: 123},
	}

	s.Set(first)
	s.Set(second)

	got, ok := s.Get()
	require.True(t, ok)
	assert.Equal(t, int64(123), got.Requests.Remaining, "Get must return the latest value")
}

// ---------------------------------------------------------------------------
// Concurrency: store must be goroutine-safe (sync.RWMutex)
// ---------------------------------------------------------------------------

// TestConcurrentSetGet verifies that concurrent Set+Get calls do not race.
// Run with: go test -race ./internal/ratelimitstate/...
func TestConcurrentSetGet(t *testing.T) {
	s := ratelimitstate.New()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			snap := &ratelimitstate.Snapshot{
				CapturedAt: time.Now().UTC(),
				Requests:   &ratelimitstate.DimensionState{Limit: int64(i), Remaining: int64(i)},
			}
			s.Set(snap)
		}(i)

		go func() {
			defer wg.Done()
			s.Get()
		}()
	}

	wg.Wait()
}
