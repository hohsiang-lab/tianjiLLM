package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDynamicRateLimiterNilRedis(t *testing.T) {
	d := NewDynamicRateLimiter(nil)
	d.SetSaturationThreshold(0.9)

	// All methods should be safe with nil redis
	d.RecordUtilization(context.Background(), 0.5)
	d.RecordModelUtilization(context.Background(), "gpt-4", 0.5)
	d.RecordTokens(context.Background(), "hash", "model", 100)

	allowed, err := d.Check(context.Background(), "hash", 1, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("should be allowed with nil redis")
	}

	result, err := d.CheckFull(context.Background(), "hash", "model", 1, 100, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Allowed {
		t.Fatal("should be allowed")
	}
	if result.RPMLimit != 100 {
		t.Fatalf("rpm: %d", result.RPMLimit)
	}
	if result.TPMLimit != 1000 {
		t.Fatalf("tpm: %d", result.TPMLimit)
	}
}

func TestDynamicRateLimiterComputeFactor(t *testing.T) {
	d := NewDynamicRateLimiter(nil)
	d.SetSaturationThreshold(0.8)

	// Below threshold
	f := d.computeFactor(0.5, 1)
	if f != 1.0 {
		t.Fatalf("got %f", f)
	}

	// Zero priority
	f = d.computeFactor(0.9, 0)
	if f != 1.0 {
		t.Fatalf("got %f", f)
	}

	// Above threshold, priority 1
	f = d.computeFactor(0.9, 1)
	if f >= 1.0 || f <= 0 {
		t.Fatalf("got %f", f)
	}

	// Very high saturation
	f = d.computeFactor(1.0, 5)
	if f < 0.1 {
		t.Fatalf("should not go below 0.1, got %f", f)
	}
}

func TestSetSaturationThresholdInvalid(t *testing.T) {
	d := NewDynamicRateLimiter(nil)
	d.SetSaturationThreshold(0)    // invalid, should not change
	d.SetSaturationThreshold(-0.5) // invalid
	d.SetSaturationThreshold(1.5)  // invalid
	if d.saturationThreshold != 0.8 {
		t.Fatalf("threshold: %f", d.saturationThreshold)
	}
	d.SetSaturationThreshold(1.0) // valid edge
	if d.saturationThreshold != 1.0 {
		t.Fatalf("threshold: %f", d.saturationThreshold)
	}
}

func TestSetRateLimitHeaders(t *testing.T) {
	rr := httptest.NewRecorder()
	setRateLimitHeaders(rr, CheckResult{
		RPMLimit:     100,
		RPMRemaining: 50,
		TPMLimit:     1000,
		TPMRemaining: 500,
		ResetSeconds: 60,
		EffectiveRPM: 80,
		EffectiveTPM: 800,
	})

	if rr.Header().Get("X-RateLimit-Limit-Requests") != "80" {
		t.Fatalf("rpm limit: %q", rr.Header().Get("X-RateLimit-Limit-Requests"))
	}
	if rr.Header().Get("X-RateLimit-Remaining-Requests") != "50" {
		t.Fatalf("rpm remaining: %q", rr.Header().Get("X-RateLimit-Remaining-Requests"))
	}
	if rr.Header().Get("X-RateLimit-Limit-Tokens") != "800" {
		t.Fatalf("tpm limit: %q", rr.Header().Get("X-RateLimit-Limit-Tokens"))
	}
	if rr.Header().Get("X-RateLimit-Reset-Requests") != "60s" {
		t.Fatalf("reset: %q", rr.Header().Get("X-RateLimit-Reset-Requests"))
	}
}

func TestDynamicRateLimitMiddlewareNil(t *testing.T) {
	mw := NewDynamicRateLimitMiddleware(nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code: %d", rr.Code)
	}
}

func TestDynamicRateLimitMiddlewareNoToken(t *testing.T) {
	d := NewDynamicRateLimiter(nil)
	mw := NewDynamicRateLimitMiddleware(d)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("POST", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code: %d", rr.Code)
	}
}

func TestDynamicRateLimitMiddlewareWithLimits(t *testing.T) {
	d := NewDynamicRateLimiter(nil)
	mw := NewDynamicRateLimitMiddleware(d)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest("POST", "/", nil)
	ctx := context.WithValue(req.Context(), tokenHashKey, "hash123")
	ctx = context.WithValue(ctx, rpmLimitKey, int64(100))
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code: %d", rr.Code)
	}
}

func TestCheckZeroLimits(t *testing.T) {
	d := NewDynamicRateLimiter(nil)
	result, err := d.CheckFull(context.Background(), "hash", "", 1, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Allowed {
		t.Fatal("zero limits should allow")
	}
}

func TestGetSaturationNilRedis(t *testing.T) {
	d := NewDynamicRateLimiter(nil)
	s := d.getSaturation(context.Background(), "")
	if s != 0 {
		t.Fatalf("got %f", s)
	}
}

// --- RPM/TPM enforcement tests with miniredis (T012–T014a) ---

func setupDynamicLimiter(t *testing.T) (*DynamicRateLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewDynamicRateLimiter(rdb), mr
}

func TestDynamicRateLimit_RPMExceeded(t *testing.T) {
	limiter, _ := setupDynamicLimiter(t)
	mw := NewDynamicRateLimitMiddleware(limiter)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	makeReq := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		ctx := context.WithValue(req.Context(), tokenHashKey, "testkey123hash")
		ctx = context.WithValue(ctx, rpmLimitKey, int64(10))
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}

	// Send 10 requests — all should pass
	for i := 0; i < 10; i++ {
		rr := makeReq()
		assert.Equal(t, http.StatusOK, rr.Code, "request %d should be allowed", i+1)
	}

	// 11th request should be rejected
	rr := makeReq()
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "11th request should be rate limited")
}

func TestDynamicRateLimit_TPMExceeded(t *testing.T) {
	limiter, mr := setupDynamicLimiter(t)
	mw := NewDynamicRateLimitMiddleware(limiter)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	keyHash := "testkey456hash"

	// Pre-populate Redis TPM counter at the limit
	require.NoError(t, mr.Set("tianji:dynamic_tpm:"+keyHash, "100000"))

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	ctx := context.WithValue(req.Context(), tokenHashKey, keyHash)
	ctx = context.WithValue(ctx, tpmLimitKey, int64(100000))
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "request should be rejected when TPM counter >= limit")
}

func TestDynamicRateLimit_WindowReset(t *testing.T) {
	limiter, mr := setupDynamicLimiter(t)

	keyHash := "testreset789hash"
	ctx := context.Background()

	// Exhaust the RPM limit
	for i := 0; i < 5; i++ {
		result, err := limiter.CheckFull(ctx, keyHash, "", 0, 5, 0)
		require.NoError(t, err)
		assert.True(t, result.Allowed, "request %d should be allowed", i+1)
	}

	// 6th should fail
	result, err := limiter.CheckFull(ctx, keyHash, "", 0, 5, 0)
	require.NoError(t, err)
	assert.False(t, result.Allowed, "6th request should be denied")

	// Fast-forward miniredis past the 60s TTL
	mr.FastForward(61 * time.Second)

	// After TTL expiry, the key is gone — next request should be allowed
	result, err = limiter.CheckFull(ctx, keyHash, "", 0, 5, 0)
	require.NoError(t, err)
	assert.True(t, result.Allowed, "request after window reset should be allowed")
}

func TestDynamicRateLimit_SetsRateLimitHeaders(t *testing.T) {
	limiter, _ := setupDynamicLimiter(t)
	mw := NewDynamicRateLimitMiddleware(limiter)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	ctx := context.WithValue(req.Context(), tokenHashKey, "headerkey123")
	ctx = context.WithValue(ctx, rpmLimitKey, int64(100))
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Limit-Requests"), "should set X-RateLimit-Limit-Requests header")
	assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Remaining-Requests"), "should set X-RateLimit-Remaining-Requests header")
	assert.NotEmpty(t, rr.Header().Get("X-RateLimit-Reset-Requests"), "should set X-RateLimit-Reset-Requests header")

	// Verify remaining is limit - 1 (first request)
	assert.Equal(t, "100", rr.Header().Get("X-RateLimit-Limit-Requests"))
	assert.Equal(t, "99", rr.Header().Get("X-RateLimit-Remaining-Requests"))
}
