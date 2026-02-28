package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
