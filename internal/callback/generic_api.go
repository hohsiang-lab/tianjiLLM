package callback

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// GenericAPICallback sends log events to an arbitrary HTTP endpoint.
// This is the extensibility framework for third-party callback integrations
// that don't need a custom implementation — users configure a URL and optional
// headers, and all log events are POSTed as JSON.
type GenericAPICallback struct {
	url     string
	client  *http.Client
	headers map[string]string
}

// NewGenericAPICallback creates a generic API callback.
// url: the HTTP endpoint to POST events to.
// headers: optional headers to include in every request (e.g. Authorization).
func NewGenericAPICallback(url string, headers map[string]string) *GenericAPICallback {
	return &GenericAPICallback{
		url:     url,
		client:  &http.Client{Timeout: 10 * time.Second},
		headers: headers,
	}
}

// genericPayload is the full event payload sent to the API endpoint.
type genericPayload struct {
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
	StartTime        string   `json:"start_time"`
	EndTime          string   `json:"end_time"`
}

func (g *GenericAPICallback) LogSuccess(data LogData) {
	g.send(genericPayload{
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
		StartTime:        data.StartTime.UTC().Format(time.RFC3339),
		EndTime:          data.EndTime.UTC().Format(time.RFC3339),
	})
}

func (g *GenericAPICallback) LogFailure(data LogData) {
	errMsg := ""
	if data.Error != nil {
		errMsg = data.Error.Error()
	}
	g.send(genericPayload{
		Event:       "llm.failure",
		Model:       data.Model,
		Provider:    data.Provider,
		Latency:     data.Latency.Seconds(),
		UserID:      data.UserID,
		TeamID:      data.TeamID,
		RequestTags: data.RequestTags,
		Error:       errMsg,
		StartTime:   data.StartTime.UTC().Format(time.RFC3339),
		EndTime:     data.EndTime.UTC().Format(time.RFC3339),
	})
}

func (g *GenericAPICallback) send(payload genericPayload) {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("warn: generic_api callback marshal failed: %v", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, g.url, bytes.NewReader(body))
	if err != nil {
		log.Printf("warn: generic_api callback request creation failed: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range g.headers {
		req.Header.Set(k, v)
	}

	resp, err := g.client.Do(req)
	if err != nil {
		log.Printf("warn: generic_api callback send failed: %v", err)
		return
	}
	resp.Body.Close()
}
