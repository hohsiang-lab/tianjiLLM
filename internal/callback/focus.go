package callback

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type FocusCallback struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewFocusCallback(apiKey, baseURL string) *FocusCallback {
	if baseURL == "" {
		baseURL = "https://api.focus.ai/v1"
	}
	return &FocusCallback{apiKey: apiKey, baseURL: baseURL, client: &http.Client{Timeout: 5 * time.Second}}
}

func (c *FocusCallback) LogSuccess(data LogData) { c.send("llm.success", data) }
func (c *FocusCallback) LogFailure(data LogData) { c.send("llm.failure", data) }

func (c *FocusCallback) send(event string, data LogData) {
	errMsg := ""
	if data.Error != nil {
		errMsg = data.Error.Error()
	}
	payload := map[string]any{
		"event": event, "model": data.Model, "provider": data.Provider,
		"prompt_tokens": data.PromptTokens, "completion_tokens": data.CompletionTokens,
		"total_tokens": data.TotalTokens, "cost": data.Cost,
		"latency_seconds": data.Latency.Seconds(), "user_id": data.UserID,
		"team_id": data.TeamID, "cache_hit": data.CacheHit, "error": errMsg,
		"timestamp": data.EndTime.UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("warn: focus marshal: %v", err)
		return
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/log", bytes.NewReader(body))
	if err != nil {
		log.Printf("warn: focus request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("warn: focus send: %v", err)
		return
	}
	resp.Body.Close()
}
