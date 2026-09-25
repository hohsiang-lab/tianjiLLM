package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// ArgillaCallback sends annotation candidate logs to Argilla.
type ArgillaCallback struct {
	apiKey  string
	baseURL string
}

func NewArgillaCallback(apiKey, baseURL string) *ArgillaCallback {
	if baseURL == "" {
		baseURL = "https://api.argilla.io"
	}
	return &ArgillaCallback{apiKey: apiKey, baseURL: baseURL}
}

func (c *ArgillaCallback) LogSuccess(data LogData) { c.log(data) }
func (c *ArgillaCallback) LogFailure(data LogData) { c.log(data) }

func (c *ArgillaCallback) log(data LogData) {
	payload := map[string]any{
		"model":     data.Model,
		"input":     data.Request,
		"output":    data.Response,
		"cost":      data.Cost,
		"latency":   data.Latency.Milliseconds(),
		"user_id":   data.UserID,
		"tags":      data.RequestTags,
		"timestamp": data.EndTime.Format(time.RFC3339Nano),
	}
	if data.Error != nil {
		payload["error"] = data.Error.Error()
	}

	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/records", bytes.NewReader(body))
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
