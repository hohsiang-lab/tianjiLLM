package callback

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type WebSearchCallback struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewWebSearchCallback(apiKey, baseURL string) *WebSearchCallback {
	return &WebSearchCallback{apiKey: apiKey, baseURL: baseURL, client: &http.Client{Timeout: 5 * time.Second}}
}

func (c *WebSearchCallback) LogSuccess(data LogData) { c.send("llm.success", data) }
func (c *WebSearchCallback) LogFailure(data LogData) { c.send("llm.failure", data) }

func (c *WebSearchCallback) send(event string, data LogData) {
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
		log.Printf("warn: websearch marshal: %v", err)
		return
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/log", bytes.NewReader(body))
	if err != nil {
		log.Printf("warn: websearch request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("warn: websearch send: %v", err)
		return
	}
	resp.Body.Close()
}
