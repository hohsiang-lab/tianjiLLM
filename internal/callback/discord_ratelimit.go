package callback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// AnthropicRateLimitState holds raw parsed header values.
// No derived or computed fields. Values exactly as received from Anthropic response headers.
// Missing or unparseable integer fields are set to -1 as sentinel (C-04).
type AnthropicRateLimitState struct {
	InputTokensLimit      int64
	InputTokensRemaining  int64
	InputTokensReset      string // raw RFC3339 string from header; "" if missing
	OutputTokensLimit     int64
	OutputTokensRemaining int64
	OutputTokensReset     string
	RequestsLimit         int64
	RequestsRemaining     int64
	RequestsReset         string
}

// ParseAnthropicRateLimitHeaders reads Anthropic rate limit headers from an HTTP response.
// Missing or unparseable integer headers are logged as errors and set to -1 (C-04, FR-003, FR-004).
// Missing reset string headers are set to "".
func ParseAnthropicRateLimitHeaders(h http.Header) AnthropicRateLimitState {
	var s AnthropicRateLimitState

	parseInt64OrNeg1 := func(name string) int64 {
		raw := h.Get(name)
		if raw == "" {
			log.Printf("ERROR ratelimit: Anthropic response missing header %q", name)
			return -1
		}
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			log.Printf("ERROR ratelimit: cannot parse header %q value %q: %v", name, raw, err)
			return -1
		}
		return v
	}
	getResetOrEmpty := func(name string) string {
		raw := h.Get(name)
		if raw == "" {
			log.Printf("ERROR ratelimit: Anthropic response missing header %q", name)
		}
		return raw
	}

	s.InputTokensLimit = parseInt64OrNeg1("anthropic-ratelimit-input-tokens-limit")
	s.InputTokensRemaining = parseInt64OrNeg1("anthropic-ratelimit-input-tokens-remaining")
	s.InputTokensReset = getResetOrEmpty("anthropic-ratelimit-input-tokens-reset")

	s.OutputTokensLimit = parseInt64OrNeg1("anthropic-ratelimit-output-tokens-limit")
	s.OutputTokensRemaining = parseInt64OrNeg1("anthropic-ratelimit-output-tokens-remaining")
	s.OutputTokensReset = getResetOrEmpty("anthropic-ratelimit-output-tokens-reset")

	s.RequestsLimit = parseInt64OrNeg1("anthropic-ratelimit-requests-limit")
	s.RequestsRemaining = parseInt64OrNeg1("anthropic-ratelimit-requests-remaining")
	s.RequestsReset = getResetOrEmpty("anthropic-ratelimit-requests-reset")

	return s
}

// DiscordRateLimitAlerter sends Discord alerts when Anthropic rate limit tokens drop below threshold.
// Thread-safe; uses per-key cooldown to prevent alert spam (FR-008).
type DiscordRateLimitAlerter struct {
	webhookURL string
	threshold  float64
	cooldown   time.Duration
	mu         sync.Mutex
	alerted    map[string]time.Time
	client     *http.Client
}

// NewDiscordRateLimitAlerter creates a new alerter. Returns nil if webhookURL is empty (SC-005).
// If threshold is 0, defaults to 0.2 (20%).
func NewDiscordRateLimitAlerter(webhookURL string, threshold float64) *DiscordRateLimitAlerter {
	if webhookURL == "" {
		return nil
	}
	if threshold == 0 {
		threshold = 0.2
	}
	return &DiscordRateLimitAlerter{
		webhookURL: webhookURL,
		threshold:  threshold,
		cooldown:   1 * time.Hour,
		alerted:    make(map[string]time.Time),
		client:     &http.Client{Timeout: 5 * time.Second},
	}
}

// CheckAndAlert evaluates the rate limit state against the threshold and sends Discord alerts
// if appropriate. Alert sending is non-blocking (goroutine). FR-005, FR-006, FR-007, FR-008.
// Fields set to -1 (sentinel) are skipped (C-04).
func (a *DiscordRateLimitAlerter) CheckAndAlert(state AnthropicRateLimitState) {
	// input check
	if state.InputTokensLimit > 0 && state.InputTokensRemaining >= 0 {
		ratio := float64(state.InputTokensRemaining) / float64(state.InputTokensLimit)
		if ratio < a.threshold {
			go a.sendIfNotCooling("ratelimit:anthropic:input", "input", state)
		}
	}
	// output check
	if state.OutputTokensLimit > 0 && state.OutputTokensRemaining >= 0 {
		ratio := float64(state.OutputTokensRemaining) / float64(state.OutputTokensLimit)
		if ratio < a.threshold {
			go a.sendIfNotCooling("ratelimit:anthropic:output", "output", state)
		}
	}
}

// sendIfNotCooling formats a legacy rate limit alert and sends it via sendWithCooldown.
func (a *DiscordRateLimitAlerter) sendIfNotCooling(key, alertType string, state AnthropicRateLimitState) {
	msg := fmt.Sprintf(
		"⚠️ Anthropic rate limit alert (%s)\n"+
			"Input: limit=%d remaining=%d reset=%s\n"+
			"Output: limit=%d remaining=%d reset=%s\n"+
			"Requests: limit=%d remaining=%d reset=%s",
		alertType,
		state.InputTokensLimit, state.InputTokensRemaining, state.InputTokensReset,
		state.OutputTokensLimit, state.OutputTokensRemaining, state.OutputTokensReset,
		state.RequestsLimit, state.RequestsRemaining, state.RequestsReset,
	)
	a.sendWithCooldown(key, msg)
}

// CheckAndAlertOAuth evaluates unified OAuth rate limit state and sends Discord alerts if:
//   - UnifiedStatus == "rate_limited"
//   - Unified5hUtilization >= threshold
//   - Unified7dUtilization >= threshold
//
// Alert sending is non-blocking (goroutine). Per-key cooldown prevents spam.
// When status is "rate_limited", only the rate_limited alert fires (early return) —
// utilization alerts are redundant since the token is already hard-blocked by Anthropic.
func (a *DiscordRateLimitAlerter) CheckAndAlertOAuth(state AnthropicOAuthRateLimitState) {
	if state.UnifiedStatus == "rate_limited" {
		go a.sendOAuthAlertIfNotCooling("ratelimit:oauth:rate_limited:"+state.TokenKey, "🚨 rate_limited", state)
		return
	}
	if state.Unified5hUtilization >= 0 && state.Unified5hUtilization >= a.threshold {
		key := fmt.Sprintf("ratelimit:oauth:5h_util:%s", state.TokenKey)
		go a.sendOAuthAlertIfNotCooling(key, fmt.Sprintf("⚠️ 5h utilization %.1f%%", state.Unified5hUtilization*100), state)
	}
	if state.Unified7dUtilization >= 0 && state.Unified7dUtilization >= a.threshold {
		key := fmt.Sprintf("ratelimit:oauth:7d_util:%s", state.TokenKey)
		go a.sendOAuthAlertIfNotCooling(key, fmt.Sprintf("⚠️ 7d utilization %.1f%%", state.Unified7dUtilization*100), state)
	}
}

func formatUtilization(v float64) string {
	if v < 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", v*100)
}

// sendOAuthAlertIfNotCooling formats an OAuth rate limit alert and sends it via sendWithCooldown.
func (a *DiscordRateLimitAlerter) sendOAuthAlertIfNotCooling(key, reason string, state AnthropicOAuthRateLimitState) {
	msg := fmt.Sprintf(
		"%s — Anthropic OAuth token `%s`\n"+
			"status: **%s** | 5h utilization: **%s** (reset: %s) | 7d utilization: **%s** (reset: %s)\n"+
			"representative claim: %s",
		reason,
		state.TokenKey,
		state.UnifiedStatus,
		formatUtilization(state.Unified5hUtilization),
		state.Unified5hReset,
		formatUtilization(state.Unified7dUtilization),
		state.Unified7dReset,
		state.RepresentativeClaim,
	)
	a.sendWithCooldown(key, msg)
}

// sendWithCooldown checks per-key cooldown, then POSTs msg to the Discord webhook.
// Shared by both legacy and OAuth alert paths.
func (a *DiscordRateLimitAlerter) sendWithCooldown(key, msg string) {
	a.mu.Lock()
	last, exists := a.alerted[key]
	if exists && time.Since(last) < a.cooldown {
		a.mu.Unlock()
		return
	}
	a.alerted[key] = time.Now()
	a.mu.Unlock()

	payload, err := json.Marshal(map[string]string{"content": msg})
	if err != nil {
		log.Printf("ERROR ratelimit: failed to marshal Discord payload: %v", err)
		return
	}
	resp, err := a.client.Post(a.webhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("ERROR ratelimit: Discord webhook request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var body bytes.Buffer
		if _, err := body.ReadFrom(resp.Body); err != nil {
			log.Printf("ERROR ratelimit: failed to read Discord webhook response: %v", err)
		}
		log.Printf("ERROR ratelimit: Discord webhook returned %d: %s", resp.StatusCode, body.String())
	}
}
