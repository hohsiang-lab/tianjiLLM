package callback

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDiscordServerForOAuth creates an httptest server that records received payloads for OAuth alerts.
func mockDiscordServerForOAuth(t *testing.T, statusCode int) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var msgs []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err == nil {
			mu.Lock()
			msgs = append(msgs, payload["content"])
			mu.Unlock()
		}
		w.WriteHeader(statusCode)
	}))

	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		cp := make([]string, len(msgs))
		copy(cp, msgs)
		return cp
	}
}

// --- OAuth Rate Limit Alert Tests ---

// Test 1: unified_5h_utilization >= 80% -> should trigger alert
func TestCheckAndAlertOAuth_5hUtilizationAbove80Percent_TriggersAlert(t *testing.T) {
	srv, getMsgs := mockDiscordServerForOAuth(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.8)
	require.NotNil(t, a, "alerter should be created with valid webhook URL")
	a.cooldown = 0 // disable cooldown for this test

	// Simulate OAuth state with 5h utilization >= 80%
	state := AnthropicOAuthRateLimitState{
		TokenKey:             "abc123",
		UnifiedStatus:        "allowed",
		Unified5hStatus:      "allowed",
		Unified5hUtilization: 0.85, // 85% - above 80% threshold
		Unified7dUtilization: 0.3,
	}

	a.CheckAndAlertOAuth(state)
	time.Sleep(100 * time.Millisecond)

	msgs := getMsgs()
	require.Len(t, msgs, 1, "alert should fire when Unified5hUtilization >= 80%")
	assert.Contains(t, msgs[0], "85.0", "alert should mention the utilization value")
	assert.Contains(t, msgs[0], "5h", "alert should mention 5h utilization")
}

// Test 2: unified_status == "rate_limited" -> should trigger alert immediately
func TestCheckAndAlertOAuth_StatusRateLimited_TriggersAlert(t *testing.T) {
	srv, getMsgs := mockDiscordServerForOAuth(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.8)
	require.NotNil(t, a)
	a.cooldown = 0

	// Simulate OAuth state with status = "rate_limited"
	state := AnthropicOAuthRateLimitState{
		TokenKey:             "def456",
		UnifiedStatus:        "rate_limited",
		Unified5hStatus:      "allowed",
		Unified5hUtilization: 0.5,
		Unified7dUtilization: 0.3,
	}

	a.CheckAndAlertOAuth(state)
	time.Sleep(100 * time.Millisecond)

	msgs := getMsgs()
	require.Len(t, msgs, 1, "alert should fire when UnifiedStatus == rate_limited")
	assert.Contains(t, msgs[0], "rate_limited", "alert should mention rate_limited status")
}

// Test 3: same token within 1 hour should NOT trigger duplicate alert (cooldown)
func TestCheckAndAlertOAuth_Cooldown_SameTokenFiresOnlyOnce(t *testing.T) {
	var callCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.8)
	require.NotNil(t, a)
	// Use default 1h cooldown (do NOT set to 0)

	state := AnthropicOAuthRateLimitState{
		TokenKey:             "ghi789",
		UnifiedStatus:        "rate_limited",
		Unified5hStatus:      "allowed",
		Unified5hUtilization: 0.9,
		Unified7dUtilization: 0.3,
	}

	// Fire 10 triggering responses for the same token
	for i := 0; i < 10; i++ {
		a.CheckAndAlertOAuth(state)
	}
	time.Sleep(150 * time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount),
		"cooldown: only first alert should fire within 1h for the same token key")
}

// --- Edge cases ---

// Test: utilization exactly at 80% should trigger alert
func TestCheckAndAlertOAuth_5hUtilizationExactly80Percent_TriggersAlert(t *testing.T) {
	srv, getMsgs := mockDiscordServerForOAuth(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.8)
	require.NotNil(t, a)
	a.cooldown = 0

	state := AnthropicOAuthRateLimitState{
		TokenKey:             "jkl012",
		UnifiedStatus:        "allowed",
		Unified5hStatus:      "allowed",
		Unified5hUtilization: 0.8, // exactly 80%
	}

	a.CheckAndAlertOAuth(state)
	time.Sleep(100 * time.Millisecond)

	msgs := getMsgs()
	require.Len(t, msgs, 1, "alert should fire when Unified5hUtilization == 80%")
}

// Test: utilization below 80% should NOT trigger alert
func TestCheckAndAlertOAuth_5hUtilizationBelow80Percent_NoAlert(t *testing.T) {
	srv, getMsgs := mockDiscordServerForOAuth(t, 200)
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.8)
	require.NotNil(t, a)
	a.cooldown = 0

	state := AnthropicOAuthRateLimitState{
		TokenKey:             "mno345",
		UnifiedStatus:        "allowed",
		Unified5hStatus:      "allowed",
		Unified5hUtilization: 0.75, // below 80%
	}

	a.CheckAndAlertOAuth(state)
	time.Sleep(100 * time.Millisecond)

	assert.Len(t, getMsgs(), 0, "no alert when Unified5hUtilization < 80%")
}

// Test: different token keys should trigger independent alerts
func TestCheckAndAlertOAuth_DifferentTokens_IndependentAlerts(t *testing.T) {
	var callCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	a := NewDiscordRateLimitAlerter(srv.URL, 0.8)
	require.NotNil(t, a)
	a.cooldown = 0

	// First token triggers alert
	state1 := AnthropicOAuthRateLimitState{
		TokenKey:             "token1",
		UnifiedStatus:        "rate_limited",
		Unified5hStatus:      "allowed",
		Unified5hUtilization: 0.5,
	}
	a.CheckAndAlertOAuth(state1)

	// Second token (different key) should also trigger alert
	state2 := AnthropicOAuthRateLimitState{
		TokenKey:             "token2",
		UnifiedStatus:        "rate_limited",
		Unified5hStatus:      "allowed",
		Unified5hUtilization: 0.5,
	}
	a.CheckAndAlertOAuth(state2)

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount),
		"different token keys should trigger independent alerts")
}
