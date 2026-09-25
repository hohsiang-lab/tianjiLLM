package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// OpenMeterCallback sends usage metering events to OpenMeter in CloudEvents format.
type OpenMeterCallback struct {
	apiKey  string
	baseURL string
}

func NewOpenMeterCallback(apiKey, baseURL string) *OpenMeterCallback {
	if baseURL == "" {
		baseURL = "https://openmeter.cloud"
	}
	return &OpenMeterCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *OpenMeterCallback) LogSuccess(data LogData) { c.log(data) }
func (c *OpenMeterCallback) LogFailure(data LogData) { c.log(data) }

func (c *OpenMeterCallback) log(data LogData) {
	subject := data.UserID
	if subject == "" {
		subject = data.TeamID
	}
	if subject == "" {
		subject = "tianji-proxy"
	}

	event := map[string]any{
		"specversion": "1.0",
		"type":        "llm.usage",
		"source":      "tianji-proxy",
		"subject":     subject,
		"time":        data.EndTime.Format(time.RFC3339),
		"data": map[string]any{
			"model":             data.Model,
			"provider":          data.Provider,
			"total_tokens":      data.TotalTokens,
			"prompt_tokens":     data.PromptTokens,
			"completion_tokens": data.CompletionTokens,
			"cost":              data.Cost,
			"duration_ms":       data.Latency.Milliseconds(),
		},
	}

	body, _ := json.Marshal(event)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/events", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/cloudevents+json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
