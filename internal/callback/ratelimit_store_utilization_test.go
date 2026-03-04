package callback

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUtilization_Present(t *testing.T) {
	t.Parallel()
	store := NewInMemoryRateLimitStore()
	store.Set("tok-a", AnthropicOAuthRateLimitState{
		TokenKey:             "tok-a",
		Unified5hUtilization: 0.42,
	})

	util, ok := store.GetUtilization("tok-a")
	require.True(t, ok)
	assert.InDelta(t, 42.0, util, 0.01)
}

func TestGetUtilization_Missing(t *testing.T) {
	t.Parallel()
	store := NewInMemoryRateLimitStore()

	_, ok := store.GetUtilization("nonexistent")
	assert.False(t, ok)
}

func TestGetUtilization_SentinelNeg1(t *testing.T) {
	t.Parallel()
	store := NewInMemoryRateLimitStore()
	store.Set("tok-b", AnthropicOAuthRateLimitState{
		TokenKey:             "tok-b",
		Unified5hUtilization: -1, // sentinel: header missing
	})

	_, ok := store.GetUtilization("tok-b")
	assert.False(t, ok)
}

func TestGetLowestUtilization_Basic(t *testing.T) {
	t.Parallel()
	store := NewInMemoryRateLimitStore()
	store.Set("tok-1", AnthropicOAuthRateLimitState{
		TokenKey:             "tok-1",
		Unified5hUtilization: 0.80,
		Unified5hStatus:      "allowed",
	})
	store.Set("tok-2", AnthropicOAuthRateLimitState{
		TokenKey:             "tok-2",
		Unified5hUtilization: 0.30,
		Unified5hStatus:      "allowed",
	})
	store.Set("tok-3", AnthropicOAuthRateLimitState{
		TokenKey:             "tok-3",
		Unified5hUtilization: 0.55,
		Unified5hStatus:      "allowed",
	})

	key, util := store.GetLowestUtilization([]string{"tok-1", "tok-2", "tok-3"})
	assert.Equal(t, "tok-2", key)
	assert.InDelta(t, 30.0, util, 0.01)
}

func TestGetLowestUtilization_SkipsRateLimited(t *testing.T) {
	t.Parallel()
	store := NewInMemoryRateLimitStore()
	store.Set("tok-1", AnthropicOAuthRateLimitState{
		TokenKey:             "tok-1",
		Unified5hUtilization: 0.10,
		Unified5hStatus:      "rate_limited",
	})
	store.Set("tok-2", AnthropicOAuthRateLimitState{
		TokenKey:             "tok-2",
		Unified5hUtilization: 0.50,
		Unified5hStatus:      "allowed",
	})

	key, util := store.GetLowestUtilization([]string{"tok-1", "tok-2"})
	assert.Equal(t, "tok-2", key)
	assert.InDelta(t, 50.0, util, 0.01)
}

func TestGetLowestUtilization_AllRateLimited(t *testing.T) {
	t.Parallel()
	store := NewInMemoryRateLimitStore()
	store.Set("tok-1", AnthropicOAuthRateLimitState{
		TokenKey:             "tok-1",
		Unified5hUtilization: 0.10,
		Unified5hStatus:      "rate_limited",
	})

	key, util := store.GetLowestUtilization([]string{"tok-1"})
	assert.Equal(t, "", key)
	assert.Equal(t, float64(-1), util)
}

func TestGetLowestUtilization_NoData(t *testing.T) {
	t.Parallel()
	store := NewInMemoryRateLimitStore()

	key, util := store.GetLowestUtilization([]string{"tok-1", "tok-2"})
	assert.Equal(t, "", key)
	assert.Equal(t, float64(-1), util)
}

func TestGetLowestUtilization_SkipsMissingSentinel(t *testing.T) {
	t.Parallel()
	store := NewInMemoryRateLimitStore()
	store.Set("tok-1", AnthropicOAuthRateLimitState{
		TokenKey:             "tok-1",
		Unified5hUtilization: -1, // no data
		Unified5hStatus:      "allowed",
	})
	store.Set("tok-2", AnthropicOAuthRateLimitState{
		TokenKey:             "tok-2",
		Unified5hUtilization: 0.60,
		Unified5hStatus:      "allowed",
	})

	key, util := store.GetLowestUtilization([]string{"tok-1", "tok-2"})
	assert.Equal(t, "tok-2", key)
	assert.InDelta(t, 60.0, util, 0.01)
}
