package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// TraceloopCallback sends traces to Traceloop via HTTP API.
type TraceloopCallback struct {
	apiKey  string
	baseURL string
}

func NewTraceloopCallback(apiKey, baseURL string) *TraceloopCallback {
	if baseURL == "" {
		baseURL = "https://api.traceloop.com"
	}
	return &TraceloopCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *TraceloopCallback) LogSuccess(data LogData) { c.log(data) }
func (c *TraceloopCallback) LogFailure(data LogData) { c.log(data) }

func (c *TraceloopCallback) log(data LogData) {
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
