package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// HeliconeLogger is a post-request logger that sends call data to Helicone.
// Following Python LiteLLM: log_success → HTTP POST to {api_base}/oai/v1/log.
// NOT a BatchLogger — sends individual logs per request.
type HeliconeLogger struct {
	apiKey  string
	baseURL string
}

// NewHeliconeLogger creates a Helicone callback.
func NewHeliconeLogger(apiKey, baseURL string) *HeliconeLogger {
	if baseURL == "" {
		baseURL = "https://api.hconeai.com"
	}
	return &HeliconeLogger{apiKey: apiKey, baseURL: baseURL}
}

func (h *HeliconeLogger) LogSuccess(data LogData) {
	h.log(data)
}

func (h *HeliconeLogger) LogFailure(data LogData) {
	h.log(data)
}

func (h *HeliconeLogger) log(data LogData) {
	// Determine endpoint: /oai/v1/log for OpenAI-like, /anthropic/v1/log for Claude
	path := "/oai/v1/log"
	if strings.Contains(strings.ToLower(data.Provider), "anthropic") {
		path = "/anthropic/v1/log"
	}

	status := 200
	if data.Error != nil {
		status = 500
	}

	payload := map[string]any{
		"providerRequest": map[string]any{
			"url":  "https://api.openai.com/v1/chat/completions",
			"json": data.Request,
			"meta": map[string]any{
				"Helicone-Auth": "Bearer " + h.apiKey,
			},
		},
		"providerResponse": map[string]any{
			"json":   data.Response,
			"status": status,
		},
		"timing": map[string]any{
			"startTime": map[string]any{
				"seconds":      data.StartTime.Unix(),
				"milliseconds": data.StartTime.UnixMilli() % 1000,
			},
			"endTime": map[string]any{
				"seconds":      data.EndTime.Unix(),
				"milliseconds": data.EndTime.UnixMilli() % 1000,
			},
		},
	}

	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
