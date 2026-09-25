package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// PromptLayerCallback sends prompt version tracking data to PromptLayer.
type PromptLayerCallback struct {
	apiKey  string
	baseURL string
}

func NewPromptLayerCallback(apiKey, baseURL string) *PromptLayerCallback {
	if baseURL == "" {
		baseURL = "https://api.promptlayer.com"
	}
	return &PromptLayerCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *PromptLayerCallback) LogSuccess(data LogData) { c.log(data) }
func (c *PromptLayerCallback) LogFailure(data LogData) { c.log(data) }

func (c *PromptLayerCallback) log(data LogData) {
	payload := map[string]any{
		"function_name": "tianji.completion",
		"kwargs": map[string]any{
			"model":    data.Model,
			"messages": data.Request,
		},
		"request_response":   data.Response,
		"request_start_time": data.StartTime.Unix(),
		"request_end_time":   data.EndTime.Unix(),
		"tags":               data.RequestTags,
	}

	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/rest/track-request", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", c.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
