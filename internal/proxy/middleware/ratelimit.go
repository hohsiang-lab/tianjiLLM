package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/redis/go-redis/v9"
)

// slidingWindowScript is a Redis Lua script for sliding window rate limiting.
const slidingWindowScript = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

-- Remove expired entries
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)

-- Count current entries
local count = redis.call('ZCARD', key)

if count >= limit then
    return 0
end

-- Add new entry
redis.call('ZADD', key, now, now .. '-' .. math.random(1000000))
redis.call('EXPIRE', key, window)

return 1
`

// RateLimiter checks RPM/TPM limits using Redis sliding window.
type RateLimiter struct {
	rdb    redis.UniversalClient
	script *redis.Script
}

// NewRateLimiter creates a rate limiter backed by Redis.
func NewRateLimiter(rdb redis.UniversalClient) *RateLimiter {
	return &RateLimiter{
		rdb:    rdb,
		script: redis.NewScript(slidingWindowScript),
	}
}

// CheckRPM checks if the request rate is within the RPM limit.
func (rl *RateLimiter) CheckRPM(ctx context.Context, keyHash string, limit int64) (bool, error) {
	key := fmt.Sprintf("tianji:rpm:%s", keyHash)
	return rl.check(ctx, key, limit, 60)
}

// tpmIncrScript atomically adds tokens to a TPM counter and returns current total.
const tpmIncrScript = `
local key = KEYS[1]
local tokens = tonumber(ARGV[1])
local window = tonumber(ARGV[2])

local current = redis.call('INCRBY', key, tokens)
if current == tokens then
    redis.call('EXPIRE', key, window)
end
return current
`

// CheckTPM checks if adding tokens would exceed the TPM limit for the given key+model.
// Returns nil if allowed, model.ErrRateLimit if rejected.
func (rl *RateLimiter) CheckTPM(ctx context.Context, keyHash, modelName string, tokens int, tpmLimit int64) error {
	if rl.rdb == nil || tpmLimit <= 0 || tokens <= 0 {
		return nil
	}
	key := fmt.Sprintf("tianji:tpm:%s:%s", keyHash, modelName)
	script := redis.NewScript(tpmIncrScript)
	current, err := script.Run(ctx, rl.rdb, []string{key}, tokens, 60).Int64()
	if err != nil {
		return nil // allow on error
	}
	if current > tpmLimit {
		return model.ErrRateLimit
	}
	return nil
}

// TPMUtilization returns the current TPM utilization ratio (0.0-1.0) for a key+model.
func (rl *RateLimiter) TPMUtilization(ctx context.Context, keyHash, modelName string, tpmLimit int64) float64 {
	if rl.rdb == nil || tpmLimit <= 0 {
		return 0
	}
	key := fmt.Sprintf("tianji:tpm:%s:%s", keyHash, modelName)
	current, err := rl.rdb.Get(ctx, key).Int64()
	if err != nil {
		return 0
	}
	ratio := float64(current) / float64(tpmLimit)
	if ratio > 1 {
		return 1
	}
	return ratio
}

func (rl *RateLimiter) check(ctx context.Context, key string, limit int64, windowSecs int64) (bool, error) {
	now := time.Now().Unix()
	result, err := rl.script.Run(ctx, rl.rdb, []string{key}, limit, windowSecs, now).Int64()
	if err != nil {
		return true, err // allow on error
	}
	return result == 1, nil
}

// NewRateLimitMiddleware creates middleware that enforces RPM limits.
func NewRateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	if limiter == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenHash, _ := r.Context().Value(tokenHashKey).(string)
			rpmLimit, _ := r.Context().Value(rpmLimitKey).(int64)

			if tokenHash != "" && rpmLimit > 0 {
				allowed, err := limiter.CheckRPM(r.Context(), tokenHash, rpmLimit)
				if err == nil && !allowed {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusTooManyRequests)
					writeJSONResponse(w, model.ErrorResponse{
						Error: model.ErrorDetail{
							Message: "rate limit exceeded",
							Type:    "rate_limit_exceeded",
							Code:    "rate_limit_exceeded",
						},
					})
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
