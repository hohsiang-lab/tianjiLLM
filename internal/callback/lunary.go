package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// LunaryCallback sends LLM trace data to Lunary.
type LunaryCallback struct {
	apiKey  string
	baseURL string
}

func NewLunaryCallback(apiKey, baseURL string) *LunaryCallback {
	if baseURL == "" {
		baseURL = "https://api.lunary.ai"
	}
	return &LunaryCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *LunaryCallback) LogSuccess(data LogData) { c.log(data, "success") }
func (c *LunaryCallback) LogFailure(data LogData) { c.log(data, "error") }

func (c *LunaryCallback) log(data LogData, status string) {
	payload := map[string]any{
		"type":   "llm",
		"event":  status,
		"model":  data.Model,
		"name":   "chat",
		"input":  data.Request,
		"output": data.Response,
		"tokens": map[string]int{
			"prompt":     data.PromptTokens,
			"completion": data.CompletionTokens,
		},
		"duration": data.Latency.Milliseconds(),
		"cost":     data.Cost,
		"tags":     data.RequestTags,
		"userId":   data.UserID,
	}
	if data.Error != nil {
		payload["error"] = data.Error.Error()
	}

	body, _ := json.Marshal(map[string]any{"events": []any{payload}})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/runs/ingest", bytes.NewReader(body))
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
