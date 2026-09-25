package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// LogfireCallback sends structured logs to Logfire (Pydantic).
type LogfireCallback struct {
	apiKey  string
	baseURL string
}

func NewLogfireCallback(apiKey, baseURL string) *LogfireCallback {
	if baseURL == "" {
		baseURL = "https://logfire-api.pydantic.dev"
	}
	return &LogfireCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *LogfireCallback) LogSuccess(data LogData) { c.log(data, "info") }
func (c *LogfireCallback) LogFailure(data LogData) { c.log(data, "error") }

func (c *LogfireCallback) log(data LogData, level string) {
	payload := map[string]any{
		"level":   level,
		"message": "llm_call",
		"attributes": map[string]any{
			"model":             data.Model,
			"provider":          data.Provider,
			"prompt_tokens":     data.PromptTokens,
			"completion_tokens": data.CompletionTokens,
			"cost":              data.Cost,
			"latency_ms":        data.Latency.Milliseconds(),
			"user_id":           data.UserID,
			"team_id":           data.TeamID,
			"tags":              data.RequestTags,
		},
		"timestamp": data.EndTime.Format(time.RFC3339Nano),
	}
	if data.Error != nil {
		if attrs, ok := payload["attributes"].(map[string]any); ok {
			attrs["error"] = data.Error.Error()
		}
	}

	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/logs", bytes.NewReader(body))
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
