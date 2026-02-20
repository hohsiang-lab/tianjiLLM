package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// AthinaCallback sends LLM monitoring data to Athina.
type AthinaCallback struct {
	apiKey  string
	baseURL string
}

func NewAthinaCallback(apiKey, baseURL string) *AthinaCallback {
	if baseURL == "" {
		baseURL = "https://log.athina.ai"
	}
	return &AthinaCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *AthinaCallback) LogSuccess(data LogData) { c.log(data) }
func (c *AthinaCallback) LogFailure(data LogData) { c.log(data) }

func (c *AthinaCallback) log(data LogData) {
	payload := map[string]any{
		"language_model_id": data.Model,
		"prompt":            data.Request,
		"response":          data.Response,
		"prompt_tokens":     data.PromptTokens,
		"completion_tokens": data.CompletionTokens,
		"total_tokens":      data.TotalTokens,
		"cost":              data.Cost,
		"response_time":     data.Latency.Milliseconds(),
		"user_id":           data.UserID,
		"tags":              data.RequestTags,
		"timestamp":         data.EndTime.Format(time.RFC3339Nano),
	}
	if data.Error != nil {
		payload["error"] = data.Error.Error()
	}

	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/log/inference", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("athina-api-key", c.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
