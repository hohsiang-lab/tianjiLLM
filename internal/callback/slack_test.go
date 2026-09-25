package callback

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlackCallback_AlertRouting(t *testing.T) {
	var defaultMsgs, failMsgs []string
	var mu sync.Mutex

	defaultSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		defaultMsgs = append(defaultMsgs, body["text"])
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer defaultSrv.Close()

	failSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		failMsgs = append(failMsgs, body["text"])
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer failSrv.Close()

	s := NewSlackCallback(defaultSrv.URL, 0.8)
	s.cooldown = 0 // disable throttling for test
	s.SetAlertWebhooks(map[string]string{
		"fail": failSrv.URL,
	})

	// Failure alert should go to fail webhook
	s.LogFailure(LogData{
		Model:    "gpt-4",
		Provider: "openai",
		Error:    assert.AnError,
	})

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	assert.Len(t, failMsgs, 1, "failure should route to fail webhook")
	assert.Len(t, defaultMsgs, 0, "default webhook should not receive fail alerts")
	mu.Unlock()
}

func TestSlackCallback_HangingDetection(t *testing.T) {
	var msgs []string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		msgs = append(msgs, body["text"])
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	s := NewSlackCallback(srv.URL, 0.8)
	s.cooldown = 0
	s.SetHangingThreshold(1 * time.Millisecond) // very low for testing

	s.TrackRequestStart("req-1")
	time.Sleep(5 * time.Millisecond)

	s.CheckHangingRequests()

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0], "Hanging Request Alert")
	assert.Contains(t, msgs[0], "req-1")
	mu.Unlock()

	// After tracking end, no more hanging
	s.TrackRequestEnd("req-1")
	msgs = nil
	s.CheckHangingRequests()
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	assert.Len(t, msgs, 0)
	mu.Unlock()
}

func TestSlackCallback_OutageDetection(t *testing.T) {
	var msgs []string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		msgs = append(msgs, body["text"])
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	s := NewSlackCallback(srv.URL, 0.8)
	s.cooldown = 0
	s.SetOutageErrorRate(0.5)

	// Simulate 10 failures (100% error rate, >= 10 samples)
	for i := 0; i < 10; i++ {
		s.recordProviderResult("openai", false)
	}

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	require.GreaterOrEqual(t, len(msgs), 1)
	assert.Contains(t, msgs[0], "Provider Outage Alert")
	assert.Contains(t, msgs[0], "openai")
	mu.Unlock()
}

func TestSlackCallback_DailyReport(t *testing.T) {
	var msgs []string
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		msgs = append(msgs, body["text"])
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	s := NewSlackCallback(srv.URL, 0.8)

	// Log some events
	s.LogSuccess(LogData{Model: "gpt-4", Provider: "openai", Cost: 0.01})
	s.LogSuccess(LogData{Model: "gpt-4", Provider: "openai", Cost: 0.02})
	s.LogFailure(LogData{Model: "claude-3", Provider: "anthropic", Error: assert.AnError})

	// Get report
	report := s.GetDailyReport()
	assert.Equal(t, 3, report.TotalRequests)
	assert.Equal(t, 1, report.TotalErrors)
	assert.InDelta(t, 0.03, report.TotalCost, 0.001)
	assert.Equal(t, 2, report.ModelCounts["gpt-4"])
	assert.Equal(t, 1, report.ModelCounts["claude-3"])

	// After getting report, stats should be reset
	report2 := s.GetDailyReport()
	assert.Equal(t, 0, report2.TotalRequests)
}

func TestSlackCallback_Throttling(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	s := NewSlackCallback(srv.URL, 0.8)
	s.cooldown = 1 * time.Hour // long cooldown

	// Same key should be throttled
	s.sendThrottled("test-key", "msg1")
	s.sendThrottled("test-key", "msg2")

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	assert.Equal(t, 1, callCount, "second message should be throttled")
	mu.Unlock()
}
