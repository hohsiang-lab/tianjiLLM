package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// PostHogCallback sends LLM usage events to PostHog.
type PostHogCallback struct {
	apiKey  string
	baseURL string
}

func NewPostHogCallback(apiKey, baseURL string) *PostHogCallback {
	if baseURL == "" {
		baseURL = "https://app.posthog.com"
	}
	return &PostHogCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *PostHogCallback) LogSuccess(data LogData) { c.log(data) }
func (c *PostHogCallback) LogFailure(data LogData) { c.log(data) }

func (c *PostHogCallback) log(data LogData) {
	distinctID := data.UserID
	if distinctID == "" {
		distinctID = "tianji-proxy"
	}

	payload := map[string]any{
		"api_key":     c.apiKey,
		"distinct_id": distinctID,
		"event":       "llm_call",
		"properties": map[string]any{
			"model":             data.Model,
			"provider":          data.Provider,
			"prompt_tokens":     data.PromptTokens,
			"completion_tokens": data.CompletionTokens,
			"total_tokens":      data.TotalTokens,
			"cost":              data.Cost,
			"latency_ms":        data.Latency.Milliseconds(),
			"cache_hit":         data.CacheHit,
			"team_id":           data.TeamID,
			"tags":              data.RequestTags,
		},
	}
	if data.Error != nil {
		if props, ok := payload["properties"].(map[string]any); ok {
			props["error"] = data.Error.Error()
		}
	}

	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/capture", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
