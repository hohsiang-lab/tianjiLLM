package middleware

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// BatchRateLimiter checks rate limits for batch submissions.
// On batch submission, it estimates total requests/tokens and reserves capacity
// against the key's RPM/TPM limits.
type BatchRateLimiter struct {
	rdb    redis.UniversalClient
	script *redis.Script
}

// batchReserveScript atomically reserves capacity for a batch.
const batchReserveScript = `
local rpm_key = KEYS[1]
local tpm_key = KEYS[2]
local request_count = tonumber(ARGV[1])
local estimated_tokens = tonumber(ARGV[2])
local rpm_limit = tonumber(ARGV[3])
local tpm_limit = tonumber(ARGV[4])
local ttl = tonumber(ARGV[5])

-- Check RPM
if rpm_limit > 0 then
    local current_rpm = tonumber(redis.call('GET', rpm_key) or '0')
    if current_rpm + request_count > rpm_limit then
        return 0
    end
end

-- Check TPM
if tpm_limit > 0 then
    local current_tpm = tonumber(redis.call('GET', tpm_key) or '0')
    if current_tpm + estimated_tokens > tpm_limit then
        return 0
    end
end

-- Reserve capacity
if rpm_limit > 0 then
    local rpm = redis.call('INCRBY', rpm_key, request_count)
    if rpm == request_count then
        redis.call('EXPIRE', rpm_key, ttl)
    end
end

if tpm_limit > 0 then
    local tpm = redis.call('INCRBY', tpm_key, estimated_tokens)
    if tpm == estimated_tokens then
        redis.call('EXPIRE', tpm_key, ttl)
    end
end

return 1
`

// NewBatchRateLimiter creates a batch rate limiter.
func NewBatchRateLimiter(rdb redis.UniversalClient) *BatchRateLimiter {
	return &BatchRateLimiter{
		rdb:    rdb,
		script: redis.NewScript(batchReserveScript),
	}
}

// ReserveCapacity reserves capacity for a batch of requests.
// Returns true if the batch is within limits, false if it would exceed them.
func (b *BatchRateLimiter) ReserveCapacity(ctx context.Context, keyHash string, requestCount, estimatedTokens int, rpmLimit, tpmLimit int64) (bool, error) {
	if b.rdb == nil {
		return true, nil
	}

	rpmKey := fmt.Sprintf("tianji:batch_rpm:%s", keyHash)
	tpmKey := fmt.Sprintf("tianji:batch_tpm:%s", keyHash)

	result, err := b.script.Run(ctx, b.rdb, []string{rpmKey, tpmKey},
		requestCount, estimatedTokens, rpmLimit, tpmLimit, 60,
	).Int64()
	if err != nil {
		return true, err // allow on error
	}
	return result == 1, nil
}

// AdjustActuals adjusts reserved capacity to actual usage after batch completion.
// delta = actual - estimated (positive means we used more, negative means less).
func (b *BatchRateLimiter) AdjustActuals(ctx context.Context, keyHash string, tokenDelta int) {
	if b.rdb == nil || tokenDelta == 0 {
		return
	}

	tpmKey := fmt.Sprintf("tianji:batch_tpm:%s", keyHash)
	if tokenDelta > 0 {
		b.rdb.IncrBy(ctx, tpmKey, int64(tokenDelta))
	} else {
		b.rdb.DecrBy(ctx, tpmKey, int64(-tokenDelta))
	}
}
