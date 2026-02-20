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

// SlackCallback sends budget threshold alerts and failure notifications via Slack webhook.
// Tracks cumulative spend per user/team and alerts when budget threshold is crossed.
// Supports per-alert-type webhook routing, hanging request detection, outage detection,
// and daily usage report aggregation.
type SlackCallback struct {
	webhookURL        string
	alertToWebhookURL map[string]string // per-alert-type webhook routing
	budgetThreshold   float64           // percentage (0.0-1.0) at which to alert
	hangingThreshold  time.Duration     // alert when request exceeds this duration
	outageErrorRate   float64           // alert when provider error rate exceeds this (0.0-1.0)
	client            *http.Client

	mu       sync.Mutex
	alerted  map[string]time.Time // tracks last alert time per key to avoid spam
	cooldown time.Duration

	// Hanging request tracking
	inFlightMu sync.Mutex
	inFlight   map[string]time.Time // requestID → start time

	// Outage detection: sliding window of success/failure per provider
	outageMu     sync.Mutex
	providerHits map[string]*errorWindow

	// Daily report aggregation
	dailyMu    sync.Mutex
	dailyStats *dailyReport
}

// errorWindow tracks recent success/failure counts for outage detection.
type errorWindow struct {
	successes int
	failures  int
	resetAt   time.Time
}

// dailyReport accumulates stats for the daily usage report.
type dailyReport struct {
	TotalRequests int
	TotalErrors   int
	TotalCost     float64
	ModelCounts   map[string]int
	StartTime     time.Time
}

// NewSlackCallback creates a Slack alerting callback.
func NewSlackCallback(webhookURL string, budgetThreshold float64) *SlackCallback {
	if budgetThreshold == 0 {
		budgetThreshold = 0.8
	}
	s := &SlackCallback{
		webhookURL:       webhookURL,
		budgetThreshold:  budgetThreshold,
		hangingThreshold: 5 * time.Minute,
		outageErrorRate:  0.5,
		client:           &http.Client{Timeout: 5 * time.Second},
		alerted:          make(map[string]time.Time),
		cooldown:         1 * time.Hour,
		inFlight:         make(map[string]time.Time),
		providerHits:     make(map[string]*errorWindow),
		dailyStats:       &dailyReport{ModelCounts: make(map[string]int), StartTime: time.Now()},
	}
	return s
}

// SetAlertWebhooks configures per-alert-type webhook URLs.
// Alert types: "slow", "fail", "budget", "hanging", "outage", "daily".
func (s *SlackCallback) SetAlertWebhooks(m map[string]string) {
	s.alertToWebhookURL = m
}

// SetHangingThreshold configures the threshold for hanging request detection.
func (s *SlackCallback) SetHangingThreshold(d time.Duration) {
	s.hangingThreshold = d
}

// SetOutageErrorRate configures the error rate threshold for outage detection.
func (s *SlackCallback) SetOutageErrorRate(rate float64) {
	s.outageErrorRate = rate
}

// TrackRequestStart records the start of a request for hanging detection.
func (s *SlackCallback) TrackRequestStart(requestID string) {
	s.inFlightMu.Lock()
	s.inFlight[requestID] = time.Now()
	s.inFlightMu.Unlock()
}

// TrackRequestEnd removes a request from in-flight tracking.
func (s *SlackCallback) TrackRequestEnd(requestID string) {
	s.inFlightMu.Lock()
	delete(s.inFlight, requestID)
	s.inFlightMu.Unlock()
}

// CheckHangingRequests scans in-flight requests and alerts on any exceeding the threshold.
func (s *SlackCallback) CheckHangingRequests() {
	s.inFlightMu.Lock()
	now := time.Now()
	var hanging []string
	for id, start := range s.inFlight {
		if now.Sub(start) > s.hangingThreshold {
			hanging = append(hanging, fmt.Sprintf("`%s` (%.0fs)", id, now.Sub(start).Seconds()))
		}
	}
	s.inFlightMu.Unlock()

	if len(hanging) > 0 {
		msg := fmt.Sprintf(":hourglass: *TianjiLLM Hanging Request Alert*\n"+
			"%d request(s) exceeding %v threshold:\n%s",
			len(hanging), s.hangingThreshold, joinLines(hanging))
		s.sendToWebhook("hanging", msg)
	}
}

// GetDailyReport returns the current daily stats and resets for the next period.
func (s *SlackCallback) GetDailyReport() *dailyReport {
	s.dailyMu.Lock()
	report := s.dailyStats
	s.dailyStats = &dailyReport{ModelCounts: make(map[string]int), StartTime: time.Now()}
	s.dailyMu.Unlock()
	return report
}

// SendDailyReport posts the aggregated daily usage report to Slack.
func (s *SlackCallback) SendDailyReport() {
	report := s.GetDailyReport()
	if report.TotalRequests == 0 {
		return
	}

	modelSummary := ""
	for m, c := range report.ModelCounts {
		modelSummary += fmt.Sprintf("  `%s`: %d requests\n", m, c)
	}

	msg := fmt.Sprintf(":bar_chart: *TianjiLLM Daily Report*\n"+
		"Period: %s → %s\n"+
		"Total Requests: *%d* | Errors: *%d* (%.1f%%)\n"+
		"Total Cost: *$%.4f*\n"+
		"Models:\n%s",
		report.StartTime.Format("15:04"), time.Now().Format("15:04"),
		report.TotalRequests, report.TotalErrors,
		float64(report.TotalErrors)/float64(report.TotalRequests)*100,
		report.TotalCost, modelSummary)
	s.sendToWebhook("daily", msg)
}

func (s *SlackCallback) LogSuccess(data LogData) {
	// Aggregate daily stats
	s.dailyMu.Lock()
	s.dailyStats.TotalRequests++
	s.dailyStats.TotalCost += data.Cost
	s.dailyStats.ModelCounts[data.Model]++
	s.dailyMu.Unlock()

	// Track provider success for outage detection
	s.recordProviderResult(data.Provider, true)

	if data.Cost <= 0 {
		return
	}

	// Alert on slow requests (>30s latency)
	if data.Latency > 30*time.Second {
		msg := fmt.Sprintf(":warning: *TianjiLLM Slow Request Alert*\n"+
			"Model: `%s` | Provider: `%s`\n"+
			"Latency: `%.1fs`\n"+
			"User: `%s` | Team: `%s`",
			data.Model, data.Provider, data.Latency.Seconds(),
			data.UserID, data.TeamID)
		s.sendThrottledToWebhook("slow", "slow:"+data.Model, msg)
	}
}

func (s *SlackCallback) LogFailure(data LogData) {
	// Aggregate daily stats
	s.dailyMu.Lock()
	s.dailyStats.TotalRequests++
	s.dailyStats.TotalErrors++
	s.dailyStats.ModelCounts[data.Model]++
	s.dailyMu.Unlock()

	// Track provider failure for outage detection
	s.recordProviderResult(data.Provider, false)

	if data.Error == nil {
		return
	}

	msg := fmt.Sprintf(":x: *TianjiLLM Request Failed*\n"+
		"Model: `%s` | Provider: `%s`\n"+
		"Error: `%v`\n"+
		"User: `%s` | Team: `%s`",
		data.Model, data.Provider, data.Error,
		data.UserID, data.TeamID)

	s.sendThrottledToWebhook("fail", "fail:"+data.Model, msg)
}

// recordProviderResult tracks success/failure per provider and checks outage threshold.
func (s *SlackCallback) recordProviderResult(providerName string, success bool) {
	if providerName == "" {
		return
	}
	s.outageMu.Lock()
	w, ok := s.providerHits[providerName]
	if !ok || time.Now().After(w.resetAt) {
		w = &errorWindow{resetAt: time.Now().Add(5 * time.Minute)}
		s.providerHits[providerName] = w
	}
	if success {
		w.successes++
	} else {
		w.failures++
	}
	total := w.successes + w.failures
	errorRate := float64(w.failures) / float64(total)
	s.outageMu.Unlock()

	// Alert if error rate exceeds threshold with minimum sample size
	if total >= 10 && errorRate >= s.outageErrorRate {
		msg := fmt.Sprintf(":fire: *TianjiLLM Provider Outage Alert*\n"+
			"Provider: `%s`\n"+
			"Error rate: *%.0f%%* (%d/%d requests in last 5min)",
			providerName, errorRate*100, w.failures, total)
		s.sendThrottledToWebhook("outage", "outage:"+providerName, msg)
	}
}

// AlertBudgetThreshold sends a budget alert when spend crosses the threshold.
// Called externally by budget-checking middleware, not by the callback loop itself.
func (s *SlackCallback) AlertBudgetThreshold(entityType, entityID string, currentSpend, maxBudget float64) {
	pct := currentSpend / maxBudget * 100
	key := fmt.Sprintf("budget:%s:%s", entityType, entityID)
	msg := fmt.Sprintf(":rotating_light: *TianjiLLM Budget Alert*\n"+
		"%s `%s` has used *%.1f%%* of budget\n"+
		"Spend: $%.4f / $%.4f",
		entityType, entityID, pct, currentSpend, maxBudget)
	s.sendThrottledToWebhook("budget", key, msg)
}

// sendThrottledToWebhook sends a message with alert-type routing and throttling.
func (s *SlackCallback) sendThrottledToWebhook(alertType, key, text string) {
	s.mu.Lock()
	if last, ok := s.alerted[key]; ok && time.Since(last) < s.cooldown {
		s.mu.Unlock()
		return
	}
	s.alerted[key] = time.Now()
	s.mu.Unlock()

	s.sendToWebhook(alertType, text)
}

// sendThrottled sends a Slack message but throttles duplicate keys to avoid spam.
func (s *SlackCallback) sendThrottled(key, text string) {
	s.sendThrottledToWebhook("", key, text)
}

// sendToWebhook sends to an alert-type-specific webhook URL or the default.
func (s *SlackCallback) sendToWebhook(alertType, text string) {
	url := s.webhookURL
	if s.alertToWebhookURL != nil {
		if u, ok := s.alertToWebhookURL[alertType]; ok && u != "" {
			url = u
		}
	}
	s.sendURL(url, text)
}

func (s *SlackCallback) sendURL(url, text string) {
	payload, _ := json.Marshal(map[string]string{"text": text})

	resp, err := s.client.Do(mustNewRequest(http.MethodPost, url, payload))
	if err != nil {
		log.Printf("warn: slack webhook failed: %v", err)
		return
	}
	resp.Body.Close()
}

func mustNewRequest(method, url string, body []byte) *http.Request {
	req, _ := http.NewRequest(method, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func joinLines(lines []string) string {
	result := ""
	for _, l := range lines {
		result += "• " + l + "\n"
	}
	return result
}
