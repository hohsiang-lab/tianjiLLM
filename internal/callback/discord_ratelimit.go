package callback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

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

// CheckAndAlert evaluates rate limit state and sends Discord alerts when thresholds are exceeded.
// Handles both OAuth tokens (via unified headers) and legacy API keys (via per-type headers).
// Token type is detected from the state itself: unified headers present = OAuth token.
func (a *DiscordRateLimitAlerter) CheckAndAlert(state AnthropicOAuthRateLimitState) {
	if state.UnifiedStatus != "" {
		// OAuth token path: rejected = quota exhausted, no extra usage. Alert + early return.
		if state.UnifiedStatus == UnifiedStatusRejected {
			go a.sendAlertIfNotCooling("ratelimit:oauth:"+state.UnifiedStatus+":"+state.TokenKey,
				fmt.Sprintf("🚨 %s", state.UnifiedStatus), state)
			return
		}
		// allowed_warning: extra usage active. Non-blocking alert; continue to utilization checks.
		if state.UnifiedStatus == UnifiedStatusAllowedWarning {
			go a.sendAlertIfNotCooling("ratelimit:oauth:"+UnifiedStatusAllowedWarning+":"+state.TokenKey,
				fmt.Sprintf("⚠️ %s (extra usage active)", UnifiedStatusAllowedWarning), state)
		}
		if state.Unified5hUtilization >= 0 && state.Unified5hUtilization >= a.threshold {
			go a.sendAlertIfNotCooling(fmt.Sprintf("ratelimit:oauth:5h_util:%s", state.TokenKey),
				fmt.Sprintf("⚠️ 5h utilization %.1f%%", state.Unified5hUtilization*100), state)
		}
		if state.Unified7dUtilization >= 0 && state.Unified7dUtilization >= a.threshold {
			go a.sendAlertIfNotCooling(fmt.Sprintf("ratelimit:oauth:7d_util:%s", state.TokenKey),
				fmt.Sprintf("⚠️ 7d utilization %.1f%%", state.Unified7dUtilization*100), state)
		}
		return
	}

	// Legacy API key path: check remaining tokens against limit.
	if state.RequestsLimit > 0 && state.RequestsRemaining >= 0 {
		ratio := float64(state.RequestsRemaining) / float64(state.RequestsLimit)
		if ratio < a.threshold {
			go a.sendAlertIfNotCooling("ratelimit:anthropic:requests:"+state.TokenKey,
				"⚠️ requests", state)
		}
	}
	if state.TokensLimit > 0 && state.TokensRemaining >= 0 {
		ratio := float64(state.TokensRemaining) / float64(state.TokensLimit)
		if ratio < a.threshold {
			go a.sendAlertIfNotCooling("ratelimit:anthropic:tokens:"+state.TokenKey,
				"⚠️ tokens", state)
		}
	}
}

func formatUtilization(v float64) string {
	if v < 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", v*100)
}

// sendAlertIfNotCooling formats an alert message and sends it via sendWithCooldown.
func (a *DiscordRateLimitAlerter) sendAlertIfNotCooling(key, reason string, state AnthropicOAuthRateLimitState) {
	var msg string
	if state.UnifiedStatus != "" {
		msg = fmt.Sprintf(
			"%s — Anthropic OAuth token `%s`\n"+
				"status: **%s** | 5h: **%s** (reset: %s) | 7d: **%s** (reset: %s)\n"+
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
	} else {
		msg = fmt.Sprintf(
			"%s — Anthropic API key `%s`\n"+
				"requests: remaining=%d limit=%d reset=%s\n"+
				"tokens: remaining=%d limit=%d reset=%s",
			reason,
			state.TokenKey,
			state.RequestsRemaining, state.RequestsLimit, state.RequestsResetAt,
			state.TokensRemaining, state.TokensLimit, state.TokensResetAt,
		)
	}
	a.sendWithCooldown(key, msg)
}

// sendWithCooldown checks per-key cooldown, then POSTs msg to the Discord webhook.
// The cooldown entry is reserved before sending to prevent concurrent goroutines from
// double-firing, then cleared on failure so transient webhook errors don't suppress
// alerts for the full cooldown window.
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
		a.clearCooldown(key)
		return
	}
	resp, err := a.client.Post(a.webhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("ERROR ratelimit: Discord webhook request failed: %v", err)
		a.clearCooldown(key)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var body bytes.Buffer
		if _, err := body.ReadFrom(resp.Body); err != nil {
			log.Printf("ERROR ratelimit: failed to read Discord webhook response: %v", err)
		}
		log.Printf("ERROR ratelimit: Discord webhook returned %d: %s", resp.StatusCode, body.String())
		a.clearCooldown(key)
	}
}

// clearCooldown removes the cooldown entry for key so the next alert attempt is not suppressed.
func (a *DiscordRateLimitAlerter) clearCooldown(key string) {
	a.mu.Lock()
	delete(a.alerted, key)
	a.mu.Unlock()
}
