package contract

import (
	"context"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/stretchr/testify/assert"
)

func TestParallelRequestLimiter_NilRedis(t *testing.T) {
	limiter := middleware.NewParallelRequestLimiter(nil)

	allowed, err := limiter.Check(context.Background(), "key-hash-1", 5)
	assert.NoError(t, err)
	assert.True(t, allowed, "should allow when Redis is nil")
}

func TestParallelRequestLimiter_ZeroLimit(t *testing.T) {
	limiter := middleware.NewParallelRequestLimiter(nil)

	allowed, err := limiter.Check(context.Background(), "key-hash-1", 0)
	assert.NoError(t, err)
	assert.True(t, allowed, "should allow when limit is zero")
}

func TestParallelRequestLimiter_NegativeLimit(t *testing.T) {
	limiter := middleware.NewParallelRequestLimiter(nil)

	allowed, err := limiter.Check(context.Background(), "key-hash-1", -1)
	assert.NoError(t, err)
	assert.True(t, allowed, "should allow when limit is negative")
}

func TestParallelRequestLimiter_ReleaseNilRedis(t *testing.T) {
	limiter := middleware.NewParallelRequestLimiter(nil)
	// Should not panic
	limiter.Release(context.Background(), "key-hash-1")
}

func TestNewParallelRequestMiddleware_NilLimiter(t *testing.T) {
	mw := middleware.NewParallelRequestMiddleware(nil)
	assert.NotNil(t, mw, "should return passthrough middleware when limiter is nil")
}
