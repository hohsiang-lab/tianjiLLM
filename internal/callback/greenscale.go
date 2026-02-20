package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// GreenscaleCallback sends carbon emission tracking data to Greenscale.
type GreenscaleCallback struct {
	apiKey  string
	baseURL string
}

func NewGreenscaleCallback(apiKey, baseURL string) *GreenscaleCallback {
	if baseURL == "" {
		baseURL = "https://api.greenscale.ai"
	}
	return &GreenscaleCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *GreenscaleCallback) LogSuccess(data LogData) { c.log(data) }
func (c *GreenscaleCallback) LogFailure(data LogData) { c.log(data) }

func (c *GreenscaleCallback) log(data LogData) {
	payload := map[string]any{
		"model":             data.Model,
		"provider":          data.Provider,
		"prompt_tokens":     data.PromptTokens,
		"completion_tokens": data.CompletionTokens,
		"total_tokens":      data.TotalTokens,
		"cost":              data.Cost,
		"latency_ms":        data.Latency.Milliseconds(),
		"start_time":        data.StartTime.Format(time.RFC3339Nano),
		"end_time":          data.EndTime.Format(time.RFC3339Nano),
	}

	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/emissions", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
