package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// LagoCallback sends billing usage events to Lago.
type LagoCallback struct {
	apiKey  string
	baseURL string
}

func NewLagoCallback(apiKey, baseURL string) *LagoCallback {
	if baseURL == "" {
		baseURL = "https://api.getlago.com"
	}
	return &LagoCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *LagoCallback) LogSuccess(data LogData) { c.log(data) }
func (c *LagoCallback) LogFailure(data LogData) { c.log(data) }

func (c *LagoCallback) log(data LogData) {
	externalID := data.UserID
	if externalID == "" {
		externalID = data.TeamID
	}

	payload := map[string]any{
		"event": map[string]any{
			"transaction_id":       data.StartTime.Format("20060102150405") + "-" + data.Model,
			"external_customer_id": externalID,
			"code":                 "llm_tokens",
			"timestamp":            data.EndTime.Unix(),
			"properties": map[string]any{
				"model":             data.Model,
				"total_tokens":      data.TotalTokens,
				"prompt_tokens":     data.PromptTokens,
				"completion_tokens": data.CompletionTokens,
				"cost":              data.Cost,
			},
		},
	}

	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/events", bytes.NewReader(body))
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
