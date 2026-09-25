package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// LiteralAICallback sends observability data to Literal AI.
type LiteralAICallback struct {
	apiKey  string
	baseURL string
}

func NewLiteralAICallback(apiKey, baseURL string) *LiteralAICallback {
	if baseURL == "" {
		baseURL = "https://cloud.getliteral.ai"
	}
	return &LiteralAICallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *LiteralAICallback) LogSuccess(data LogData) { c.log(data) }
func (c *LiteralAICallback) LogFailure(data LogData) { c.log(data) }

func (c *LiteralAICallback) log(data LogData) {
	payload := map[string]any{
		"type":   "llm",
		"name":   data.Model,
		"input":  data.Request,
		"output": data.Response,
		"metadata": map[string]any{
			"model":             data.Model,
			"provider":          data.Provider,
			"prompt_tokens":     data.PromptTokens,
			"completion_tokens": data.CompletionTokens,
			"total_tokens":      data.TotalTokens,
			"cost":              data.Cost,
		},
		"start_time": data.StartTime.Format(time.RFC3339Nano),
		"end_time":   data.EndTime.Format(time.RFC3339Nano),
		"tags":       data.RequestTags,
	}
	if data.Error != nil {
		payload["error"] = data.Error.Error()
	}

	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/graphql", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
