package ratelimit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// DiscordAlerter sends Discord webhook alerts when token ratio drops below threshold.
type DiscordAlerter struct {
	webhookURL string
	threshold  float64
	cooldown   time.Duration
	client     *http.Client
	mu         sync.Mutex
	alerted    map[string]time.Time
}

// NewDiscordAlerter creates a new DiscordAlerter.
// threshold is 0.0–1.0 (e.g. 0.20 = alert when < 20% remaining).
func NewDiscordAlerter(webhookURL string, threshold float64, cooldown time.Duration) *DiscordAlerter {
	return &DiscordAlerter{
		webhookURL: webhookURL,
		threshold:  threshold,
		cooldown:   cooldown,
		client:     &http.Client{Timeout: 10 * time.Second},
		alerted:    make(map[string]time.Time),
	}
}

// Check inspects the store for providerKey and fires an alert if needed.
func (a *DiscordAlerter) Check(providerKey string, store *Store) {
	st, ok := store.Get(providerKey)
	if !ok || st.TokensLimit == 0 {
		return
	}

	ratio := float64(st.TokensRemaining) / float64(st.TokensLimit)
	if ratio > a.threshold {
		return
	}

	a.mu.Lock()
	last, seen := a.alerted[providerKey]
	if seen && time.Since(last) < a.cooldown {
		a.mu.Unlock()
		return
	}
	a.alerted[providerKey] = time.Now()
	a.mu.Unlock()

	a.sendAlert(providerKey, st, ratio)
}

func (a *DiscordAlerter) sendAlert(providerKey string, st *State, ratio float64) {
	pct := ratio * 100
	payload := map[string]any{
		"embeds": []map[string]any{
			{
				"title": "⚠️ Anthropic Rate Limit 警告",
				"color": 16776960,
				"fields": []map[string]any{
					{"name": "Provider Key", "value": providerKey, "inline": false},
					{"name": "Tokens Remaining", "value": fmt.Sprintf("%d / %d (%.1f%%)", st.TokensRemaining, st.TokensLimit, pct), "inline": false},
					{"name": "Reset At", "value": st.TokensReset.UTC().Format(time.RFC3339), "inline": false},
				},
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, a.webhookURL, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}
