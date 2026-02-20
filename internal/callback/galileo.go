package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// GalileoCallback sends LLM quality observability data to Galileo.
type GalileoCallback struct {
	apiKey  string
	baseURL string
}

func NewGalileoCallback(apiKey, baseURL string) *GalileoCallback {
	if baseURL == "" {
		baseURL = "https://api.rungalileo.io"
	}
	return &GalileoCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *GalileoCallback) LogSuccess(data LogData) { c.log(data) }
func (c *GalileoCallback) LogFailure(data LogData) { c.log(data) }

func (c *GalileoCallback) log(data LogData) {
	payload := map[string]any{
		"model":             data.Model,
		"input":             data.Request,
		"output":            data.Response,
		"num_input_tokens":  data.PromptTokens,
		"num_output_tokens": data.CompletionTokens,
		"total_tokens":      data.TotalTokens,
		"cost":              data.Cost,
		"latency_ms":        data.Latency.Milliseconds(),
		"tags":              data.RequestTags,
		"created_at":        data.EndTime.Format(time.RFC3339Nano),
	}
	if data.Error != nil {
		payload["error"] = data.Error.Error()
		payload["status_code"] = 500
	}

	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/observe/logs", bytes.NewReader(body))
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
