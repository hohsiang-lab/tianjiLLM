package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// DeepEvalCallback sends LLM evaluation data to DeepEval.
type DeepEvalCallback struct {
	apiKey  string
	baseURL string
}

func NewDeepEvalCallback(apiKey, baseURL string) *DeepEvalCallback {
	if baseURL == "" {
		baseURL = "https://app.confident-ai.com"
	}
	return &DeepEvalCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *DeepEvalCallback) LogSuccess(data LogData) { c.log(data) }
func (c *DeepEvalCallback) LogFailure(data LogData) { c.log(data) }

func (c *DeepEvalCallback) log(data LogData) {
	payload := map[string]any{
		"model":             data.Model,
		"input":             data.Request,
		"output":            data.Response,
		"prompt_tokens":     data.PromptTokens,
		"completion_tokens": data.CompletionTokens,
		"cost":              data.Cost,
		"latency":           data.Latency.Seconds(),
		"tags":              data.RequestTags,
		"timestamp":         data.EndTime.Format(time.RFC3339Nano),
	}
	if data.Error != nil {
		payload["error"] = data.Error.Error()
	}

	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/log", bytes.NewReader(body))
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
