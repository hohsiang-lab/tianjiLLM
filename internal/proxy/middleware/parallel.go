package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/redis/go-redis/v9"
)

// parallelIncrScript atomically increments a counter and returns the new value.
// Sets TTL only on first increment to avoid resetting the window.
const parallelIncrScript = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])

local current = redis.call('INCR', key)
if current == 1 then
    redis.call('EXPIRE', key, ttl)
end

if current > limit then
    redis.call('DECR', key)
    return 0
end

return 1
`

// ParallelRequestLimiter enforces max_parallel_requests using Redis INCR/DECR.
type ParallelRequestLimiter struct {
	rdb    redis.UniversalClient
	script *redis.Script
	ttl    int // window TTL in seconds
}

// NewParallelRequestLimiter creates a parallel request limiter.
func NewParallelRequestLimiter(rdb redis.UniversalClient) *ParallelRequestLimiter {
	return &ParallelRequestLimiter{
		rdb:    rdb,
		script: redis.NewScript(parallelIncrScript),
		ttl:    60,
	}
}

// Check attempts to acquire a slot. Returns true if allowed.
func (p *ParallelRequestLimiter) Check(ctx context.Context, keyHash string, limit int) (bool, error) {
	if p.rdb == nil || limit <= 0 {
		return true, nil
	}
	key := fmt.Sprintf("tianji:parallel:%s", keyHash)
	result, err := p.script.Run(ctx, p.rdb, []string{key}, limit, p.ttl).Int64()
	if err != nil {
		return true, err // allow on Redis error
	}
	return result == 1, nil
}

// Release decrements the parallel counter for the key.
func (p *ParallelRequestLimiter) Release(ctx context.Context, keyHash string) {
	if p.rdb == nil {
		return
	}
	key := fmt.Sprintf("tianji:parallel:%s", keyHash)
	p.rdb.Decr(ctx, key)
}

// parallelResponseWriter wraps http.ResponseWriter to trigger Release on write completion.
type parallelResponseWriter struct {
	http.ResponseWriter
	released bool
	release  func()
}

func (w *parallelResponseWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
}

func (w *parallelResponseWriter) Write(b []byte) (int, error) {
	return w.ResponseWriter.Write(b)
}

func (w *parallelResponseWriter) done() {
	if !w.released {
		w.released = true
		w.release()
	}
}

// maxParallelLimitKey is the context key for max_parallel_requests.
var maxParallelLimitKey contextKey = "max_parallel_requests"

// NewParallelRequestMiddleware creates middleware enforcing max_parallel_requests.
func NewParallelRequestMiddleware(limiter *ParallelRequestLimiter) func(http.Handler) http.Handler {
	if limiter == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenHash, _ := r.Context().Value(tokenHashKey).(string)
			limit, _ := r.Context().Value(maxParallelLimitKey).(int)

			if tokenHash == "" || limit <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			allowed, err := limiter.Check(r.Context(), tokenHash, limit)
			if err == nil && !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				writeJSONResponse(w, model.ErrorResponse{
					Error: model.ErrorDetail{
						Message: "max parallel requests exceeded",
						Type:    "rate_limit_exceeded",
						Code:    "rate_limit_exceeded",
					},
				})
				return
			}

			// Use a background context for release so it works even after request context cancels.
			releaseCtx := context.Background()
			pw := &parallelResponseWriter{
				ResponseWriter: w,
				release: func() {
					ctx, cancel := context.WithTimeout(releaseCtx, 5*time.Second)
					defer cancel()
					limiter.Release(ctx, tokenHash)
				},
			}
			defer pw.done()

			next.ServeHTTP(pw, r)
		})
	}
}
