package middleware

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRateLimiter(t *testing.T) (*RateLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewRateLimiter(rdb), mr
}

func TestCheckRPM_WithinLimit(t *testing.T) {
	rl, _ := setupRateLimiter(t)
	ctx := context.Background()

	allowed, err := rl.CheckRPM(ctx, "key1", 10)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestCheckRPM_ExceedsLimit(t *testing.T) {
	rl, _ := setupRateLimiter(t)
	ctx := context.Background()

	// Fill up the limit
	for i := 0; i < 5; i++ {
		allowed, err := rl.CheckRPM(ctx, "key2", 5)
		require.NoError(t, err)
		assert.True(t, allowed, "request %d should be allowed", i)
	}

	// Next should be rejected
	allowed, err := rl.CheckRPM(ctx, "key2", 5)
	require.NoError(t, err)
	assert.False(t, allowed, "6th request should be rejected")
}

func TestCheckTPM_WithinLimit(t *testing.T) {
	rl, _ := setupRateLimiter(t)
	ctx := context.Background()

	err := rl.CheckTPM(ctx, "key1", "gpt-4", 100, 1000)
	assert.NoError(t, err)
}

func TestCheckTPM_ExceedsLimit(t *testing.T) {
	rl, _ := setupRateLimiter(t)
	ctx := context.Background()

	// Add tokens up to the limit
	err := rl.CheckTPM(ctx, "key1", "gpt-4", 800, 1000)
	assert.NoError(t, err)

	// This should push over the limit
	err = rl.CheckTPM(ctx, "key1", "gpt-4", 300, 1000)
	assert.ErrorIs(t, err, model.ErrRateLimit)
}

func TestCheckTPM_ZeroLimit(t *testing.T) {
	rl, _ := setupRateLimiter(t)
	ctx := context.Background()

	// Zero limit should always allow
	err := rl.CheckTPM(ctx, "key1", "gpt-4", 100, 0)
	assert.NoError(t, err)
}

func TestTPMUtilization(t *testing.T) {
	rl, _ := setupRateLimiter(t)
	ctx := context.Background()

	// Add 500 tokens out of 1000 limit
	err := rl.CheckTPM(ctx, "key1", "gpt-4", 500, 1000)
	require.NoError(t, err)

	util := rl.TPMUtilization(ctx, "key1", "gpt-4", 1000)
	assert.InDelta(t, 0.5, util, 0.01)
}

func TestTPMUtilization_NoData(t *testing.T) {
	rl, _ := setupRateLimiter(t)
	ctx := context.Background()

	util := rl.TPMUtilization(ctx, "nonexistent", "gpt-4", 1000)
	assert.Equal(t, 0.0, util)
}
