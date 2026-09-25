package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// DatadogLLMCallback sends LLM observability data to Datadog LLM Observability API.
type DatadogLLMCallback struct {
	apiKey  string
	baseURL string
}

func NewDatadogLLMCallback(apiKey, baseURL string) *DatadogLLMCallback {
	if baseURL == "" {
		baseURL = "https://api.datadoghq.com"
	}
	return &DatadogLLMCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *DatadogLLMCallback) LogSuccess(data LogData) { c.log(data) }
func (c *DatadogLLMCallback) LogFailure(data LogData) { c.log(data) }

func (c *DatadogLLMCallback) log(data LogData) {
	status := "ok"
	if data.Error != nil {
		status = "error"
	}

	span := map[string]any{
		"name":     "llm.completion",
		"kind":     "llm",
		"status":   status,
		"model":    data.Model,
		"provider": data.Provider,
		"meta": map[string]any{
			"input.prompt_tokens":      data.PromptTokens,
			"output.completion_tokens": data.CompletionTokens,
			"output.total_tokens":      data.TotalTokens,
			"output.cost":              data.Cost,
		},
		"start_ns": data.StartTime.UnixNano(),
		"duration": data.Latency.Nanoseconds(),
		"tags":     data.RequestTags,
	}
	if data.Error != nil {
		span["error"] = data.Error.Error()
	}

	payload := map[string]any{"data": map[string]any{"type": "span", "attributes": span}}
	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/llm-obs/v1/trace/spans", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", c.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
