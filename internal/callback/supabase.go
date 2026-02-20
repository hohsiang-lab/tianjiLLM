package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// SupabaseCallback stores LLM logs to Supabase via REST API.
type SupabaseCallback struct {
	apiKey  string
	baseURL string
}

func NewSupabaseCallback(apiKey, baseURL string) *SupabaseCallback {
	return &SupabaseCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *SupabaseCallback) LogSuccess(data LogData) { c.log(data) }
func (c *SupabaseCallback) LogFailure(data LogData) { c.log(data) }

func (c *SupabaseCallback) log(data LogData) {
	record := map[string]any{
		"model":             data.Model,
		"provider":          data.Provider,
		"prompt_tokens":     data.PromptTokens,
		"completion_tokens": data.CompletionTokens,
		"total_tokens":      data.TotalTokens,
		"cost":              data.Cost,
		"latency_ms":        data.Latency.Milliseconds(),
		"user_id":           data.UserID,
		"team_id":           data.TeamID,
		"tags":              data.RequestTags,
		"created_at":        data.EndTime.Format(time.RFC3339),
	}
	if data.Error != nil {
		record["error"] = data.Error.Error()
	}

	body, _ := json.Marshal(record)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/rest/v1/llm_logs", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
