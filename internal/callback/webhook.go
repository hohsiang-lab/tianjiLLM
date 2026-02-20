package callback

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// WebhookCallback sends structured log data to an HTTP webhook.
type WebhookCallback struct {
	url     string
	client  *http.Client
	headers map[string]string
}

// NewWebhookCallback creates a generic webhook callback.
func NewWebhookCallback(url string, headers map[string]string) *WebhookCallback {
	return &WebhookCallback{
		url:     url,
		client:  &http.Client{Timeout: 5 * time.Second},
		headers: headers,
	}
}

type webhookPayload struct {
	Event            string   `json:"event"`
	Model            string   `json:"model"`
	Provider         string   `json:"provider"`
	PromptTokens     int      `json:"prompt_tokens"`
	CompletionTokens int      `json:"completion_tokens"`
	TotalTokens      int      `json:"total_tokens"`
	Cost             float64  `json:"cost"`
	Latency          float64  `json:"latency_seconds"`
	UserID           string   `json:"user_id,omitempty"`
	TeamID           string   `json:"team_id,omitempty"`
	RequestTags      []string `json:"request_tags,omitempty"`
	CacheHit         bool     `json:"cache_hit"`
	Error            string   `json:"error,omitempty"`
	Timestamp        string   `json:"timestamp"`
}

func (w *WebhookCallback) LogSuccess(data LogData) {
	w.send(webhookPayload{
		Event:            "llm.success",
		Model:            data.Model,
		Provider:         data.Provider,
		PromptTokens:     data.PromptTokens,
		CompletionTokens: data.CompletionTokens,
		TotalTokens:      data.TotalTokens,
		Cost:             data.Cost,
		Latency:          data.Latency.Seconds(),
		UserID:           data.UserID,
		TeamID:           data.TeamID,
		RequestTags:      data.RequestTags,
		CacheHit:         data.CacheHit,
		Timestamp:        data.EndTime.UTC().Format(time.RFC3339),
	})
}

func (w *WebhookCallback) LogFailure(data LogData) {
	errMsg := ""
	if data.Error != nil {
		errMsg = data.Error.Error()
	}
	w.send(webhookPayload{
		Event:     "llm.failure",
		Model:     data.Model,
		Provider:  data.Provider,
		Latency:   data.Latency.Seconds(),
		UserID:    data.UserID,
		TeamID:    data.TeamID,
		Error:     errMsg,
		Timestamp: data.EndTime.UTC().Format(time.RFC3339),
	})
}

func (w *WebhookCallback) send(payload webhookPayload) {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("warn: webhook marshal failed: %v", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		log.Printf("warn: webhook request failed: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		log.Printf("warn: webhook send failed: %v", err)
		return
	}
	resp.Body.Close()
}
