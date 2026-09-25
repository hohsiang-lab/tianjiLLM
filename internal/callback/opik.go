package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// OpikCallback sends experiment tracking data to Opik.
type OpikCallback struct {
	apiKey  string
	baseURL string
}

func NewOpikCallback(apiKey, baseURL string) *OpikCallback {
	if baseURL == "" {
		baseURL = "https://www.comet.com/opik/api"
	}
	return &OpikCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *OpikCallback) LogSuccess(data LogData) { c.log(data) }
func (c *OpikCallback) LogFailure(data LogData) { c.log(data) }

func (c *OpikCallback) log(data LogData) {
	payload := map[string]any{
		"model":             data.Model,
		"provider":          data.Provider,
		"input":             data.Request,
		"output":            data.Response,
		"prompt_tokens":     data.PromptTokens,
		"completion_tokens": data.CompletionTokens,
		"cost":              data.Cost,
		"duration_ms":       data.Latency.Milliseconds(),
		"start_time":        data.StartTime.Format(time.RFC3339Nano),
		"end_time":          data.EndTime.Format(time.RFC3339Nano),
		"tags":              data.RequestTags,
	}
	if data.Error != nil {
		payload["error"] = data.Error.Error()
	}

	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/traces", bytes.NewReader(body))
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
