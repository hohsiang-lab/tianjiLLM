package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/redis/go-redis/v9"
)

// DynamicRateLimiter implements saturation-aware rate limiting.
// When overall system utilization exceeds a threshold, lower-priority
// requests are throttled more aggressively.
//
// v3 enhancements:
//   - Per-model saturation tracking (model-level RPM utilization)
//   - TPM (tokens per minute) dimension in addition to RPM
//   - X-RateLimit-* response header injection
type DynamicRateLimiter struct {
	rdb                 redis.UniversalClient
	saturationThreshold float64 // 0.0-1.0, default 0.8
}

// NewDynamicRateLimiter creates a dynamic rate limiter.
func NewDynamicRateLimiter(rdb redis.UniversalClient) *DynamicRateLimiter {
	return &DynamicRateLimiter{
		rdb:                 rdb,
		saturationThreshold: 0.8,
	}
}

// SetSaturationThreshold sets the saturation level above which throttling kicks in.
func (d *DynamicRateLimiter) SetSaturationThreshold(t float64) {
	if t > 0 && t <= 1 {
		d.saturationThreshold = t
	}
}

const ttl60s = 60 * time.Second

// getSaturation reads utilization ratio from Redis.
// If modelGroup is non-empty, reads model-specific saturation; otherwise global.
func (d *DynamicRateLimiter) getSaturation(ctx context.Context, modelGroup string) float64 {
	if d.rdb == nil {
		return 0
	}
	key := "tianji:dynamic_rate:saturation"
	if modelGroup != "" {
		key = fmt.Sprintf("tianji:dynamic_rate:saturation:%s", modelGroup)
	}
	val, err := d.rdb.Get(ctx, key).Float64()
	if err != nil {
		return 0
	}
	return val
}

// RecordUtilization updates the global saturation counter.
func (d *DynamicRateLimiter) RecordUtilization(ctx context.Context, utilization float64) {
	if d.rdb == nil {
		return
	}
	d.rdb.Set(ctx, "tianji:dynamic_rate:saturation", utilization, 0)
}

// RecordModelUtilization updates the per-model saturation counter.
func (d *DynamicRateLimiter) RecordModelUtilization(ctx context.Context, modelGroup string, utilization float64) {
	if d.rdb == nil || modelGroup == "" {
		return
	}
	key := fmt.Sprintf("tianji:dynamic_rate:saturation:%s", modelGroup)
	d.rdb.Set(ctx, key, utilization, 0)
}

// computeFactor returns the throttle factor (0.1 to 1.0) based on saturation.
func (d *DynamicRateLimiter) computeFactor(saturation float64, priority int) float64 {
	if saturation < d.saturationThreshold || priority <= 0 {
		return 1.0
	}
	excess := saturation - d.saturationThreshold
	factor := 1.0 - (excess * float64(priority) * 2.0)
	if factor < 0.1 {
		factor = 0.1
	}
	return factor
}

// CheckResult holds rate limit check output including remaining counts for headers.
type CheckResult struct {
	Allowed      bool
	RPMRemaining int64
	RPMLimit     int64
	TPMRemaining int64
	TPMLimit     int64
	ResetSeconds int // seconds until counters reset
	EffectiveRPM int64
	EffectiveTPM int64
}

// Check determines if a request should be allowed based on priority, saturation, and limits.
func (d *DynamicRateLimiter) Check(ctx context.Context, keyHash string, priority int, rpmLimit int64) (bool, error) {
	r, err := d.CheckFull(ctx, keyHash, "", priority, rpmLimit, 0)
	return r.Allowed, err
}

// CheckFull performs the full rate limit check including model-level saturation and TPM.
func (d *DynamicRateLimiter) CheckFull(ctx context.Context, keyHash, modelGroup string, priority int, rpmLimit, tpmLimit int64) (CheckResult, error) {
	result := CheckResult{
		Allowed:      true,
		RPMLimit:     rpmLimit,
		TPMLimit:     tpmLimit,
		RPMRemaining: rpmLimit,
		TPMRemaining: tpmLimit,
		ResetSeconds: 60,
	}

	if d.rdb == nil || (rpmLimit <= 0 && tpmLimit <= 0) {
		return result, nil
	}

	// Use model-specific saturation if available, fall back to global
	saturation := d.getSaturation(ctx, modelGroup)
	if saturation == 0 && modelGroup != "" {
		saturation = d.getSaturation(ctx, "")
	}

	factor := d.computeFactor(saturation, priority)
	result.EffectiveRPM = int64(float64(rpmLimit) * factor)
	result.EffectiveTPM = int64(float64(tpmLimit) * factor)
	if result.EffectiveRPM <= 0 && rpmLimit > 0 {
		result.EffectiveRPM = 1
	}
	if result.EffectiveTPM <= 0 && tpmLimit > 0 {
		result.EffectiveTPM = 1
	}

	// RPM check
	if rpmLimit > 0 {
		key := fmt.Sprintf("tianji:dynamic_rpm:%s", keyHash)
		if modelGroup != "" {
			key = fmt.Sprintf("tianji:dynamic_rpm:%s:%s", keyHash, modelGroup)
		}
		count, err := d.rdb.Incr(ctx, key).Result()
		if err != nil {
			return result, err
		}
		if count == 1 {
			d.rdb.Expire(ctx, key, ttl60s)
		}
		result.RPMRemaining = result.EffectiveRPM - count
		if result.RPMRemaining < 0 {
			result.RPMRemaining = 0
		}
		if count > result.EffectiveRPM {
			result.Allowed = false
		}
	}

	// TPM check
	if tpmLimit > 0 {
		key := fmt.Sprintf("tianji:dynamic_tpm:%s", keyHash)
		if modelGroup != "" {
			key = fmt.Sprintf("tianji:dynamic_tpm:%s:%s", keyHash, modelGroup)
		}
		count, err := d.rdb.Get(ctx, key).Int64()
		if err != nil && err != redis.Nil {
			return result, err
		}
		result.TPMRemaining = result.EffectiveTPM - count
		if result.TPMRemaining < 0 {
			result.TPMRemaining = 0
		}
		if count >= result.EffectiveTPM {
			result.Allowed = false
		}
	}

	return result, nil
}

// RecordTokens records token usage for TPM tracking.
func (d *DynamicRateLimiter) RecordTokens(ctx context.Context, keyHash, modelGroup string, tokens int64) {
	if d.rdb == nil || tokens <= 0 {
		return
	}
	key := fmt.Sprintf("tianji:dynamic_tpm:%s", keyHash)
	if modelGroup != "" {
		key = fmt.Sprintf("tianji:dynamic_tpm:%s:%s", keyHash, modelGroup)
	}
	d.rdb.IncrBy(ctx, key, tokens)
	d.rdb.Expire(ctx, key, ttl60s)
}

// setRateLimitHeaders writes X-RateLimit-* headers on the response.
func setRateLimitHeaders(w http.ResponseWriter, r CheckResult) {
	if r.RPMLimit > 0 {
		w.Header().Set("X-RateLimit-Limit-Requests", strconv.FormatInt(r.EffectiveRPM, 10))
		w.Header().Set("X-RateLimit-Remaining-Requests", strconv.FormatInt(r.RPMRemaining, 10))
	}
	if r.TPMLimit > 0 {
		w.Header().Set("X-RateLimit-Limit-Tokens", strconv.FormatInt(r.EffectiveTPM, 10))
		w.Header().Set("X-RateLimit-Remaining-Tokens", strconv.FormatInt(r.TPMRemaining, 10))
	}
	w.Header().Set("X-RateLimit-Reset-Requests", strconv.Itoa(r.ResetSeconds)+"s")
}

// NewDynamicRateLimitMiddleware creates middleware for dynamic rate limiting.
func NewDynamicRateLimitMiddleware(limiter *DynamicRateLimiter) func(http.Handler) http.Handler {
	if limiter == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenHash, _ := r.Context().Value(tokenHashKey).(string)
			rpmLimit, _ := r.Context().Value(rpmLimitKey).(int64)
			tpmLimit, _ := r.Context().Value(tpmLimitKey).(int64)
			priority, _ := r.Context().Value(priorityKey).(int)
			modelGroup, _ := r.Context().Value(modelGroupKey).(string)

			if tokenHash == "" || (rpmLimit <= 0 && tpmLimit <= 0) {
				next.ServeHTTP(w, r)
				return
			}

			result, err := limiter.CheckFull(r.Context(), tokenHash, modelGroup, priority, rpmLimit, tpmLimit)
			setRateLimitHeaders(w, result)

			if err == nil && !result.Allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				writeJSONResponse(w, model.ErrorResponse{
					Error: model.ErrorDetail{
						Message: "rate limit exceeded (dynamic throttling)",
						Type:    "rate_limit_exceeded",
						Code:    "rate_limit_exceeded",
					},
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// priorityKey is the context key for request priority level.
var priorityKey contextKey = "priority"
